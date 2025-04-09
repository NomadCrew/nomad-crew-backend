package router

import (
	"net/http"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/handlers"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	// Add any other handlers or dependencies needed for routes
}

// SetupRouter configures and returns the main Gin engine with all routes defined.
func SetupRouter(deps Dependencies) *gin.Engine {
	r := gin.Default()

	// Global Middleware
	r.Use(middleware.ErrorHandler())
	// Pass pointer to ServerConfig for CORS middleware
	r.Use(middleware.CORSMiddleware(&deps.Config.Server))

	// --- Define Routes Below ---

	// Health and Metrics Routes (typically don't require auth)
	r.GET("/health", deps.HealthHandler.DetailedHealth)
	r.GET("/health/liveness", deps.HealthHandler.LivenessCheck)
	r.GET("/health/readiness", deps.HealthHandler.ReadinessCheck)
	r.GET("/metrics", gin.WrapH(promhttp.Handler())) // Prometheus metrics endpoint

	// Debug routes (only in non-production)
	if deps.Config.Server.Environment != config.EnvProduction {
		debugRoutes := r.Group("/debug")
		{
			debugRoutes.GET("/jwt", handlers.DebugJWTHandler())
			debugRoutes.GET("/jwt/direct", handlers.DebugJWTDirectHandler())
			// Add other debug routes if necessary
		}
	}

	// Versioned API Group (v1)
	v1 := r.Group("/v1")
	{
		// WebSocket Route - Placeholder
		// Need to verify correct middleware and handler method name
		/*
			wsConfig := middleware.WSConfig{ // This config might be used within the handler now
				PongWait:        60 * time.Second,
				PingPeriod:      30 * time.Second,
				WriteWait:       10 * time.Second,
				MaxMessageSize:  1024,
				ReauthInterval:  5 * time.Minute,
				BufferHighWater: 256,
				BufferLowWater:  64,
			}
			// Placeholder middleware/handler call - This may need significant adjustment
			v1.GET("/ws", deps.WSHandler.HandleConnection) // Placeholder name - Linter Error
		*/
		// TODO: Define correct WebSocket route (middleware & handler)
		v1.GET("/ws", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "WebSocket endpoint not fully configured in router")
		})

		// --- Authenticated Routes ---
		authRoutes := v1.Group("")
		authRoutes.Use(middleware.AuthMiddleware(deps.JWTValidator))
		{
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
					memberRoutes.GET("", deps.TripHandler.GetTripMembersHandler)
					memberRoutes.POST("", deps.TripHandler.AddMemberHandler)
					memberRoutes.PUT("/:memberId/role", deps.TripHandler.UpdateMemberRoleHandler)
					memberRoutes.DELETE("/:memberId", deps.TripHandler.RemoveMemberHandler)
				}

				// Trip Invitation Routes
				invitationRoutes := tripRoutes.Group("/:id/invitations")
				{
					invitationRoutes.POST("", deps.TripHandler.InviteMemberHandler)
				}

				// Trip Todo Routes - Placeholders for methods with linter errors
				todoRoutes := tripRoutes.Group("/:id/todos")
				{
					todoRoutes.POST("", deps.TodoHandler.CreateTodoHandler)
					todoRoutes.GET("", deps.TodoHandler.ListTodosHandler)
					// todoRoutes.GET("/:todoId", deps.TodoHandler.GetTodoHandler) // Placeholder name - Linter Error
					todoRoutes.PUT("/:todoId", deps.TodoHandler.UpdateTodoHandler)
					todoRoutes.DELETE("/:todoId", deps.TodoHandler.DeleteTodoHandler)
					// todoRoutes.PATCH("/:todoId/status", deps.TodoHandler.UpdateTodoStatusHandler) // Placeholder name - Linter Error
				}

				// Trip Chat Routes (Placeholder - Add actual handlers)
				// ...
			}

			// Location Routes
			locationRoutes := authRoutes.Group("/location")
			{
				locationRoutes.POST("/update", deps.LocationHandler.UpdateLocationHandler)
				locationRoutes.POST("/offline", deps.LocationHandler.SaveOfflineLocationsHandler)
			}

			// Notification Routes - Placeholders for methods with linter errors
			// notificationRoutes := authRoutes.Group("/notifications") // Commented out as unused
			// {
			// 	// notificationRoutes.GET("", deps.NotificationHandler.GetUserNotificationsHandler) // Placeholder name - Linter Error
			// 	// notificationRoutes.POST("/read", deps.NotificationHandler.MarkNotificationsAsReadHandler) // Placeholder name - Linter Error
			// 	// notificationRoutes.DELETE("", deps.NotificationHandler.DeleteNotificationsHandler) // Placeholder name - Linter Error
			// }

			// User Routes (Placeholder - Add actual handlers)
			// ...
		}
	}

	return r
}

// TODO: Verify/implement correct handler method calls for WS, Todo, Notification routes.
// TODO: Add handlers for Chat, User, Invitation Acceptance if they exist
// TODO: Ensure all routes from main.go have been migrated here.
