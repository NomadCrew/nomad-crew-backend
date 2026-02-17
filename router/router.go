package router

import (
	"context"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/internal/websocket"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	tripinterfaces "github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	userservice "github.com/NomadCrew/nomad-crew-backend/models/user/service"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// Dependencies contains all the dependencies needed to set up the router.
type Dependencies struct {
	Config              *config.Config
	JWTValidator        middleware.Validator
	UserService         userservice.UserServiceInterface
	TripHandler         *handlers.TripHandler
	TodoHandler         *handlers.TodoHandler
	HealthHandler       *handlers.HealthHandler
	NotificationHandler *handlers.NotificationHandler
	UserHandler         *handlers.UserHandler
	Logger              *zap.SugaredLogger
	MemberHandler       *handlers.MemberHandler
	InvitationHandler   *handlers.InvitationHandler
	SupabaseService     *services.SupabaseService
	RedisClient         *redis.Client
	// TripModel for RBAC middleware
	TripModel tripinterfaces.TripModelInterface
	// Supabase Realtime handlers (only ones we use now)
	ChatHandlerSupabase     *handlers.ChatHandlerSupabase
	LocationHandlerSupabase *handlers.LocationHandlerSupabase
	// WebSocket handler for real-time events
	WebSocketHandler *websocket.Handler
	// Push token handler for push notification registration
	PushTokenHandler *handlers.PushTokenHandler
	// Poll handler for polls feature
	PollHandler *handlers.PollHandler
	// Feedback handler for public feedback submissions
	FeedbackHandler *handlers.FeedbackHandler
	// Wallet handler for document management
	WalletHandler *handlers.WalletHandler
}

// userServiceAdapter adapts the UserService to implement the middleware.UserResolver interface.
// This adapter converts between models.User and types.User to avoid import cycles.
type userServiceAdapter struct {
	userService userservice.UserServiceInterface
}

// GetUserBySupabaseID implements middleware.UserResolver interface
func (a *userServiceAdapter) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	// Get the models.User from the service
	modelUser, err := a.userService.GetUserBySupabaseID(ctx, supabaseID)
	if err != nil {
		return nil, err
	}
	if modelUser == nil {
		return nil, nil
	}

	// Convert models.User to types.User
	typesUser := &types.User{
		ID:                modelUser.ID.String(),
		Username:          modelUser.Username,
		FirstName:         modelUser.FirstName,
		LastName:          modelUser.LastName,
		Email:             modelUser.Email,
		CreatedAt:         modelUser.CreatedAt,
		UpdatedAt:         modelUser.UpdatedAt,
		ProfilePictureURL: modelUser.ProfilePictureURL,
		RawUserMetaData:   modelUser.RawUserMetaData,
		LastSeenAt:        modelUser.LastSeenAt,
		IsOnline:          modelUser.IsOnline,
	}

	// Convert preferences from []byte to map[string]interface{}
	if len(modelUser.Preferences) > 0 {
		// Note: This is a simplified conversion. In a real scenario, you might want to
		// unmarshal the JSON properly, but for now we'll leave it as nil since
		// the middleware doesn't need the preferences.
		typesUser.Preferences = nil
	}

	return typesUser, nil
}

