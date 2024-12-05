package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"

)

// UserHandler struct with userModel
type UserHandler struct {
    userModel models.UserModelInterface
}

func NewUserHandler(userModel models.UserModelInterface) *UserHandler {
    return &UserHandler{userModel: userModel}
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	ProfilePicture string `json:"profile_picture"`
	PhoneNumber    string `json:"phone_number"`
	Address        string `json:"address"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	ProfilePicture string `json:"profile_picture"`
	PhoneNumber    string `json:"phone_number"`
	Address        string `json:"address"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// CreateUserHandler handles the creation of a new user
func (h *UserHandler) CreateUserHandler(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        _ = c.Error(errors.ValidationFailed("Invalid input", err.Error()))
        return
    }

    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        _ = c.Error(errors.New(errors.ServerError, "Failed to hash password", err.Error()))
        return
    }

    user := &types.User{
        Username:       req.Username,
        Email:          req.Email,
        PasswordHash:   string(hashedPassword),
        FirstName:      req.FirstName,
        LastName:       req.LastName,
        ProfilePicture: req.ProfilePicture,
        PhoneNumber:    req.PhoneNumber,
        Address:        req.Address,
    }

    ctx := c.Request.Context()
    if err := h.userModel.CreateUser(ctx, user); err != nil {
        _ = c.Error(errors.NewDatabaseError(err))
        return
    }

    // Convert to UserResponse before sending
    response := types.CreateUserResponse(user)
    c.JSON(http.StatusCreated, response)
}

// GetUserHandler handles retrieving a user by ID
func (h *UserHandler) GetUserHandler(c *gin.Context) {
    log := logger.GetLogger()

    idStr := c.Param("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        log.Errorw("Invalid user ID format", "id", idStr)
        _ = c.Error(errors.ValidationFailed("Invalid user ID", ""))
        return
    }

    ctx := c.Request.Context()
    user, err := h.userModel.GetUserByID(ctx, id)
    if err != nil {
        log.Errorw("Failed to get user", "userId", id, "error", err)
        _ = c.Error(err)
        return
    }

    // Convert to UserResponse format
    response := types.CreateUserResponse(user)
    c.JSON(http.StatusOK, response)
}

func (h *UserHandler) verifyUserAccess(c *gin.Context, targetUserID int64) bool {
    // Get authenticated user ID from context
    contextUserID, exists := c.Get("user_id")
    if !exists {
        c.Error(errors.AuthenticationFailed("User not authenticated"))
        return false
    }

    // Type assert with safety check
    authUserID, ok := contextUserID.(int64)
    if !ok {
        c.Error(errors.AuthenticationFailed("Invalid user ID format"))
        return false
    }

    // Users can only modify their own details
    if authUserID != targetUserID {
        c.Error(errors.AuthenticationFailed("Cannot modify other users' details"))
        return false
    }

    return true
}

// UpdateUserHandler handles updating user information
func (h *UserHandler) UpdateUserHandler(c *gin.Context) {
	log := logger.GetLogger()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Errorw("Invalid user ID format", "id", idStr)
		_ = c.Error(errors.ValidationFailed("Invalid user ID", err.Error()))
		return
	}

	if !h.verifyUserAccess(c, id) {
        return
    }

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.ValidationFailed("Invalid input", err.Error()))
		return
	}

	ctx := c.Request.Context()
    user, err := h.userModel.GetUserByID(ctx, id)
    if err != nil {
        log.Errorw("Failed to get user for update", "userId", id, "error", err)
        _ = c.Error(err)
        return
    }


	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.ProfilePicture != "" {
		user.ProfilePicture = req.ProfilePicture
	}
	if req.PhoneNumber != "" {
		user.PhoneNumber = req.PhoneNumber
	}
	if req.Address != "" {
		user.Address = req.Address
	}

    if err := h.userModel.UpdateUser(ctx, user); err != nil {
        log.Errorw("Failed to update user", "userId", id, "error", err)
        _ = c.Error(err)
        return
    }

	c.JSON(http.StatusOK, user)
}

// DeleteUserHandler handles deleting a user
func (h *UserHandler) DeleteUserHandler(c *gin.Context) {
	log := logger.GetLogger()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Errorw("Invalid user ID format", "id", idStr)
		_ = c.Error(errors.ValidationFailed("Invalid user ID", err.Error()))
		return
	}

	if !h.verifyUserAccess(c, id) {
        return
    }

	ctx := c.Request.Context()
	err = h.userModel.DeleteUser(ctx, id)
	if err != nil {
		log.Errorw("Failed to delete user", "userId", id, "error", err)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *UserHandler) LoginHandler(c *gin.Context) {
	log := logger.GetLogger()

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Failed to bind JSON", "error", err)
		_ = c.Error(errors.ValidationFailed("Invalid input", err.Error()))
		return
	}

	ctx := c.Request.Context()
	user, err := h.userModel.AuthenticateUser(ctx, req.Email, req.Password)
	if err != nil {
		log.Errorw("Authentication failed", "email", req.Email, "error", err)
		_ = c.Error(err)
		return
	}

	token, err := models.GenerateJWT(user)
	if err != nil {
		log.Errorw("Failed to generate token", "userId", user.ID, "error", err)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}
