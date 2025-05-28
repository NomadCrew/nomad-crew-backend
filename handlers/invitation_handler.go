package handlers

import (
	"net/http"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/command"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InvitationHandler handles HTTP requests related to trip invitations.
type InvitationHandler struct {
	tripModel    interfaces.TripModelInterface
	userStore    store.UserStore
	eventService types.EventPublisher
	serverConfig *config.ServerConfig
}

// NewInvitationHandler creates a new InvitationHandler.
func NewInvitationHandler(
	tripModel interfaces.TripModelInterface,
	userStore store.UserStore,
	eventService types.EventPublisher,
	serverConfig *config.ServerConfig,
) *InvitationHandler {
	return &InvitationHandler{
		tripModel:    tripModel,
		userStore:    userStore,
		eventService: eventService,
		serverConfig: serverConfig,
	}
}

// InviteMemberRequest defines the request body for inviting a member.
// This will be removed from trip_handler.go later.
type InviteMemberRequest struct {
	Email string           `json:"email" binding:"required,email"`
	Role  types.MemberRole `json:"role" binding:"required"`
}

// AcceptInvitationRequest defines the request body for accepting an invitation.
// This will be removed from trip_handler.go later.
type AcceptInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}

// DeclineInvitationRequest defines the request body for declining an invitation.
type DeclineInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}

// InviteMemberHandler godoc
// @Summary Invite a user to a trip
// @Description Invites a user by email to join a specific trip with a given role.
// @Tags trips-invitations
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param request body InviteMemberRequest true "Invitation details (email, role)"
// @Success 201 {object} types.TripInvitation "Successfully created invitation"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid input or user already member/invited"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User does not have permission to invite"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/invitations [post]
// @Security BearerAuth
func (h *InvitationHandler) InviteMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	inviterID := c.GetString(string(middleware.InternalUserIDKey))

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid invite member request", "error", err, "tripID", tripID)
		if err := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	invitation := &types.TripInvitation{
		TripID:       tripID,
		InviterID:    inviterID,
		InviteeEmail: strings.ToLower(req.Email),
		Role:         req.Role,
		Status:       types.InvitationStatusPending,
	}

	if err := h.tripModel.CreateInvitation(c.Request.Context(), invitation); err != nil {
		handleModelError(c, err)
		return
	}

	c.JSON(http.StatusCreated, invitation)
}

// AcceptInvitationHandler godoc
// @Summary Accept a trip invitation
// @Description Allows a user to accept an invitation to join a trip using a token.
// @Tags trips-invitations
// @Accept json
// @Produce json
// @Param request body AcceptInvitationRequest true "Invitation token"
// @Success 200 {object} types.TripMembership "Successfully joined trip"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid or expired token"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User ID in token does not match logged-in user (if applicable)"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found or already processed"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /invitations/accept [post]
// @Security BearerAuth // User must be logged in to accept
func (h *InvitationHandler) AcceptInvitationHandler(c *gin.Context) {
	log := logger.GetLogger()
	acceptingUserID := c.GetString(string(middleware.InternalUserIDKey))

	var req AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid accept invitation request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	claims, err := auth.ValidateInvitationToken(req.Token, h.serverConfig.JwtSecretKey)
	if err != nil {
		log.Warnw("Invalid invitation token", "token", req.Token, "error", err)
		handleModelError(c, apperrors.AuthenticationFailed("invalid or expired token"))
		return
	}

	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), claims.InvitationID)
	if err != nil {
		handleModelError(c, err)
		return
	}

	if invitation.Status != types.InvitationStatusPending {
		log.Warnw("Invitation already processed or not pending", "invitationID", invitation.ID, "status", invitation.Status)
		handleModelError(c, apperrors.ValidationFailed("invitation_not_pending", "Invitation has already been processed or is not in a pending state."))
		return
	}

	if invitation.InviteeID != nil && *invitation.InviteeID != "" && *invitation.InviteeID != acceptingUserID {
		log.Warnw("User trying to accept invitation not meant for them", "invitationID", invitation.ID, "inviteeID", *invitation.InviteeID, "acceptingUserID", acceptingUserID)
		handleModelError(c, apperrors.Forbidden("auth_mismatch", "You are not authorized to accept this invitation."))
		return
	}

	membership := &types.TripMembership{
		TripID: invitation.TripID,
		UserID: acceptingUserID,
		Role:   invitation.Role,
	}

	if err := h.tripModel.AddMember(c.Request.Context(), membership); err != nil {
		handleModelError(c, err)
		return
	}

	if err := h.tripModel.UpdateInvitationStatus(c.Request.Context(), invitation.ID, types.InvitationStatusAccepted); err != nil {
		log.Errorw("Failed to update invitation status after member addition", "invitationID", invitation.ID, "error", err)
		handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, membership)
}

