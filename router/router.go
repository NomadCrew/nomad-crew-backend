package router

import (
	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
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
	WSHandler           *handlers.WSHandler
	ChatHandler         *handlers.ChatHandler
	UserHandler         *handlers.UserHandler
	Logger              *zap.SugaredLogger
	MemberHandler       *handlers.MemberHandler
	InvitationHandler   *handlers.InvitationHandler
	TripChatHandler     *handlers.TripChatHandler
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
		// WebSocket Route
		v1.GET("/ws", deps.WSHandler.HandleWebSocketConnection)

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

				// Trip Chat Routes
				chatRoutes := tripRoutes.Group("/:id/chat")
				{
					chatRoutes.GET("/ws/events", deps.TripChatHandler.WSStreamEvents)  // Added WebSocket stream
					chatRoutes.GET("/messages", deps.TripChatHandler.ListTripMessages) // Updated to TripChatHandler
					// chatRoutes.POST("/messages", deps.ChatHandler.SendMessage) // To be mapped to TripChatHandler.SendChatMessageViaHTTP (needs creating)
					// chatRoutes.PUT("/messages/:messageId", deps.ChatHandler.UpdateMessage) // To be mapped to TripChatHandler.UpdateChatMessageViaHTTP (needs creating)
					// chatRoutes.DELETE("/messages/:messageId", deps.ChatHandler.DeleteMessage) // To be mapped to TripChatHandler.DeleteChatMessageViaHTTP (needs creating)

					// Chat reaction routes
					// chatRoutes.GET("/messages/:messageId/reactions", deps.ChatHandler.ListReactions) // To be mapped to TripChatHandler (needs creating)
					// chatRoutes.POST("/messages/:messageId/reactions", deps.ChatHandler.AddReaction) // To be mapped to TripChatHandler (needs creating)
					// chatRoutes.DELETE("/messages/:messageId/reactions/:reactionType", deps.ChatHandler.RemoveReaction) // To be mapped to TripChatHandler (needs creating)

					// Chat read status route
					chatRoutes.PUT("/read", deps.TripChatHandler.UpdateLastReadMessage) // Updated to TripChatHandler

					// Chat members route
					// chatRoutes.GET("/members", deps.ChatHandler.ListMembers) // To be mapped to TripChatHandler (needs creating)
				}
			}

			// Location Routes
			locationRoutes := authRoutes.Group("/location")
			{
				locationRoutes.POST("/update", deps.LocationHandler.UpdateLocationHandler)
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
			}
		}
	}

	return r
}
