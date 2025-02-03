package handlers

import (
	"fmt"
	"net/http"
    "context"
	"time"
	"encoding/json"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
	"github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
)

type TripHandler struct {
    tripModel    *models.TripModel
    eventService types.EventPublisher
}

type UpdateTripStatusRequest struct {
    Status string `json:"status" binding:"required"`
}

func NewTripHandler(model *models.TripModel, eventService types.EventPublisher) *TripHandler {
    return &TripHandler{
        tripModel:    model,
        eventService: eventService,
    }
}

// CreateTripRequest represents the request body for creating a trip
type CreateTripRequest struct {
    Name        string    `json:"name" binding:"required"`
    Description string    `json:"description"`
    Destination string    `json:"destination" binding:"required"`
    StartDate   time.Time `json:"startDate" binding:"required"`
    EndDate     time.Time `json:"endDate" binding:"required"`
}

func (h *TripHandler) CreateTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Errorw("Failed to load config", "error", err)
        if err := c.Error(errors.InternalServerError("Failed to load configuration")); err != nil {
            log.Errorw("Failed to add internal server error", "error", err)
        }
        return
    }
    pexelsClient := pexels.NewClient(cfg.PexelsAPIKey)

    var req CreateTripRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid request body", "error", err)
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    // Get user ID from context (set by auth middleware)
    userID, exists := c.Get("user_id")
    if !exists {
        if err := c.Error(errors.AuthenticationFailed("User not authenticated")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    trip := &types.Trip{
        Name:        req.Name,
        Description: req.Description,
        Destination: req.Destination,
        StartDate:   req.StartDate,
        EndDate:     req.EndDate,
        CreatedBy:   userID.(string),
        Status:      types.TripStatusPlanning,
    }
    log.Infow("Creating trip", "trip", trip)

    imageURL, err := pexelsClient.SearchDestinationImage(trip.Destination)
    if err != nil {
        log.Warnw("Failed to fetch background image", "error", err)
        // Continue without image - don't fail the trip creation
    }
    
    trip.BackgroundImageURL = imageURL

    if err := h.tripModel.CreateTrip(c.Request.Context(), trip); err != nil {
        log.Errorw("Failed to create trip", "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusCreated, trip)
}

func (h *TripHandler) GetTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    // Get trip with members
    trip, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        }
        return
    }

    // Verify user has access to this trip
    hasAccess := false
    for _, member := range trip.Members {
        if member.UserID == userID {
            hasAccess = true
            break
        }
    }

    if !hasAccess {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to view this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Verify ownership
    if trip.CreatedBy != userID {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to update this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    var update types.TripUpdate
    if err := c.ShouldBindJSON(&update); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    if err := h.tripModel.UpdateTrip(c.Request.Context(), tripID, &update); err != nil {
        log.Errorw("Failed to update trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    payload, _ := json.Marshal(trip)
    if err := h.eventService.Publish(c.Request.Context(), tripID, types.Event{
        Type:    types.EventTypeTripUpdated,
        Payload: payload,
    }); err != nil {
        log.Errorw("Failed to publish trip update event", "error", err)
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip updated successfully"})
}

func (h *TripHandler) UpdateTripStatusHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    // Parse request body
    var req UpdateTripStatusRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    // Convert string to TripStatus and validate
    newStatus := types.TripStatus(req.Status)
    if !newStatus.IsValid() {
        if err := c.Error(errors.ValidationFailed("Invalid status", "Status must be one of: PLANNING, ACTIVE, COMPLETED, CANCELLED")); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    // Verify trip ownership
    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    if trip.CreatedBy != userID {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to update this trip's status")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    // Update status
    if err := h.tripModel.UpdateTripStatus(c.Request.Context(), tripID, newStatus); err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": fmt.Sprintf("Trip status updated to %s", newStatus),
    })
}

func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
    log := logger.GetLogger()
    userID := c.GetString("user_id")

    trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userID)
    if err != nil {
        log.Errorw("Failed to list trips", "userId", userID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to get trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Verify ownership
    if trip.CreatedBy != userID {
        if err := c.Error(errors.AuthenticationFailed("Not authorized to delete this trip")); err != nil {
            log.Errorw("Failed to add authentication error", "error", err)
        }
        return
    }

    if err := h.tripModel.DeleteTrip(c.Request.Context(), tripID); err != nil {
        log.Errorw("Failed to delete trip", "tripId", tripID, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Trip deleted successfully"})
}

func (h *TripHandler) SearchTripsHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    var criteria types.TripSearchCriteria
    if err := c.ShouldBindJSON(&criteria); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid search criteria", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    trips, err := h.tripModel.SearchTrips(c.Request.Context(), criteria)
    if err != nil {
        log.Errorw("Failed to search trips", "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trips)
}

type AddMemberRequest struct {
    UserID string         `json:"userId" binding:"required"`
    Role   types.MemberRole `json:"role" binding:"required"`
}

type UpdateMemberRoleRequest struct {
    Role types.MemberRole `json:"role" binding:"required"`
}

// AddMemberHandler handles adding a new member to a trip
func (h *TripHandler) AddMemberHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    // requestingUserID := c.GetString("user_id")

    var req AddMemberRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    // Add the member
    err := h.tripModel.AddMember(c.Request.Context(), tripID, req.UserID, req.Role)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add member error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Member added successfully",
    })
}

// UpdateMemberRoleHandler handles updating a member's role
func (h *TripHandler) UpdateMemberRoleHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.Param("userId")
    requestingUserID := c.GetString("user_id")

    var req UpdateMemberRoleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    err := h.tripModel.UpdateMemberRole(c.Request.Context(), tripID, userID, req.Role, requestingUserID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to update member role error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Member role updated successfully",
    })
}

// RemoveMemberHandler handles removing a member from a trip
func (h *TripHandler) RemoveMemberHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.Param("userId")
    requestingUserID := c.GetString("user_id")

    err := h.tripModel.RemoveMember(c.Request.Context(), tripID, userID, requestingUserID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to remove member error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Member removed successfully",
    })
}

