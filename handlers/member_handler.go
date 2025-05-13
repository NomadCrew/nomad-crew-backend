package handlers

import (
	"net/http"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MemberHandler handles HTTP requests related to trip members.
type MemberHandler struct {
	tripModel    interfaces.TripModelInterface
	userStore    istore.UserStore
	eventService types.EventPublisher
}

// NewMemberHandler creates a new MemberHandler with the given dependencies.
func NewMemberHandler(
	tripModel interfaces.TripModelInterface,
	userStore istore.UserStore,
	eventService types.EventPublisher,
) *MemberHandler {
	return &MemberHandler{
		tripModel:    tripModel,
		userStore:    userStore,
		eventService: eventService,
	}
}

// AddMemberRequest defines the structure for the add member request body.
// This will be removed once trip_handler.go is updated.
type AddMemberRequest struct {
	UserID string           `json:"userId" binding:"required"`
	Role   types.MemberRole `json:"role" binding:"required"`
}

// UpdateMemberRoleRequest defines the structure for the update member role request body.
// This will be removed once trip_handler.go is updated.
type UpdateMemberRoleRequest struct {
	Role types.MemberRole `json:"role" binding:"required"`
}

// TripMemberResponse defines the structure for a trip member with user profile.
type TripMemberResponse struct {
	Membership types.TripMembership `json:"membership"`
	User       types.UserResponse   `json:"user"`
}

// AddMemberHandler godoc
// @Summary Add a member to a trip
// @Description Adds a user as a member to a specific trip with a given role
// @Tags trips-members
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param request body docs.AddMemberRequest true "Member details"
// @Success 201 {object} docs.TripMemberResponse "Successfully added member"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid input or user already member"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User does not have permission to add members"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip or User not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/members [post]
// @Security BearerAuth
func (h *MemberHandler) AddMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("tripId")

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid add member request", "error", err, "tripID", tripID)
		if err := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	newMembership := &types.TripMembership{
		TripID: tripID,
		UserID: req.UserID,
		Role:   req.Role,
	}

	err := h.tripModel.AddMember(c.Request.Context(), newMembership)
	if err != nil {
		handleModelError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newMembership)
}

// UpdateMemberRoleHandler godoc
// @Summary Update a trip member's role
// @Description Updates the role of an existing member in a specific trip
// @Tags trips-members
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param userId path string true "User ID of the member to update"
// @Param request body docs.UpdateMemberRoleRequest true "New role"
// @Success 200 {object} docs.TripMemberResponse "Successfully updated member's role"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid input"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User does not have permission to update roles"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip, User, or Membership not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/members/{userId}/role [put]
// @Security BearerAuth
func (h *MemberHandler) UpdateMemberRoleHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("tripId")
	memberUserID := c.Param("userId")

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid update member role request", "error", err, "tripID", tripID, "memberUserID", memberUserID)
		if err := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	_, err := h.tripModel.UpdateMemberRole(c.Request.Context(), tripID, memberUserID, req.Role)
	if err != nil {
		handleModelError(c, err)
		return
	}

	responseMembership := types.TripMembership{
		TripID: tripID,
		UserID: memberUserID,
		Role:   req.Role,
	}
	c.JSON(http.StatusOK, responseMembership)
}

// RemoveMemberHandler godoc
// @Summary Remove a member from a trip
// @Description Removes a user from a specific trip
// @Tags trips-members
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param userId path string true "User ID of the member to remove"
// @Success 204 "Successfully removed member"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User does not have permission to remove members"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip or User not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/members/{userId} [delete]
// @Security BearerAuth
func (h *MemberHandler) RemoveMemberHandler(c *gin.Context) {
	tripID := c.Param("tripId")
	memberUserID := c.Param("userId")

	err := h.tripModel.RemoveMember(c.Request.Context(), tripID, memberUserID)
	if err != nil {
		handleModelError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetTripMembersHandler godoc
// @Summary Get all members of a trip
// @Description Retrieves a list of all members for a specific trip, including their profile information.
// @Tags trips-members
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Success 200 {array} docs.TripMemberDetailResponse "List of trip members with profile information"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User is not a member of this trip"
// @Failure 404 {object} docs.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/members [get]
// @Security BearerAuth
func (h *MemberHandler) GetTripMembersHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("tripId")

	memberships, err := h.tripModel.GetTripMembers(c.Request.Context(), tripID)
	if err != nil {
		handleModelError(c, err)
		return
	}

	var membersResponse []TripMemberResponse
	for _, member := range memberships {
		userUUID, err := uuid.Parse(member.UserID)
		if err != nil {
			log.Errorw("Failed to parse member UserID", "userID", member.UserID, "error", err, "tripID", tripID)
			continue
		}

		profile, err := h.userStore.GetUserByID(c.Request.Context(), userUUID.String())
		if err != nil {
			log.Errorw("Failed to get user profile for member", "userID", member.UserID, "error", err, "tripID", tripID)
			membersResponse = append(membersResponse, TripMemberResponse{
				Membership: member,
				User:       types.UserResponse{ID: member.UserID, Username: "User not found"},
			})
			continue
		}

		membersResponse = append(membersResponse, TripMemberResponse{
			Membership: member,
			User: types.UserResponse{
				ID:          profile.ID,
				Username:    profile.Username,
				Email:       profile.Email,
				FirstName:   profile.FirstName,
				LastName:    profile.LastName,
				AvatarURL:   profile.ProfilePictureURL,
				DisplayName: profile.GetFullName(),
			},
		})
	}

	c.JSON(http.StatusOK, membersResponse)
}

// handleModelError is a helper function to translate model errors to HTTP responses.
func handleModelError(c *gin.Context, err error) {
	log := logger.GetLogger()

	var response types.ErrorResponse
	var statusCode int

	switch e := err.(type) {
	case *apperrors.AppError:
		response.Code = string(e.Type)
		response.Message = e.Message
		response.Error = e.Detail
		statusCode = e.GetHTTPStatus()
	default:
		log.Errorw("Unexpected error from model", "error", err)
		response.Code = apperrors.ServerError
		response.Message = "An unexpected error occurred"
		response.Error = "Internal server error"
		statusCode = http.StatusInternalServerError
	}

	if !c.Writer.Written() {
		c.JSON(statusCode, response)
	}
	c.Abort()
}
