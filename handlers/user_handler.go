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
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - No authenticated user"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /users/me [get]
// @Security BearerAuth
// GetCurrentUser returns the currently authenticated user's profile
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	// Get the Supabase user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No authenticated user"})
		return
	}

	supabaseID := userID.(string)

	// Get the user by Supabase ID
	user, err := h.userService.GetUserBySupabaseID(c.Request.Context(), supabaseID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update last seen timestamp (non-blocking)
	go func(userID uuid.UUID) {
		// Create a new background context for this self-contained task
		bgCtx := context.Background()
		if err := h.userService.UpdateLastSeen(bgCtx, userID); err != nil {
			logger.GetLogger().Warnw("Failed to update last seen", "error", err, "userID", userID)
		}
	}(user.ID)

	// Get the user profile (now includes supabase_id)
	profile, err := h.userService.GetUserProfile(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user profile"})
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
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid user ID format"
// @Failure 404 {object} docs.ErrorResponse "Not found - User not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
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
// @Success 200 {object} docs.UserListResponse "List of users with pagination info"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid pagination parameters"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
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

// UpdateUser godoc
// @Summary Update user profile
// @Description Updates fields for a specified user. Only admin or the user themselves can perform this action.
// @Tags user
// @Accept json
// @Produce json
// @Param id path string true "User ID to update"
// @Param request body docs.UserUpdateRequest true "User fields to update"
// @Success 200 {object} types.UserProfile "Successfully updated user profile"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid user ID or request body"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to update this profile"
// @Failure 404 {object} docs.ErrorResponse "Not found - User not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /users/{id} [put]
// @Security BearerAuth
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

// UpdateUserPreferences godoc
// @Summary Update user preferences
// @Description Updates the preferences for a specified user. Only admin or the user themselves can perform this action.
// @Tags user
// @Accept json
// @Produce json
// @Param id path string true "User ID whose preferences to update"
// @Param request body docs.UserPreferencesRequest true "User preferences map"
// @Success 204 "Preferences updated successfully"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid user ID or preferences format"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} docs.ErrorResponse "Forbidden - User not authorized to update these preferences"
// @Failure 404 {object} docs.ErrorResponse "Not found - User not found"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /users/{id}/preferences [put]
// @Security BearerAuth
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

// CreateUser creates a new user (Admin only - Route currently disabled)
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

// SyncWithSupabase syncs the current user's local data with Supabase (Route currently disabled)
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

// DeleteUser deletes a user by ID (Admin only - Route currently disabled)
func (h *UserHandler) DeleteUser(c *gin.Context) {
	// This would typically be admin-only and requires an admin check middleware.
	// Placeholder: Not implemented as route is disabled.
	c.JSON(http.StatusNotImplemented, gin.H{"message": "DeleteUser endpoint is not active"})
}

// OnboardUser handles idempotent user onboarding from Supabase JWT
// @Summary Onboard or upsert user from Supabase JWT
// @Description Upserts the user into the backend users table using info from the Supabase JWT
// @Tags user
// @Accept json
// @Produce json
// @Success 200 {object} types.UserProfile "User profile"
// @Failure 400 {object} docs.ErrorResponse "Bad request - Invalid or missing JWT"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized - Invalid token"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Router /users/onboard [post]
// @Security BearerAuth
func (h *UserHandler) OnboardUser(c *gin.Context) {
	// Extract JWT from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid Authorization header"})
		return
	}
	tokenString := authHeader[7:]

	// Parse optional username from JSON body
	var req struct {
		Username string `json:"username"`
	}
	_ = c.ShouldBindJSON(&req) // Ignore error, username is optional

	// Validate and parse JWT
	claims, err := h.userService.ValidateAndExtractClaims(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token: " + err.Error()})
		return
	}

	// If username is provided in the request, override claims.Username
	if req.Username != "" {
		claims.Username = req.Username
	}

	// Onboard (upsert) user using claims (with username)
	profile, err := h.userService.OnboardUserFromJWTClaims(c.Request.Context(), claims)
	if err != nil {
		if strings.Contains(err.Error(), "username is already taken") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username is already taken"})
			return
		}
		if strings.Contains(err.Error(), "username is required") || strings.Contains(err.Error(), "cannot be empty") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required and cannot be empty"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to onboard user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, profile)
}