// SetupRouter configures and returns the main Gin engine with all routes defined.
func SetupRouter(deps Dependencies) *gin.Engine {
	r := gin.Default()

	// SECURITY: Configure trusted proxies before any other middleware
	// This ensures c.ClientIP() returns the actual client IP, not spoofed headers
	if len(deps.Config.Server.TrustedProxies) == 0 {
		// No trusted proxies configured - disable proxy header parsing entirely
		// This is the SAFE default: X-Forwarded-For headers are ignored
		if err := r.SetTrustedProxies(nil); err != nil {
			logger.GetLogger().Errorw("Failed to set trusted proxies", "error", err)
		}
		logger.GetLogger().Info("Trusted proxies disabled - using RemoteAddr directly for client IP")
	} else {
		// Specific trusted proxies configured - only trust headers from these IPs/CIDRs
		if err := r.SetTrustedProxies(deps.Config.Server.TrustedProxies); err != nil {
			logger.GetLogger().Fatalw("Invalid trusted proxy configuration", "error", err)
		}
		logger.GetLogger().Infow("Trusted proxies configured",
			"proxies", deps.Config.Server.TrustedProxies)
	}

	// Global Middleware
	r.Use(middleware.RequestIDMiddleware())           // Add RequestID middleware
	r.Use(middleware.SecurityHeadersMiddleware(deps.Config)) // Add security headers
	r.Use(middleware.ErrorHandler())
	// Pass pointer to ServerConfig for CORS middleware
	r.Use(middleware.CORSMiddleware(&deps.Config.Server))

	// --- Define Routes Below ---

	// Health and Metrics Routes (typically don't require auth)
	r.GET("/health", deps.HealthHandler.DetailedHealth)
	r.GET("/health/liveness", deps.HealthHandler.LivenessCheck)
	r.GET("/health/readiness", deps.HealthHandler.ReadinessCheck)
	r.GET("/metrics", gin.WrapH(promhttp.Handler())) // Prometheus metrics endpoint

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Debug routes (only in non-production)
	// if deps.Config.Server.Environment != config.EnvProduction {
	// 	// No debug routes currently active
	// }

	// Versioned API Group (v1)
	v1 := r.Group("/v1")
	{
		// WebSocket route - uses token from query param or Sec-WebSocket-Protocol header
		if deps.WebSocketHandler != nil {
			v1.GET("/ws", middleware.WSJwtAuth(deps.JWTValidator), deps.WebSocketHandler.HandleWebSocket)
		}

		// Public Invitation routes (actions that don't require user to be logged in *yet*)
		v1.GET("/invitations/join", deps.InvitationHandler.HandleInvitationDeepLink) // For deep links from emails
		v1.GET("/invitations/details", deps.InvitationHandler.GetInvitationDetails)  // To get details using a token (public potentially)

		// Create fallback limiter for when Redis is unavailable
		fallbackLimiter := middleware.NewInMemoryRateLimiter(
			deps.Config.RateLimit.AuthRequestsPerMinute,
			time.Duration(deps.Config.RateLimit.WindowSeconds)*time.Second,
		)

		// Create rate limiter for auth endpoints with fallback
		// SECURITY: Uses fail-closed behavior - rate limiting is always enforced
		authRateLimiter := middleware.AuthRateLimiterWithFallback(
			deps.RedisClient,
			fallbackLimiter,
			deps.Config.RateLimit.AuthRequestsPerMinute,
			time.Duration(deps.Config.RateLimit.WindowSeconds)*time.Second,
		)

		// Public feedback route (no auth required, rate-limited)
		if deps.FeedbackHandler != nil {
			v1.POST("/feedback", authRateLimiter, deps.FeedbackHandler.SubmitFeedback)
		}

		// Public User routes (onboarding - creates user, so can't require existing user)
		// Apply rate limiting to prevent brute force account creation
		v1.POST("/users/onboard", authRateLimiter, deps.UserHandler.OnboardUser)

		// --- Authenticated Routes ---
		// Create user resolver adapter
		userResolver := &userServiceAdapter{userService: deps.UserService}
		authMiddleware := middleware.AuthMiddleware(deps.JWTValidator, userResolver)
		authRoutes := v1.Group("")
		authRoutes.Use(authMiddleware)
		{
			// Authenticated Invitation Actions
			// These require the user (invitee) to be logged in
			// Token-based accept/decline (for email deep links)
			authRoutes.POST("/invitations/accept", authRateLimiter, deps.InvitationHandler.AcceptInvitationHandler)
			authRoutes.POST("/invitations/decline", authRateLimiter, deps.InvitationHandler.DeclineInvitationHandler)
			// ID-based endpoints (for in-app notifications)
			authRoutes.GET("/invitations/:invitationId", deps.InvitationHandler.GetInvitationByIDHandler)
			authRoutes.POST("/invitations/:invitationId/accept", authRateLimiter, deps.InvitationHandler.AcceptInvitationByIDHandler)
			authRoutes.POST("/invitations/:invitationId/decline", authRateLimiter, deps.InvitationHandler.DeclineInvitationByIDHandler)

			// Legacy Location Routes (global location updates)
			authRoutes.POST("/location/update", deps.LocationHandlerSupabase.LegacyUpdateLocation)

			// Trip Routes
			tripRoutes := authRoutes.Group("/trips")
			{
				tripRoutes.POST("", authRateLimiter, deps.TripHandler.CreateTripHandler)
				tripRoutes.GET("", deps.TripHandler.ListUserTripsHandler)
				// Trip-specific routes with RBAC
				tripRoutes.GET("/:id",
					middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
					deps.TripHandler.GetTripHandler)
				tripRoutes.PUT("/:id",
					middleware.RequirePermission(deps.TripModel, types.ActionUpdate, types.ResourceTrip, nil),
					deps.TripHandler.UpdateTripHandler)
				tripRoutes.DELETE("/:id",
					middleware.RequirePermission(deps.TripModel, types.ActionDelete, types.ResourceTrip, nil),
					deps.TripHandler.DeleteTripHandler)
				tripRoutes.PATCH("/:id/status",
					middleware.RequirePermission(deps.TripModel, types.ActionUpdate, types.ResourceTrip, nil),
					deps.TripHandler.UpdateTripStatusHandler)
				tripRoutes.POST("/:id/search",
					middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
					deps.TripHandler.SearchTripsHandler)
				tripRoutes.GET("/:id/details",
					middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
					deps.TripHandler.GetTripWithMembersHandler)
				tripRoutes.POST("/:id/weather/trigger",
					middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
					deps.TripHandler.TriggerWeatherUpdateHandler)
				tripRoutes.GET("/:id/weather",
					middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
					deps.TripHandler.GetWeatherHandler)

				// Trip Image Routes - ADMIN+ can manage, MEMBER can view
				tripRoutes.POST("/:id/images",
					middleware.RequirePermission(deps.TripModel, types.ActionUpdate, types.ResourceTrip, nil),
					deps.TripHandler.UploadTripImage)
				tripRoutes.GET("/:id/images",
					middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
					deps.TripHandler.ListTripImages)
				tripRoutes.DELETE("/:id/images/:imageId",
					middleware.RequirePermission(deps.TripModel, types.ActionUpdate, types.ResourceTrip, nil),
					deps.TripHandler.DeleteTripImage)

				// Trip Member Routes - nested under trip, inherit trip membership check
				memberRoutes := tripRoutes.Group("/:id/members")
				{
					memberRoutes.GET("",
						middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceMember, nil),
						deps.MemberHandler.GetTripMembersHandler)
					memberRoutes.POST("",
						middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceMember, nil),
						deps.MemberHandler.AddMemberHandler)
					memberRoutes.DELETE("/:memberId",
						middleware.RequirePermission(deps.TripModel, types.ActionRemove, types.ResourceMember, nil),
						deps.MemberHandler.RemoveMemberHandler)
					memberRoutes.PUT("/:memberId/role",
						middleware.RequirePermission(deps.TripModel, types.ActionChangeRole, types.ResourceMember, nil),
						deps.MemberHandler.UpdateMemberRoleHandler)
				}

				// Trip Invitation Routes - ADMIN+ can manage invitations
				invitationRoutes := tripRoutes.Group("/:id/invitations")
				{
					invitationRoutes.POST("",
						authRateLimiter,
						middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceInvitation, nil),
						deps.InvitationHandler.InviteMemberHandler)
					invitationRoutes.GET("",
						middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceInvitation, nil),
						deps.InvitationHandler.ListTripInvitationsHandler)
					invitationRoutes.DELETE("/:invitationId",
						middleware.RequirePermission(deps.TripModel, types.ActionDelete, types.ResourceInvitation, nil),
						deps.InvitationHandler.DeleteInvitationHandler)
				}

				// Trip Location Routes - users can only update/delete their own location
				tripLocationRoutes := tripRoutes.Group("/:id/locations")
				{
					// Supabase versions (only version now)
					// POST/PUT location: users manage their own location only (ownership implicit - user_id from context)
					tripLocationRoutes.POST("",
						middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceLocation, nil),
						deps.LocationHandlerSupabase.CreateLocation)
					tripLocationRoutes.PUT("",
						middleware.RequirePermission(deps.TripModel, types.ActionUpdate, types.ResourceLocation, currentUserAsOwner),
						deps.LocationHandlerSupabase.UpdateLocation)
					tripLocationRoutes.GET("",
						middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceLocation, nil),
						deps.LocationHandlerSupabase.GetTripMemberLocations)
				}

				// Trip Chat Routes - members can send/read, ownership checks for update/delete
				chatRoutes := tripRoutes.Group("/:id/chat")
				{
					// Supabase Realtime endpoints (only version now)
					// Any member can send and read messages
					chatRoutes.POST("/messages",
						middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceChat, nil),
						deps.ChatHandlerSupabase.SendMessage)
					chatRoutes.GET("/messages",
						middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceChat, nil),
						deps.ChatHandlerSupabase.GetMessages)
					// Update read status - any member can mark messages as read
					chatRoutes.PUT("/messages/read",
						middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceChat, nil),
						deps.ChatHandlerSupabase.UpdateReadStatus)
					// Reactions - any member can add/remove reactions
					chatRoutes.POST("/messages/:messageId/reactions",
						middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceChat, nil),
						deps.ChatHandlerSupabase.AddReaction)
					chatRoutes.DELETE("/messages/:messageId/reactions/:emoji",
						middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceChat, nil),
						deps.ChatHandlerSupabase.RemoveReaction)
				}

				// Trip Poll Routes - ADMIN+ can manage any, MEMBER can manage own
				pollRoutes := tripRoutes.Group("/:id/polls")
				{
					if deps.PollHandler != nil {
						pollRoutes.GET("",
							middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourcePoll, nil),
							deps.PollHandler.ListPollsHandler)
						pollRoutes.POST("",
							middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourcePoll, nil),
							deps.PollHandler.CreatePollHandler)
						pollRoutes.GET("/:pollID",
							middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourcePoll, nil),
							deps.PollHandler.GetPollHandler)
						pollRoutes.PUT("/:pollID",
							middleware.RequireTripMembership(deps.TripModel),
							deps.PollHandler.UpdatePollHandler)
						pollRoutes.DELETE("/:pollID",
							middleware.RequireTripMembership(deps.TripModel),
							deps.PollHandler.DeletePollHandler)
						pollRoutes.POST("/:pollID/vote",
							middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourcePoll, nil),
							deps.PollHandler.CastVoteHandler)
						pollRoutes.DELETE("/:pollID/vote/:optionID",
							middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourcePoll, nil),
							deps.PollHandler.RemoveVoteHandler)
						pollRoutes.POST("/:pollID/close",
							middleware.RequireTripMembership(deps.TripModel),
							deps.PollHandler.ClosePollHandler)
					}
				}

				// Trip Todo Routes - ADMIN+ can manage any, MEMBER can manage own
				// Note: For update/delete, ownership check is done in the handler since
				// it requires fetching the todo to get the owner_id
				todoRoutes := tripRoutes.Group("/:id/todos")
				{
					if deps.TodoHandler != nil {
						todoRoutes.GET("",
							middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTodo, nil),
							deps.TodoHandler.ListTodosHandler)
						todoRoutes.POST("",
							middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceTodo, nil),
							deps.TodoHandler.CreateTodoHandler)
						todoRoutes.GET("/:todoID",
							middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTodo, nil),
							deps.TodoHandler.GetTodoHandler)
						// Update/Delete: Middleware checks basic membership, handler checks ownership
						// ADMIN+ can update any, MEMBER can update own (verified in handler)
						todoRoutes.PUT("/:todoID",
							middleware.RequireTripMembership(deps.TripModel),
							deps.TodoHandler.UpdateTodoHandler)
						todoRoutes.DELETE("/:todoID",
							middleware.RequireTripMembership(deps.TripModel),
							deps.TodoHandler.DeleteTodoHandler)
					}
				}

				// Group Wallet Routes - trip members can upload/view group documents
				walletTripRoutes := tripRoutes.Group("/:id/wallet")
				{
					if deps.WalletHandler != nil {
						walletTripRoutes.POST("/documents",
							authRateLimiter,
							middleware.RequirePermission(deps.TripModel, types.ActionCreate, types.ResourceTrip, nil),
							deps.WalletHandler.UploadGroupDocumentHandler)
						walletTripRoutes.GET("/documents",
							middleware.RequirePermission(deps.TripModel, types.ActionRead, types.ResourceTrip, nil),
							deps.WalletHandler.ListGroupDocumentsHandler)
					}
				}
			}

			// Wallet Routes - personal documents
			walletRoutes := authRoutes.Group("/wallet")
			{
				if deps.WalletHandler != nil {
					walletRoutes.POST("/documents", authRateLimiter, deps.WalletHandler.UploadPersonalDocumentHandler)
					walletRoutes.GET("/documents", deps.WalletHandler.ListPersonalDocumentsHandler)
					walletRoutes.GET("/documents/:docID", deps.WalletHandler.GetDocumentHandler)
					walletRoutes.PUT("/documents/:docID", deps.WalletHandler.UpdateDocumentHandler)
					walletRoutes.DELETE("/documents/:docID", deps.WalletHandler.DeleteDocumentHandler)
					walletRoutes.GET("/files/:token", deps.WalletHandler.ServeFileHandler)
				}
			}

			// Notification Routes
			notificationRoutes := authRoutes.Group("/notifications")
			{
				notificationRoutes.GET("", deps.NotificationHandler.GetNotificationsByUser)
				notificationRoutes.GET("/unread-count", deps.NotificationHandler.GetUnreadCount)
				notificationRoutes.PATCH("/:notificationId/read", deps.NotificationHandler.MarkNotificationAsRead)
				notificationRoutes.PATCH("/read-all", deps.NotificationHandler.MarkAllNotificationsRead)
				notificationRoutes.DELETE("/:notificationId", deps.NotificationHandler.DeleteNotification)
				notificationRoutes.DELETE("", deps.NotificationHandler.DeleteAllNotifications)
			}

			// User Routes
			userRoutes := authRoutes.Group("/users")
			{
				userRoutes.GET("/me", deps.UserHandler.GetCurrentUser)
				userRoutes.GET("/search", deps.UserHandler.SearchUsers)
				userRoutes.PUT("/me/contact-email", deps.UserHandler.UpdateContactEmail)
				userRoutes.GET("/:id", deps.UserHandler.GetUserByID)
				userRoutes.GET("", deps.UserHandler.ListUsers)
				userRoutes.PUT("/:id", deps.UserHandler.UpdateUser)
				userRoutes.PUT("/:id/preferences", deps.UserHandler.UpdateUserPreferences)
				// Add SyncWithSupabase as a special endpoint
				userRoutes.POST("/sync", deps.UserHandler.SyncWithSupabase)

				// Push Token Routes
				if deps.PushTokenHandler != nil {
					userRoutes.POST("/push-token", deps.PushTokenHandler.RegisterPushToken)
					userRoutes.DELETE("/push-token", deps.PushTokenHandler.DeregisterPushToken)
					userRoutes.DELETE("/push-tokens", deps.PushTokenHandler.DeregisterAllPushTokens)
				}
			}
		}
	}

	return r
}

// currentUserAsOwner is an ownership extractor that returns the current user's ID.
// Use this when the resource is inherently owned by the logged-in user (e.g., their own location).
func currentUserAsOwner(c *gin.Context) string {
	return c.GetString(string(middleware.UserIDKey))
}
