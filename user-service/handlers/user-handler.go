package handlers

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "strconv"
    "golang.org/x/crypto/bcrypt"
    
    "github.com/NomadCrew/nomad-crew-backend/user-service/models"
    "github.com/NomadCrew/nomad-crew-backend/user-service/logger"
    "github.com/NomadCrew/nomad-crew-backend/user-service/db"
)

type Handler struct {
	userDB *db.UserDB
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

// NewHandler creates a new instance of Handler
func NewHandler(userDB *db.UserDB) *Handler {
	return &Handler{userDB: userDB}
}

// CreateUserHandler handles the creation of a new user
func (h *Handler) CreateUserHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Failed to bind JSON", "error", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        log.Errorw("Failed to hash password", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
        return
    }

    user := &models.User{
        Username:       req.Username,
        Email:         req.Email,
        PasswordHash:   string(hashedPassword),
        FirstName:      req.FirstName,
        LastName:       req.LastName,
        ProfilePicture: req.ProfilePicture,
        PhoneNumber:    req.PhoneNumber,
        Address:        req.Address,
    }

    ctx := c.Request.Context()
    if err := user.SaveUser(ctx, h.userDB); err != nil {
        log.Errorw("Failed to save user", 
            "username", user.Username,
            "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }

    user.PasswordHash = ""
    
    log.Infow("User created successfully", "userId", user.ID)
    c.JSON(http.StatusCreated, user)
}

// GetUserHandler handles retrieving a user by ID
func (h *Handler) GetUserHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    idStr := c.Param("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        log.Errorw("Invalid user ID format", "id", idStr)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }

    ctx := c.Request.Context()
    user, err := models.GetUserByID(ctx, h.userDB, id)
    if err != nil {
        log.Errorw("Failed to get user", 
            "userId", id,
            "error", err)
        
        if err.Error() == "user not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
            return
        }
        
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
        return
    }

    log.Infow("User retrieved successfully", "userId", id)
    c.JSON(http.StatusOK, user)
}

// UpdateUserHandler handles updating a user's information
func (h *Handler) UpdateUserHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    idStr := c.Param("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        log.Errorw("Invalid user ID format", "id", idStr)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }

    var req UpdateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Failed to bind JSON", "error", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    ctx := c.Request.Context()
    user, err := models.GetUserByID(ctx, h.userDB, id)
    if err != nil {
        log.Errorw("Failed to get user for update", 
            "userId", id,
            "error", err)
        
        if err.Error() == "user not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
            return
        }
        
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
        return
    }

    // Update fields if provided
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

    if err := user.UpdateUser(ctx, h.userDB); err != nil {
        log.Errorw("Failed to update user", 
            "userId", id,
            "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
        return
    }

    log.Infow("User updated successfully", "userId", id)
    c.JSON(http.StatusOK, user)
}

// DeleteUserHandler handles the deletion of a user
func (h *Handler) DeleteUserHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    idStr := c.Param("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        log.Errorw("Invalid user ID format", "id", idStr)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }

    user := &models.User{ID: id}
    ctx := c.Request.Context()
    
    if err := user.DeleteUser(ctx, h.userDB); err != nil {
        log.Errorw("Failed to delete user", 
            "userId", id,
            "error", err)
            
        if err.Error() == "user not found" {
            c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
            return
        }
        
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
        return
    }

    log.Infow("User deleted successfully", "userId", id)
    c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// LoginHandler handles user authentication and returns a JWT token
func (h *Handler) LoginHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Failed to bind JSON", "error", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    ctx := c.Request.Context()
    user, err := models.AuthenticateUser(ctx, h.userDB, req.Email, req.Password)
    if err != nil {
        log.Errorw("Authentication failed", 
            "email", req.Email,
            "error", err)
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }

    token, err := user.GenerateJWT()
    if err != nil {
        log.Errorw("Failed to generate token", 
            "userId", user.ID,
            "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }

    log.Infow("User logged in successfully", "userId", user.ID)
    c.JSON(http.StatusOK, gin.H{
        "token": token,
        "user":  user,
    })
}