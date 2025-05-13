package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	userservice "github.com/NomadCrew/nomad-crew-backend/models/user/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler manages user-related HTTP endpoints
type UserHandler struct {
	userService userservice.UserServiceInterface
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userService userservice.UserServiceInterface) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// RegisterRoutes registers all user routes to the router
func (h *UserHandler) RegisterRoutes(r *gin.RouterGroup) {
	userRoutes := r.Group("/users")
	{
		// Public endpoints
		userRoutes.GET("/me", h.GetCurrentUser)

		// Auth required endpoints
		userRoutes.GET("/:id", h.GetUserByID)
		userRoutes.GET("", h.ListUsers)
		userRoutes.PUT("/:id", h.UpdateUser)
		userRoutes.PUT("/:id/preferences", h.UpdateUserPreferences)

		// Admin only endpoints - would need proper middleware
		// userRoutes.POST("", h.CreateUser)
		// userRoutes.DELETE("/:id", h.DeleteUser)
	}
}

// GetCurrentUser godoc
// @Summary Get current user profile
// @Description Retrieves the profile of the currently authenticated user
// @Tags user
// @Accept json
// @Produce json
// @Success 200 {object} types.UserProfile "User profile"
// @Failure 401 {object} map[string]string "Unauthorized - No authenticated user"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /users/me [get]
// @Security BearerAuth
// GetCurrentUser returns the currently authenticated user's profile
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	// Get the user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

	// Parse the UUID
	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Update last seen timestamp
	go func(ctx context.Context, userID uuid.UUID) {
		if err := h.userService.UpdateLastSeen(ctx, userID); err != nil {
			logger.GetLogger().Warnw("Failed to update last seen", "error", err, "userID", userID)
		}
	}(c.Request.Context(), id)

	// Get the user profile
	profile, err := h.userService.GetUserProfile(c.Request.Context(), id)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to get user profile"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// GetUserByID godoc
// @Summary Get user by ID
// @Description Retrieves a user profile by user ID
// @Tags user
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} types.UserProfile "User profile"
// @Failure 400 {object} map[string]string "Bad request - Invalid user ID format"
// @Failure 404 {object} map[string]string "Not found - User not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /users/{id} [get]
// @Security BearerAuth
// GetUserByID retrieves a user by ID
func (h *UserHandler) GetUserByID(c *gin.Context) {
	// Parse the UUID from path parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Get the user profile
	profile, err := h.userService.GetUserProfile(c.Request.Context(), id)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to get user profile"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// ListUsers godoc
// @Summary List users
// @Description Retrieves a paginated list of users
// @Tags user
// @Accept json
// @Produce json
// @Param offset query int false "Pagination offset (default 0)"
// @Param limit query int false "Pagination limit (default 20, max 100)"
// @Success 200 {object} map[string]interface{} "List of users with pagination info"
// @Failure 400 {object} map[string]string "Bad request - Invalid pagination parameters"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /users [get]
// @Security BearerAuth
// ListUsers retrieves a paginated list of users
func (h *UserHandler) ListUsers(c *gin.Context) {
	// Parse pagination parameters
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "20")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset parameter"})
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter, must be between 1 and 100"})
		return
	}

	// Get users from service
	users, total, err := h.userService.ListUsers(c.Request.Context(), offset, limit)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to list users"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	// Convert to API response format
	userProfiles := make([]*types.UserProfile, 0, len(users))
	for _, user := range users {
		userProfiles = append(userProfiles, &types.UserProfile{
			ID:          user.ID.String(),
			Email:       user.Email,
			Username:    user.Username,
			FirstName:   user.FirstName,
			LastName:    user.LastName,
			AvatarURL:   user.ProfilePictureURL,
			DisplayName: user.GetDisplayName(),
		})
	}

	// Create paginated response
	response := gin.H{
		"users": userProfiles,
		"pagination": gin.H{
			"total":  total,
			"offset": offset,
			"limit":  limit,
		},
	}

	c.JSON(http.StatusOK, response)
}

