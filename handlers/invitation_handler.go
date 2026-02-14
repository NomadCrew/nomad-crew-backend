package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/command"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	notificationSvc "github.com/NomadCrew/nomad-crew-backend/models/notification/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InvitationHandler handles HTTP requests related to trip invitations.
type InvitationHandler struct {
	tripModel           interfaces.TripModelInterface
	userStore           store.UserStore
	eventService        types.EventPublisher
	serverConfig        *config.ServerConfig
	notificationService notificationSvc.NotificationService
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

// NewInvitationHandlerWithNotifications creates a new InvitationHandler with notification support.
func NewInvitationHandlerWithNotifications(
	tripModel interfaces.TripModelInterface,
	userStore store.UserStore,
	eventService types.EventPublisher,
	serverConfig *config.ServerConfig,
	notificationService notificationSvc.NotificationService,
) *InvitationHandler {
	return &InvitationHandler{
		tripModel:           tripModel,
		userStore:           userStore,
		eventService:        eventService,
		serverConfig:        serverConfig,
		notificationService: notificationService,
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
	inviterID := c.GetString(string(middleware.UserIDKey))
	if inviterID == "" {
		log.Warn("InviteMemberHandler: missing user ID in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error()))
		return
	}

	// Generate invitation ID
	invitationID := uuid.New().String()
	inviteeEmail := strings.ToLower(req.Email)

	// Set expiry to 7 days from now
	expiryDuration := 7 * 24 * time.Hour
	expiresAt := time.Now().Add(expiryDuration)

	// Generate JWT token for the invitation
	token, err := auth.GenerateInvitationToken(invitationID, tripID, inviteeEmail, h.serverConfig.JwtSecretKey, expiryDuration)
	if err != nil {
		log.Errorw("Failed to generate invitation token", "error", err, "tripID", tripID)
		handleModelError(c, apperrors.InternalServerError("failed to generate invitation token"))
		return
	}

	// Normalize role to uppercase to match database enum
	normalizedRole := types.MemberRole(strings.ToUpper(string(req.Role)))

	invitation := &types.TripInvitation{
		ID:           invitationID,
		TripID:       tripID,
		InviterID:    inviterID,
		InviteeEmail: inviteeEmail,
		Role:         normalizedRole,
		Status:       types.InvitationStatusPending,
		Token:        sql.NullString{String: token, Valid: true},
		ExpiresAt:    &expiresAt,
	}

	if err := h.tripModel.CreateInvitation(c.Request.Context(), invitation); err != nil {
		handleModelError(c, err)
		return
	}

	// Send push notification to invitee if they're a registered user and notification service is available
	// Note: Use a background context with timeout since the HTTP request context will be canceled
	// after the response is sent, which would cause the goroutine's database queries to fail.
	if h.notificationService != nil {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		go func() {
			defer cancel()
			h.sendInvitationNotification(notifyCtx, invitation, inviterID, tripID)
		}()
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
	acceptingUserID := c.GetString(string(middleware.UserIDKey))
	if acceptingUserID == "" {
		log.Warn("AcceptInvitationHandler: missing user ID in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error()))
		return
	}

	claims, err := auth.ValidateInvitationToken(req.Token, h.serverConfig.JwtSecretKey)
	if err != nil {
		log.Warnw("Invalid invitation token", "tokenLength", len(req.Token), "error", err)
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

	// Security check: If invitation is bound to a specific user ID, verify it matches
	if invitation.InviteeID != nil && *invitation.InviteeID != "" && *invitation.InviteeID != acceptingUserID {
		log.Warnw("User trying to accept invitation not meant for them", "invitationID", invitation.ID, "inviteeID", *invitation.InviteeID, "acceptingUserID", acceptingUserID)
		handleModelError(c, apperrors.Forbidden("auth_mismatch", "You are not authorized to accept this invitation."))
		return
	}

	// Security check: Verify the accepting user's email matches the invitation email
	// This prevents someone who obtains the link from accepting it with a different account
	acceptingUser, err := h.userStore.GetUserByID(c.Request.Context(), acceptingUserID)
	if err != nil {
		log.Errorw("Failed to get accepting user details", "userID", acceptingUserID, "error", err)
		handleModelError(c, err)
		return
	}
	if !strings.EqualFold(acceptingUser.Email, invitation.InviteeEmail) {
		log.Warnw("Email mismatch: user trying to accept invitation sent to different email",
			"invitationID", invitation.ID,
			"inviteeEmail", invitation.InviteeEmail,
			"acceptingUserEmail", acceptingUser.Email,
			"acceptingUserID", acceptingUserID)
		handleModelError(c, apperrors.Forbidden("email_mismatch", "You can only accept invitations sent to your email address."))
		return
	}

	// Check if user is already a member of this trip
	existingRole, err := h.tripModel.GetUserRole(c.Request.Context(), invitation.TripID, acceptingUserID)
	if err == nil && existingRole != "" {
		log.Warnw("User already a member of trip", "tripID", invitation.TripID, "userID", acceptingUserID, "role", existingRole)
		handleModelError(c, apperrors.NewConflictError("already_member", "You are already a member of this trip."))
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
	userID := c.GetString(string(middleware.UserIDKey))
	if userID == "" {
		log.Warn("DeclineInvitationHandler: missing user ID in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req DeclineInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_request_payload", err.Error()))
		return
	}

	claims, err := auth.ValidateInvitationToken(req.Token, h.serverConfig.JwtSecretKey)
	if err != nil {
		log.Warnw("Invalid invitation token for decline", "tokenLength", len(req.Token), "error", err)
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

// AcceptInvitationByIDHandler godoc
// @Summary Accept a trip invitation by ID
// @Description Allows an authenticated user to accept an invitation using the invitation ID. The user must be the invitee.
// @Tags trips-invitations
// @Produce json
// @Param invitationId path string true "Invitation ID (UUID)"
// @Success 200 {object} types.TripMembership "Successfully joined trip"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid invitation ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not authenticated"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not the invitee"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found"
// @Failure 409 {object} types.ErrorResponse "Conflict - Already a member or invitation not pending"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /invitations/{invitationId}/accept [post]
// @Security BearerAuth
func (h *InvitationHandler) AcceptInvitationByIDHandler(c *gin.Context) {
	log := logger.GetLogger()
	acceptingUserID := c.GetString(string(middleware.UserIDKey))
	if acceptingUserID == "" {
		log.Warn("AcceptInvitationByIDHandler: missing user ID in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	invitationIDStr := c.Param("invitationId")
	if invitationIDStr == "" {
		handleModelError(c, apperrors.ValidationFailed("missing_invitation_id", "Invitation ID is required"))
		return
	}

	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), invitationIDStr)
	if err != nil {
		handleModelError(c, err)
		return
	}

	if invitation.Status != types.InvitationStatusPending {
		log.Warnw("Invitation already processed or not pending", "invitationID", invitation.ID, "status", invitation.Status)
		handleModelError(c, apperrors.ValidationFailed("invitation_not_pending", "Invitation has already been processed or is not in a pending state."))
		return
	}

	// Security check: If invitation is bound to a specific user ID, verify it matches
	if invitation.InviteeID != nil && *invitation.InviteeID != "" && *invitation.InviteeID != acceptingUserID {
		log.Warnw("User trying to accept invitation not meant for them", "invitationID", invitation.ID, "inviteeID", *invitation.InviteeID, "acceptingUserID", acceptingUserID)
		handleModelError(c, apperrors.Forbidden("auth_mismatch", "You are not authorized to accept this invitation."))
		return
	}

	// Security check: Verify the accepting user's email matches the invitation email
	acceptingUser, err := h.userStore.GetUserByID(c.Request.Context(), acceptingUserID)
	if err != nil {
		log.Errorw("Failed to get accepting user details", "userID", acceptingUserID, "error", err)
		handleModelError(c, err)
		return
	}
	if !strings.EqualFold(acceptingUser.Email, invitation.InviteeEmail) {
		log.Warnw("Email mismatch: user trying to accept invitation sent to different email",
			"invitationID", invitation.ID,
			"inviteeEmail", invitation.InviteeEmail,
			"acceptingUserEmail", acceptingUser.Email,
			"acceptingUserID", acceptingUserID)
		handleModelError(c, apperrors.Forbidden("email_mismatch", "You can only accept invitations sent to your email address."))
		return
	}

	// Check if user is already a member of this trip
	existingRole, err := h.tripModel.GetUserRole(c.Request.Context(), invitation.TripID, acceptingUserID)
	if err == nil && existingRole != "" {
		log.Warnw("User already a member of trip", "tripID", invitation.TripID, "userID", acceptingUserID, "role", existingRole)
		handleModelError(c, apperrors.NewConflictError("already_member", "You are already a member of this trip."))
		return
	}

	membership := &types.TripMembership{
		TripID: invitation.TripID,
		UserID: acceptingUserID,
		Role:   invitation.Role,
		Status: types.MembershipStatusActive,
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

	log.Infow("Successfully accepted invitation by ID", "invitationID", invitation.ID, "userID", acceptingUserID, "tripID", invitation.TripID)
	c.JSON(http.StatusOK, membership)
}

// DeclineInvitationByIDHandler godoc
// @Summary Decline a trip invitation by ID
// @Description Allows an authenticated user to decline an invitation using the invitation ID. The user must be the invitee.
// @Tags trips-invitations
// @Produce json
// @Param invitationId path string true "Invitation ID (UUID)"
// @Success 204 "Successfully declined invitation"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid invitation ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not authenticated"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User is not the invitee"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found"
// @Failure 409 {object} types.ErrorResponse "Conflict - Invitation not pending"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /invitations/{invitationId}/decline [post]
// @Security BearerAuth
func (h *InvitationHandler) DeclineInvitationByIDHandler(c *gin.Context) {
	log := logger.GetLogger()
	userID := c.GetString(string(middleware.UserIDKey))
	if userID == "" {
		log.Warn("DeclineInvitationByIDHandler: missing user ID in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	invitationIDStr := c.Param("invitationId")
	if invitationIDStr == "" {
		handleModelError(c, apperrors.ValidationFailed("missing_invitation_id", "Invitation ID is required"))
		return
	}

	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), invitationIDStr)
	if err != nil {
		handleModelError(c, err)
		return
	}

	if invitation.Status != types.InvitationStatusPending {
		log.Warnw("Invitation already processed or not pending", "invitationID", invitation.ID, "status", invitation.Status)
		handleModelError(c, apperrors.ValidationFailed("invitation_not_pending", "Invitation has already been processed or is not in a pending state."))
		return
	}

	// Security check: If invitation is bound to a specific user ID, verify it matches
	if invitation.InviteeID != nil && *invitation.InviteeID != "" && *invitation.InviteeID != userID {
		log.Warnw("User trying to decline invitation not meant for them", "invitationID", invitation.ID, "inviteeID", *invitation.InviteeID, "decliningUserID", userID)
		handleModelError(c, apperrors.Forbidden("auth_mismatch", "You are not authorized to decline this invitation."))
		return
	}

	// Security check: Verify the declining user's email matches the invitation email
	decliningUser, err := h.userStore.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		log.Errorw("Failed to get declining user details", "userID", userID, "error", err)
		handleModelError(c, err)
		return
	}
	if !strings.EqualFold(decliningUser.Email, invitation.InviteeEmail) {
		log.Warnw("Email mismatch: user trying to decline invitation sent to different email",
			"invitationID", invitation.ID,
			"inviteeEmail", invitation.InviteeEmail,
			"decliningUserEmail", decliningUser.Email,
			"decliningUserID", userID)
		handleModelError(c, apperrors.Forbidden("email_mismatch", "You can only decline invitations sent to your email address."))
		return
	}

	if err := h.tripModel.UpdateInvitationStatus(c.Request.Context(), invitation.ID, types.InvitationStatusDeclined); err != nil {
		log.Errorw("Failed to update invitation status", "invitationID", invitation.ID, "error", err)
		handleModelError(c, err)
		return
	}

	log.Infow("Successfully declined invitation by ID", "invitationID", invitation.ID, "userID", userID)
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
		log.Warnw("Invalid invitation token from deep link", "tokenLength", len(token), "error", err)
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
		log.Warnw("Invalid invitation token for details", "tokenLength", len(token), "error", err)
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

// ListTripInvitationsHandler godoc
// @Summary List all invitations for a trip
// @Description Retrieves all invitations for a specific trip. Requires ADMIN+ permissions.
// @Tags trips-invitations
// @Produce json
// @Param tripId path string true "Trip ID"
// @Success 200 {array} types.TripInvitation "List of invitations"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not authenticated"
// @Failure 403 {object} types.ErrorResponse "Forbidden - Insufficient permissions"
// @Failure 404 {object} types.ErrorResponse "Not found - Trip not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/invitations [get]
// @Security BearerAuth
func (h *InvitationHandler) ListTripInvitationsHandler(c *gin.Context) {
	tripID := c.Param("id")

	invitations, err := h.tripModel.GetInvitationsByTripID(c.Request.Context(), tripID)
	if err != nil {
		handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, invitations)
}

// DeleteInvitationHandler godoc
// @Summary Revoke a trip invitation
// @Description Revokes (cancels) a pending invitation. Requires ADMIN+ permissions.
// @Tags trips-invitations
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param invitationId path string true "Invitation ID (UUID)"
// @Success 204 "Successfully revoked invitation"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invitation not pending"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not authenticated"
// @Failure 403 {object} types.ErrorResponse "Forbidden - Insufficient permissions"
// @Failure 404 {object} types.ErrorResponse "Not found - Invitation not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{tripId}/invitations/{invitationId} [delete]
// @Security BearerAuth
func (h *InvitationHandler) DeleteInvitationHandler(c *gin.Context) {
	invitationID := c.Param("invitationId")
	if invitationID == "" {
		_ = c.Error(apperrors.ValidationFailed("missing_invitation_id", "Invitation ID is required"))
		return
	}

	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), invitationID)
	if err != nil {
		handleModelError(c, err)
		return
	}

	if invitation.Status != types.InvitationStatusPending {
		_ = c.Error(apperrors.ValidationFailed("invitation_not_pending", "Only pending invitations can be revoked"))
		return
	}

	if err := h.tripModel.UpdateInvitationStatus(c.Request.Context(), invitationID, types.InvitationStatusDeclined); err != nil {
		handleModelError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Note: handleModelError is assumed to be available from the member_handler.go or a shared utility.
// If it's not in the same package or a shared one, it needs to be defined/imported.

// sendInvitationNotification sends a push notification to the invitee if they're a registered user
func (h *InvitationHandler) sendInvitationNotification(ctx context.Context, invitation *types.TripInvitation, inviterID, tripID string) {
	log := logger.GetLogger()

	// Look up the invitee by email to see if they're a registered user
	invitee, err := h.userStore.GetUserByEmail(ctx, invitation.InviteeEmail)
	if err != nil {
		// User not found - they're not registered yet, so no push notification
		log.Debugw("Invitee not registered, skipping push notification",
			"email", invitation.InviteeEmail,
			"tripID", tripID)
		return
	}

	// Get trip details for the notification
	trip, err := h.tripModel.GetTrip(ctx, tripID)
	if err != nil {
		log.Errorw("Failed to get trip for notification", "tripID", tripID, "error", err)
		return
	}

	// Get inviter details
	inviter, err := h.userStore.GetUserByID(ctx, inviterID)
	if err != nil {
		log.Errorw("Failed to get inviter for notification", "inviterID", inviterID, "error", err)
		return
	}

	// Parse invitee UUID
	inviteeUUID, err := uuid.Parse(invitee.ID)
	if err != nil {
		log.Errorw("Failed to parse invitee UUID", "inviteeID", invitee.ID, "error", err)
		return
	}

	// Build notification metadata
	metadata := map[string]interface{}{
		"inviterName":  inviter.GetFullName(),
		"inviterID":    inviterID,
		"tripID":       tripID,
		"tripName":     trip.Name,
		"invitationID": invitation.ID,
	}

	// Create the notification via the notification service
	// This will also trigger a push notification
	// Note: Using TRIP_INVITATION_RECEIVED to match the notification_type enum in the database
	_, err = h.notificationService.CreateAndPublishNotification(
		ctx,
		inviteeUUID,
		"TRIP_INVITATION_RECEIVED",
		metadata,
	)
	if err != nil {
		log.Errorw("Failed to create invitation notification",
			"inviteeID", invitee.ID,
			"tripID", tripID,
			"error", err)
		return
	}

	log.Infow("Sent push notification for invitation",
		"inviteeID", invitee.ID,
		"inviteeEmail", invitation.InviteeEmail,
		"tripID", tripID)
}
