package router

import (
	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
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
		authMiddleware := middleware.AuthMiddleware(deps.JWTValidator)
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
				tripRoutes.POST("/search", deps.TripHandler.SearchTripsHandler)
				tripRoutes.GET("/:id", deps.TripHandler.GetTripHandler)
				tripRoutes.PUT("/:id", deps.TripHandler.UpdateTripHandler)
				tripRoutes.DELETE("/:id", deps.TripHandler.DeleteTripHandler)
				tripRoutes.PATCH("/:id/status", deps.TripHandler.UpdateTripStatusHandler)

				// Trip Member Routes
				memberRoutes := tripRoutes.Group("/:id/members")
				{
					memberRoutes.GET("", deps.MemberHandler.GetTripMembersHandler)
					memberRoutes.POST("", deps.MemberHandler.AddMemberHandler)
					memberRoutes.PUT("/:memberId/role", deps.MemberHandler.UpdateMemberRoleHandler)
					memberRoutes.DELETE("/:memberId", deps.MemberHandler.RemoveMemberHandler)
				}

				// Trip Invitation Routes
				invitationRoutes := tripRoutes.Group("/:id/invitations")
				{
					invitationRoutes.POST("", deps.InvitationHandler.InviteMemberHandler)
					// invitationRoutes.GET("", deps.TripHandler.GetTripWithMembersHandler) // To be replaced with ListTripInvitationsHandler
					// invitationRoutes.PUT("/:invitationId/status", deps.TripHandler.AcceptInvitationHandler) // To be reviewed and mapped to a new/correct InvitationHandler method
				}

				// Trip Todo Routes
				todoRoutes := tripRoutes.Group("/:id/todos")
				{
					todoRoutes.POST("", deps.TodoHandler.CreateTodoHandler)
					todoRoutes.GET("", deps.TodoHandler.ListTodosHandler)
					todoRoutes.GET("/:todoId", deps.TodoHandler.GetTodoHandler)
					todoRoutes.PUT("/:todoId", deps.TodoHandler.UpdateTodoHandler)
					todoRoutes.DELETE("/:todoId", deps.TodoHandler.DeleteTodoHandler)
				}

				// Location Routes
				locationRoutes := tripRoutes.Group("/:id/locations")
				{
					locationRoutes.POST("", deps.LocationHandler.UpdateLocationHandler)
					locationRoutes.GET("", deps.LocationHandler.GetTripMemberLocationsHandler)
				}

				// Trip Chat Routes - conditionally registered based on feature flag
				chatRoutes := tripRoutes.Group("/:tripId/chat")
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

			// Location Routes - conditionally registered based on feature flag
			locationRoutes := authRoutes.Group("/location")
			{
				// Legacy endpoint - only register if Supabase Realtime is not enabled
				if !deps.FeatureFlags.EnableSupabaseRealtime {
					locationRoutes.POST("/update", deps.LocationHandler.UpdateLocationHandler)
				}
			}

			// New Supabase Location routes - only register if SupabaseRealtime is enabled
			if deps.FeatureFlags.EnableSupabaseRealtime && deps.LocationHandlerSupabase != nil {
				supabaseLocationRoutes := authRoutes.Group("/locations")
				{
					supabaseLocationRoutes.PUT("", deps.LocationHandlerSupabase.UpdateLocation)
					supabaseLocationRoutes.GET("/trips/:tripId", deps.LocationHandlerSupabase.GetTripMemberLocations)
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