// GetTripMembersHandler handles getting all members of a trip
func (h *TripHandler) GetTripMembersHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")

    members, err := h.tripModel.GetTripMembers(c.Request.Context(), tripID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to get members error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "members": members,
    })
}

// GetTripWithMembersHandler handles getting a trip with its members
func (h *TripHandler) GetTripWithMembersHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")

    trip, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to get trip with members error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, trip)
}

func (h *TripHandler) StreamEvents(c *gin.Context) {
    tripID := c.Param("id")
    userID := c.GetString("user_id")
    log := logger.GetLogger()

    role := c.GetString("user_role")
    if role == "" {
        if err := c.Error(errors.AuthenticationFailed("Role not found in context")); err != nil {
            log.Errorw("Failed to add auth error", "error", err)
        }
        return
    }

    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Transfer-Encoding", "chunked")

    // Create context-aware channel
    eventChan, err := h.eventService.Subscribe(c.Request.Context(), tripID, userID)
    if err != nil {
        if err := c.Error(errors.InternalServerError("Failed to subscribe to events")); err != nil {
            log.Errorw("Failed to add error", "error", err)
        }
        return
    }

    // Send keep-alive pulses and handle events
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-c.Request.Context().Done():
            return
        case <-ticker.C:
            c.SSEvent("ping", nil)
            c.Writer.Flush()
        case event, ok := <-eventChan:
            if !ok {
                return
            }
            c.SSEvent("event", event)
            c.Writer.Flush()
        }
    }
}

// upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// For production, adjust the origin check as needed.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSStreamEvents upgrades the HTTP connection to a WebSocket and streams events.
// It both sends events from the Redis-based event service to the client
// and reads client messages to publish them as events.
func (h *TripHandler) WSStreamEvents(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("user_id")
	log := logger.GetLogger()

	if tripID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tripID and userID are required"})
		return
	}

	// Upgrade the connection to a WebSocket.
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error("failed to upgrade connection: ", err)
		return
	}
	defer conn.Close()

	// Subscribe to the event channel for this trip.
	eventChan, err := h.eventService.Subscribe(c.Request.Context(), tripID, userID)
	if err != nil {
		log.Error("failed to subscribe to events: ", err)
		conn.WriteMessage(websocket.TextMessage, []byte("failed to subscribe to events"))
		return
	}

	// Create an EventRouter instance.
	// (In a production app, you might initialize this once and inject it.)
	eventRouter := services.NewEventRouter()
	// Example: Register a handler for USER_MESSAGE events.
	eventRouter.Register("USER_MESSAGE", func(ctx context.Context, event types.Event) {
		// Process the user message here (e.g., persist to database, moderation, etc.)
		log.Infof("Processing USER_MESSAGE event: %s", event.ID)
		// In this example, we do nothing extra.
	})

	// Use a ticker to send periodic pings.
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Create a cancellable context.
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Start a goroutine to read messages from the connection.
	// This catches client-initiated closures or errors.
	go func() {
		for {
			// We call ReadMessage twice in two separate goroutines to support duplex.
			// If any error occurs, we cancel the context.
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Start another goroutine to handle client messages.
	// When a message is received, it is published as a USER_MESSAGE event.
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}

			evt := types.Event{
				Type:      "USER_MESSAGE",
				Payload:   message,
				Timestamp: time.Now(),
			}

			// Route the event locally (execute any server-side processing).
			eventRouter.Route(ctx, evt)

			// Publish the event to the Redis channel for distribution.
			if err := h.eventService.Publish(ctx, tripID, evt); err != nil {
				log.Error("failed to publish event: ", err)
			}
		}
	}()

	// Main loop: send ping messages and forward events from Redis to the client.
	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Error("failed to send ping: ", err)
				return
			}
		case event, ok := <-eventChan:
			if !ok {
				// Subscription closed.
				return
			}
			// Write the event as JSON over the WebSocket.
			if err := conn.WriteJSON(event); err != nil {
				log.Error("failed to write event: ", err)
				return
			}
		}
	}
}