// UpdateUser updates a user's profile
func (h *UserHandler) UpdateUser(c *gin.Context) {
	// Parse the UUID from path parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Get the user ID from context (set by auth middleware)
	currentUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

	// Parse the current user ID
	currentID, err := uuid.Parse(currentUserID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid authenticated user ID format"})
		return
	}

	// For now, no users are admins
	isAdmin := false // TODO: Implement admin check

	// Parse request body
	var updateReq models.UserUpdateRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Update the user using the specialized service method that handles validation and authorization
	updatedUser, err := h.userService.UpdateUserProfile(c.Request.Context(), id, currentID, isAdmin, updateReq)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else if strings.Contains(err.Error(), "Unauthorized") {
			status = http.StatusForbidden
			message = err.Error()
		} else if strings.Contains(err.Error(), "Validation failed") {
			status = http.StatusBadRequest
			message = err.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to update user"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	// Get the updated user profile from service
	profile, err := h.userService.GetUserProfile(c.Request.Context(), updatedUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated user profile"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateUserPreferences updates a user's preferences
func (h *UserHandler) UpdateUserPreferences(c *gin.Context) {
	// Parse the UUID from path parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Get the user ID from context (set by auth middleware)
	currentUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

	// Parse the current user ID
	currentID, err := uuid.Parse(currentUserID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid authenticated user ID format"})
		return
	}

	// For now, no users are admins
	isAdmin := false // TODO: Implement admin check

	// Parse request body
	var prefsMap map[string]interface{}
	if err := c.ShouldBindJSON(&prefsMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid preferences format"})
		return
	}

	// Update preferences using the specialized service method
	err = h.userService.UpdateUserPreferencesWithValidation(c.Request.Context(), id, currentID, isAdmin, prefsMap)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else if strings.Contains(err.Error(), "Unauthorized") {
			status = http.StatusForbidden
			message = err.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to update preferences"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Preferences updated successfully"})
}

// CreateUser creates a new user (admin only)
func (h *UserHandler) CreateUser(c *gin.Context) {
	// This would typically be admin-only
	var userReq struct {
		SupabaseID        string `json:"supabaseId" binding:"required"`
		Email             string `json:"email" binding:"required,email"`
		Username          string `json:"username" binding:"required"`
		FirstName         string `json:"firstName"`
		LastName          string `json:"lastName"`
		ProfilePictureURL string `json:"profilePictureUrl"`
	}

	if err := c.ShouldBindJSON(&userReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Create user model
	user := &models.User{
		SupabaseID:        userReq.SupabaseID,
		Email:             userReq.Email,
		Username:          userReq.Username,
		FirstName:         userReq.FirstName,
		LastName:          userReq.LastName,
		ProfilePictureURL: userReq.ProfilePictureURL,
	}

	// Create the user
	id, err := h.userService.CreateUser(c.Request.Context(), user)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to create user"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	// Get the created user profile
	profile, err := h.userService.GetUserProfile(c.Request.Context(), id)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to get created user profile"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusCreated, profile)
}

// SyncWithSupabase syncs a user with their Supabase profile
func (h *UserHandler) SyncWithSupabase(c *gin.Context) {
	// Get the user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

	// Parse the UUID
	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Get the user to get their Supabase ID
	user, err := h.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to get user"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	if user.SupabaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User has no Supabase ID"})
		return
	}

	// Sync with Supabase
	syncedUser, err := h.userService.SyncWithSupabase(c.Request.Context(), user.SupabaseID)
	if err != nil {
		var status int
		var message string

		if appErr, ok := err.(*apperrors.AppError); ok {
			status = appErr.GetHTTPStatus()
			message = appErr.Error()
		} else {
			status = http.StatusInternalServerError
			message = "Failed to sync with Supabase"
		}

		c.JSON(status, gin.H{"error": message})
		return
	}

	// Convert to profile response
	profile := &types.UserProfile{
		ID:          syncedUser.ID.String(),
		Email:       syncedUser.Email,
		Username:    syncedUser.Username,
		FirstName:   syncedUser.FirstName,
		LastName:    syncedUser.LastName,
		AvatarURL:   syncedUser.ProfilePictureURL,
		DisplayName: syncedUser.GetDisplayName(),
	}

	c.JSON(http.StatusOK, profile)
}
