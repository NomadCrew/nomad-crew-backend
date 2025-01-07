package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

// UserHandler struct with userModel
type UserHandler struct {
	userModel   models.UserModelInterface
	generateJWT func(user *types.User) (string, error)
}

func NewUserHandler(userModel models.UserModelInterface) *UserHandler {
	return &UserHandler{
		userModel:   userModel,
		generateJWT: models.GenerateJWT,
	}
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
		if err := c.Error(errors.ValidationFailed("Invalid input", err.Error())); err != nil {
			logger.GetLogger().Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		if err := c.Error(errors.New(errors.ServerError, "Failed to hash password", err.Error())); err != nil {
			logger.GetLogger().Errorw("Failed to add password hashing error", "error", err)
		}
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
		if err := c.Error(errors.NewDatabaseError(err)); err != nil {
			logger.GetLogger().Errorw("Failed to add database error", "error", err)
		}
		return
	}

	token, err := h.generateJWT(user)
	if err != nil {
		if err := c.Error(errors.New(errors.ServerError, "Failed to generate token", err.Error())); err != nil {
			logger.GetLogger().Errorw("Failed to add token generation error", "error", err)
		}
		return
	}

	response := struct {
		User  types.UserResponse `json:"user"`
		Token string             `json:"token"`
	}{
		User:  types.CreateUserResponse(user),
		Token: token,
	}

	c.JSON(http.StatusCreated, response)
}

// GetUserHandler handles retrieving a user by ID
func (h *UserHandler) GetUserHandler(c *gin.Context) {
	log := logger.GetLogger()

	id := c.Param("id")

	ctx := c.Request.Context()
	user, err := h.userModel.GetUserByID(ctx, id)
	if err != nil {
		log.Errorw("Failed to get user", "userId", id, "error", err)
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add model error", "error", err)
		}
		return
	}

	response := types.CreateUserResponse(user)
	c.JSON(http.StatusOK, response)
}

// verifyUserAccess checks user access to modify resources
func (h *UserHandler) verifyUserAccess(c *gin.Context, targetUserID string) bool {
    contextUserID := c.GetString("user_id")

    if contextUserID == "" {
        if err := c.Error(errors.AuthenticationFailed("User not authenticated")); err != nil {
            logger.GetLogger().Errorw("Failed to add authentication error", "error", err)
        }
        return false
    }

    if contextUserID != targetUserID {
        if err := c.Error(errors.AuthenticationFailed("Cannot modify other users' details")); err != nil {
            logger.GetLogger().Errorw("Failed to add access error", "error", err)
        }
        return false
    }

    return true
}

// UpdateUserHandler handles updating user information
func (h *UserHandler) UpdateUserHandler(c *gin.Context) {
    log := logger.GetLogger()

    id := c.Param("id")

    if !h.verifyUserAccess(c, id) {
        return
    }

    var req UpdateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid input", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    ctx := c.Request.Context()
    user, err := h.userModel.GetUserByID(ctx, id)
    if err != nil {
        log.Errorw("Failed to get user for update", "userId", id, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Apply updates
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
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, user)
}

// DeleteUserHandler handles deleting a user
func (h *UserHandler) DeleteUserHandler(c *gin.Context) {
    log := logger.GetLogger()

    id := c.Param("id")

    if !h.verifyUserAccess(c, id) {
        return
    }

    ctx := c.Request.Context()
    err := h.userModel.DeleteUser(ctx, id)
    if err != nil {
        log.Errorw("Failed to delete user", "userId", id, "error", err)
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// LoginHandler handles user login
func (h *UserHandler) LoginHandler(c *gin.Context) {
	log := logger.GetLogger()

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(errors.ValidationFailed("Invalid input", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	ctx := c.Request.Context()
	user, err := h.userModel.AuthenticateUser(ctx, req.Email, req.Password)
	if err != nil {
		log.Errorw("Authentication failed", "email", req.Email, "error", err)
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add authentication error", "error", err)
		}
		return
	}

	accessToken, err := models.GenerateJWT(user)
	if err != nil {
		log.Errorw("Failed to generate access token", "userId", user.ID, "error", err)
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add token generation error", "error", err)
		}
		return
	}

	refreshToken, err := models.GenerateRefreshToken(user)
	if err != nil {
		log.Errorw("Failed to generate refresh token", "userId", user.ID, "error", err)
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add token generation error", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":        accessToken,
		"refreshToken": refreshToken,
		"user":         user,
	})
}

func (h *UserHandler) SetGenerateJWTFunc(fn func(user *types.User) (string, error)) {
	h.generateJWT = fn
}
