package router

import (
	"context"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	userservice "github.com/NomadCrew/nomad-crew-backend/models/user/service"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// Dependencies struct holds all dependencies required for setting up routes.
type Dependencies struct {
	Config              *config.Config
	JWTValidator        middleware.Validator
	UserService         userservice.UserServiceInterface
	TripHandler         *handlers.TripHandler
	TodoHandler         *handlers.TodoHandler
	HealthHandler       *handlers.HealthHandler
	LocationHandler     *handlers.LocationHandler
	NotificationHandler *handlers.NotificationHandler
	ChatHandler         *handlers.ChatHandler
	UserHandler         *handlers.UserHandler
	Logger              *zap.SugaredLogger
	MemberHandler       *handlers.MemberHandler
	InvitationHandler   *handlers.InvitationHandler
	SupabaseService     *services.SupabaseService
	FeatureFlags        config.FeatureFlags
	// Supabase Realtime handlers
	ChatHandlerSupabase     *handlers.ChatHandlerSupabase
	LocationHandlerSupabase *handlers.LocationHandlerSupabase
	// Add any other handlers or dependencies needed for routes
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
		SupabaseID:        modelUser.SupabaseID,
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

	// Global Middleware
	r.Use(middleware.RequestIDMiddleware()) // Add RequestID middleware
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
		// Public Invitation routes (actions that don't require user to be logged in *yet*)
		v1.GET("/invitations/join", deps.InvitationHandler.HandleInvitationDeepLink) // For deep links from emails
		v1.GET("/invitations/details", deps.InvitationHandler.GetInvitationDetails)  // To get details using a token (public potentially)

		// --- Authenticated Routes ---
		// Create user resolver adapter
		userResolver := &userServiceAdapter{userService: deps.UserService}
		authMiddleware := middleware.AuthMiddleware(deps.JWTValidator, userResolver)
		authRoutes := v1.Group("")
		authRoutes.Use(authMiddleware)
		{
			// Authenticated Invitation Actions
			// These require the user (invitee) to be logged in
			authRoutes.POST("/invitations/accept", deps.InvitationHandler.AcceptInvitationHandler)
			authRoutes.POST("/invitations/decline", deps.InvitationHandler.DeclineInvitationHandler)

			// Trip Routes
			tripRoutes := authRoutes.Group("/trips")
			{
				tripRoutes.POST("", deps.TripHandler.CreateTripHandler)
				tripRoutes.GET("", deps.TripHandler.ListUserTripsHandler)
				tripRoutes.GET("/:id", deps.TripHandler.GetTripHandler)
				tripRoutes.PUT("/:id", deps.TripHandler.UpdateTripHandler)
				tripRoutes.DELETE("/:id", deps.TripHandler.DeleteTripHandler)
				tripRoutes.PATCH("/:id/status", deps.TripHandler.UpdateTripStatusHandler)
				tripRoutes.POST("/:id/search", deps.TripHandler.SearchTripsHandler)
				tripRoutes.GET("/:id/details", deps.TripHandler.GetTripWithMembersHandler)
				tripRoutes.POST("/:id/weather/trigger", deps.TripHandler.TriggerWeatherUpdateHandler)

				// Trip Image Routes
				tripRoutes.POST("/:id/images", deps.TripHandler.UploadTripImage)
				tripRoutes.GET("/:id/images", deps.TripHandler.ListTripImages)
				tripRoutes.DELETE("/:id/images/:imageId", deps.TripHandler.DeleteTripImage)

				// Trip Member Routes
				memberRoutes := tripRoutes.Group("/:id/members")
				{
					memberRoutes.GET("", deps.MemberHandler.GetTripMembersHandler)
					memberRoutes.POST("", deps.MemberHandler.AddMemberHandler)
					memberRoutes.DELETE("/:memberId", deps.MemberHandler.RemoveMemberHandler)
					memberRoutes.PUT("/:memberId/role", deps.MemberHandler.UpdateMemberRoleHandler)
				}

				// Trip Invitation Routes
				invitationRoutes := tripRoutes.Group("/:id/invitations")
				{
					invitationRoutes.POST("", deps.InvitationHandler.InviteMemberHandler)
					// Note: Add other invitation routes as they are implemented
					// invitationRoutes.GET("", deps.InvitationHandler.ListTripInvitationsHandler)
					// invitationRoutes.DELETE("/:invitationId", deps.InvitationHandler.DeleteInvitationHandler)
				}

				// Trip Location Routes - conditionally registered based on feature flag
				tripLocationRoutes := tripRoutes.Group("/:id/locations")
				{
					if deps.FeatureFlags.EnableSupabaseRealtime && deps.LocationHandlerSupabase != nil {
						// Supabase versions
						tripLocationRoutes.PUT("", deps.LocationHandlerSupabase.UpdateLocation)         // Handles PUT for location updates
						tripLocationRoutes.GET("", deps.LocationHandlerSupabase.GetTripMemberLocations) // Handles GET for member locations
					} else if deps.LocationHandler != nil {
						// Non-Supabase versions
						tripLocationRoutes.POST("", deps.LocationHandler.UpdateLocationHandler)        // Handles POST for location updates
						tripLocationRoutes.GET("", deps.LocationHandler.GetTripMemberLocationsHandler) // Handles GET for member locations
					}
				}

				// Trip Chat Routes - conditionally registered based on feature flag
				chatRoutes := tripRoutes.Group("/:id/chat")
				{
					// New Supabase Realtime endpoints - only register if SupabaseRealtime is enabled
					if deps.FeatureFlags.EnableSupabaseRealtime && deps.ChatHandlerSupabase != nil {
						chatRoutes.POST("/messages", deps.ChatHandlerSupabase.SendMessage)
						chatRoutes.GET("/messages", deps.ChatHandlerSupabase.GetMessages)
						chatRoutes.PUT("/messages/read", deps.ChatHandlerSupabase.UpdateReadStatus)
						chatRoutes.POST("/messages/:messageId/reactions", deps.ChatHandlerSupabase.AddReaction)
						chatRoutes.DELETE("/messages/:messageId/reactions/:emoji", deps.ChatHandlerSupabase.RemoveReaction)
					}
				}
			}

			// Notification Routes
			notificationRoutes := authRoutes.Group("/notifications")
			{
				notificationRoutes.GET("", deps.NotificationHandler.GetNotificationsByUser)
				notificationRoutes.PATCH("/:notificationId/read", deps.NotificationHandler.MarkNotificationAsRead)
				notificationRoutes.PATCH("/read-all", deps.NotificationHandler.MarkAllNotificationsRead)
				notificationRoutes.DELETE("/:notificationId", deps.NotificationHandler.DeleteNotification)
			}

			// User Routes
			userRoutes := authRoutes.Group("/users")
			{
				userRoutes.GET("/me", deps.UserHandler.GetCurrentUser)
				userRoutes.GET("/:id", deps.UserHandler.GetUserByID)
				userRoutes.GET("", deps.UserHandler.ListUsers)
				userRoutes.PUT("/:id", deps.UserHandler.UpdateUser)
				userRoutes.PUT("/:id/preferences", deps.UserHandler.UpdateUserPreferences)
				// Add SyncWithSupabase as a special endpoint
				userRoutes.POST("/sync", deps.UserHandler.SyncWithSupabase)
				// Register the onboarding endpoint
				userRoutes.POST("/onboard", deps.UserHandler.OnboardUser)
			}
		}
	}

	return r
}
