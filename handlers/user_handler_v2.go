package handlers

import (
	"strconv"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetCurrentUserV2 returns the currently authenticated user's profile using standardized response
func (h *UserHandler) GetCurrentUserV2(c *gin.Context) {
	rb := middleware.NewResponseBuilder(c, "1.0")
	
	// Get the authenticated user object from enhanced middleware
	userObj, exists := c.Get(string(middleware.AuthenticatedUserKey))
	if !exists {
		rb.Error(c, errors.AuthenticationFailed("No authenticated user"))
		return
	}

	user := userObj.(*types.User)

	// Convert types.User ID to UUID for service compatibility
	userUUID, parseErr := uuid.Parse(user.ID)
	if parseErr != nil {
		appErr := errors.InternalServerError("Invalid user ID format")
		appErr.Raw = parseErr
		rb.Error(c, appErr)
		return
	}

	// Update last seen timestamp (non-blocking)
	go func() {
		if err := h.userService.UpdateLastSeen(c.Request.Context(), userUUID); err != nil {
			logger.GetLogger().Warnw("Failed to update last seen timestamp",
				"user_id", userUUID,
				"error", err)
		}
	}()

	// Convert to user profile response
	profile := convertToUserProfile(user)
	
	rb.Success(c, profile)
}

// ListUsersV2 lists users with pagination using standardized response
func (h *UserHandler) ListUsersV2(c *gin.Context) {
	rb := middleware.NewResponseBuilder(c, "1.0")
	
	// Parse pagination parameters
	page := 1
	perPage := 20
	
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	
	if pp := c.Query("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}
	
	// Calculate offset
	offset := (page - 1) * perPage
	
	// Get users from service
	users, total, err := h.userService.ListUsers(c.Request.Context(), perPage, offset)
	if err != nil {
		rb.Error(c, err)
		return
	}
	
	// Convert to profiles
	profiles := make([]types.UserProfile, len(users))
	for i, user := range users {
		profiles[i] = convertModelUserToProfile(user)
	}
	
	// Calculate pagination info
	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}
	
	pageInfo := &types.PageInfo{
		Page:       page,
		PerPage:    perPage,
		Total:      int64(total),
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}
	
	rb.SuccessWithPagination(c, profiles, pageInfo)
}

// CreateUserV2 creates a new user using standardized response
func (h *UserHandler) CreateUserV2(c *gin.Context) {
	rb := middleware.NewResponseBuilder(c, "1.0")
	
	var req types.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		rb.ValidationError(c, "Invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	
	// Validate request
	if err := validateCreateUserRequest(&req); err != nil {
		rb.ValidationError(c, "Validation failed", map[string]interface{}{
			"fields": err,
		})
		return
	}
	
	// Create user
	// Convert CreateUserRequest to models.User
	newUser := &models.User{
		Username:          req.Username,
		Email:             req.Email,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		ProfilePictureURL: req.ProfilePictureURL,
	}
	
	userID, err := h.userService.CreateUser(c.Request.Context(), newUser)
	if err != nil {
		rb.Error(c, err)
		return
	}
	
	// Get the created user
	createdUser, err := h.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		rb.Error(c, err)
		return
	}
	
	// Convert to profile
	profile := convertModelUserToProfile(createdUser)
	
	rb.Created(c, profile)
}

// Example helper functions
func convertModelUserToProfile(user *models.User) types.UserProfile {
	lastSeenAt := time.Time{}
	if user.LastSeenAt != nil {
		lastSeenAt = *user.LastSeenAt
	}
	
	return types.UserProfile{
		ID:          user.ID.String(),
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Email:       user.Email,
		AvatarURL:   user.ProfilePictureURL,
		DisplayName: user.GetDisplayName(),
		LastSeenAt:  lastSeenAt,
		IsOnline:    user.IsOnline,
	}
}

func convertToUserProfile(user *types.User) types.UserProfile {
	lastSeenAt := time.Time{}
	if user.LastSeenAt != nil {
		lastSeenAt = *user.LastSeenAt
	}
	
	return types.UserProfile{
		ID:          user.ID,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Email:       user.Email,
		AvatarURL:   user.ProfilePictureURL,
		DisplayName: user.GetDisplayName(),
		LastSeenAt:  lastSeenAt,
		IsOnline:    user.IsOnline,
	}
}

func validateCreateUserRequest(req *types.CreateUserRequest) map[string]string {
	errors := make(map[string]string)
	
	if req.Username == "" {
		errors["username"] = "Username is required"
	}
	if req.Email == "" {
		errors["email"] = "Email is required"
	}
	if req.FirstName == "" {
		errors["first_name"] = "First name is required"
	}
	if req.LastName == "" {
		errors["last_name"] = "Last name is required"
	}
	
	if len(errors) > 0 {
		return errors
	}
	
	return nil
}