// DeclineInvitationHandler godoc
// @Summary Decline a trip invitation
// @Description Allows a user to decline an invitation to join a trip using a token.
// @Tags trips-invitations
// @Accept json
// @Produce json
// @Param request body DeclineInvitationRequest true "Invitation token"
// @Success 204 "Successfully declined invitation"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid or expired token"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not authenticated or token mismatch"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found or already processed"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /invitations/decline [post]
// @Security BearerAuth // User must be logged in to decline
func (h *InvitationHandler) DeclineInvitationHandler(c *gin.Context) {
	log := logger.GetLogger()
	userID := c.GetString(string(middleware.InternalUserIDKey))

	var req DeclineInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid decline invitation request", "error", err)
		if appErr := c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error())); appErr != nil {
			log.Errorw("Failed to set error in context for decline invitation", "error", appErr)
		}
		return
	}

	claims, err := auth.ValidateInvitationToken(req.Token, h.serverConfig.JwtSecretKey)
	if err != nil {
		log.Warnw("Invalid invitation token for decline", "token", req.Token, "error", err)
		handleModelError(c, apperrors.AuthenticationFailed("Invalid or expired token"))
		return
	}

	cmdCtx := h.tripModel.GetCommandContext()

	updateCmd := command.UpdateInvitationStatusCommand{
		BaseCommand: command.BaseCommand{
			UserID: userID,
			Ctx:    cmdCtx,
		},
		InvitationID: claims.InvitationID,
		NewStatus:    types.InvitationStatusDeclined,
	}

	if _, cmdErr := updateCmd.Execute(c); cmdErr != nil {
		handleModelError(c, cmdErr)
		return
	}

	log.Infow("Successfully declined invitation", "invitationID", claims.InvitationID, "userID", userID)
	c.Status(http.StatusNoContent)
}

// HandleInvitationDeepLink godoc
// @Summary Handle a deep link for a trip invitation
// @Description Validates an invitation token from a deep link and redirects the user or provides invitation details.
// @Tags trips-invitations
// @Accept json
// @Produce json
// @Param token query string true "Invitation token"
// @Failure 302 "Redirects to frontend with token or error"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid or expired token"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /invitations/join [get]
func (h *InvitationHandler) HandleInvitationDeepLink(c *gin.Context) {
	log := logger.GetLogger()
	token := c.Query("token")

	if token == "" {
		log.Warn("Invitation deep link called without token")
		c.Redirect(http.StatusFound, h.serverConfig.FrontendURL+"/join-trip?error=missing_token")
		return
	}

	claims, err := auth.ValidateInvitationToken(token, h.serverConfig.JwtSecretKey)
	if err != nil {
		log.Warnw("Invalid invitation token from deep link", "token", token, "error", err)
		c.Redirect(http.StatusFound, h.serverConfig.FrontendURL+"/join-trip?error=invalid_token")
		return
	}

	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), claims.InvitationID)
	if err != nil {
		appErr, ok := err.(*apperrors.AppError)
		if ok && appErr.Type == apperrors.NotFoundError {
			log.Warnw("Invitation not found for token from deep link", "invitationID", claims.InvitationID)
			c.Redirect(http.StatusFound, h.serverConfig.FrontendURL+"/join-trip?error=invitation_not_found")
			return
		}
		log.Errorw("Failed to retrieve invitation for deep link token", "invitationID", claims.InvitationID, "error", err)
		handleModelError(c, err)
		return
	}

	if invitation.Status != types.InvitationStatusPending {
		log.Warnw("Deep link for already processed invitation", "invitationID", invitation.ID, "status", invitation.Status)
		redirectURL := h.serverConfig.FrontendURL + "/join-trip?error=invitation_processed&status=" + string(invitation.Status)
		if invitation.Status == types.InvitationStatusAccepted {
			redirectURL = h.serverConfig.FrontendURL + "/trips/" + invitation.TripID + "?message=already_joined"
		}
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	redirectURL := h.serverConfig.FrontendURL + "/join-trip?token=" + token
	c.Redirect(http.StatusFound, redirectURL)
}

// GetInvitationDetails godoc
// @Summary Get details of a specific invitation using a token
// @Description Retrieves details about an invitation, such as the trip name and inviter, using the invitation token. Useful for UIs before acceptance.
// @Tags trips-invitations
// @Accept json
// @Produce json
// @Param token query string true "Invitation Token"
// @Success 200 {object} types.InvitationDetailsResponse "Details of the invitation"
// @Failure 400 {object} types.ErrorResponse "Bad request - Missing or invalid token"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found or token invalid"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /invitations/details [get]
func (h *InvitationHandler) GetInvitationDetails(c *gin.Context) {
	log := logger.GetLogger()
	token := c.Query("token")

	if token == "" {
		handleModelError(c, apperrors.ValidationFailed("missing_token", "Invitation token is required."))
		return
	}

	claims, err := auth.ValidateInvitationToken(token, h.serverConfig.JwtSecretKey)
	if err != nil {
		log.Warnw("Invalid invitation token for details", "token", token, "error", err)
		handleModelError(c, apperrors.AuthenticationFailed("invalid or expired token"))
		return
	}

	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), claims.InvitationID)
	if err != nil {
		handleModelError(c, err)
		return
	}

	var tripBasicInfo types.TripBasicInfo
	trip, err := h.tripModel.GetTrip(c.Request.Context(), invitation.TripID)
	if err != nil {
		log.Errorw("Failed to get trip details for invitation", "tripID", invitation.TripID, "invitationID", invitation.ID, "error", err)
	} else if trip != nil {
		tripBasicInfo = types.TripBasicInfo{
			ID:          trip.ID,
			Name:        trip.Name,
			Description: trip.Description,
			StartDate:   trip.StartDate,
			EndDate:     trip.EndDate,
		}
	}

	var inviterResponse types.UserResponse
	if inviterUUID, pErr := uuid.Parse(invitation.InviterID); pErr == nil {
		inviter, iErr := h.userStore.GetUserByID(c.Request.Context(), inviterUUID.String())
		if iErr != nil {
			log.Errorw("Failed to get inviter details for invitation", "inviterID", invitation.InviterID, "invitationID", invitation.ID, "error", iErr)
		} else if inviter != nil {
			inviterResponse = types.UserResponse{
				ID:          inviter.ID,
				Username:    inviter.Username,
				Email:       inviter.Email,
				FirstName:   inviter.FirstName,
				LastName:    inviter.LastName,
				AvatarURL:   inviter.ProfilePictureURL,
				DisplayName: inviter.GetFullName(),
			}
		}
	} else {
		log.Errorw("Failed to parse inviter ID from invitation", "inviterID", invitation.InviterID, "invitationID", invitation.ID, "error", pErr)
	}

	response := types.InvitationDetailsResponse{
		ID:        invitation.ID,
		TripID:    invitation.TripID,
		Email:     invitation.InviteeEmail,
		Status:    invitation.Status,
		Role:      invitation.Role,
		CreatedAt: invitation.CreatedAt,
		ExpiresAt: invitation.ExpiresAt,
		Trip:      &tripBasicInfo,
		Inviter:   &inviterResponse,
	}

	c.JSON(http.StatusOK, response)
}

// Note: handleModelError is assumed to be available from the member_handler.go or a shared utility.
// If it's not in the same package or a shared one, it needs to be defined/imported.
