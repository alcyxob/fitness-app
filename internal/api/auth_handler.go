package api

import (
	"alcyxob/fitness-app/internal/domain"  // For domain.Role
	"alcyxob/fitness-app/internal/service" // Import service package
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive" // For converting ID string
)

// AuthHandler holds the authentication service dependency.
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// --- Request/Response Structs ---

type RegisterRequest struct {
	Name     string      `json:"name" binding:"required"`
	Email    string      `json:"email" binding:"required,email"`
	Password string      `json:"password" binding:"required,min=8"`            // Add password complexity later if needed
	Role     domain.Role `json:"role" binding:"required,oneof=trainer client"` // Validate role
}

// UserResponse excludes sensitive info like password hash
type UserResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	CreatedAt time.Time   `json:"createdAt"`
	ClientIDs []string    `json:"clientIds,omitempty"` // Use string ObjectIDs
	TrainerID *string     `json:"trainerId,omitempty"` // Use string ObjectID
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// --- Handler Methods ---

// Register godoc
// @Summary Register a new user (Trainer or Client)
// @Description Creates a new user account.
// @Tags Auth
// @Accept json
// @Produce json
// @Param user body RegisterRequest true "Registration details"
// @Success 201 {object} UserResponse "User created successfully"
// @Failure 400 {object} gin.H "Invalid input (validation error)"
// @Failure 409 {object} gin.H "Conflict (email already exists)"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	// Bind JSON request body and perform validation based on `binding` tags
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}

	// Call the AuthService to register the user
	user, err := h.authService.Register(c.Request.Context(), req.Name, req.Email, req.Password, req.Role)
	if err != nil {
		// Handle specific service errors
		if errors.Is(err, service.ErrUserAlreadyExists) {
			abortWithError(c, http.StatusConflict, err.Error())
		} else if errors.Is(err, service.ErrHashingFailed) {
			// Log internal error?
			abortWithError(c, http.StatusInternalServerError, "Could not process registration")
		} else {
			// Log internal error?
			abortWithError(c, http.StatusInternalServerError, "An unexpected error occurred during registration")
		}
		return
	}

	// Return the created user details (without password hash)
	c.JSON(http.StatusCreated, MapUserToResponse(user))
}

// Login godoc
// @Summary Log in a user
// @Description Authenticates a user and returns a JWT token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse "Login successful"
// @Failure 400 {object} gin.H "Invalid input (validation error)"
// @Failure 401 {object} gin.H "Unauthorized (invalid credentials)"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}

	// Call the AuthService to log in
	token, user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrAuthenticationFailed) {
			abortWithError(c, http.StatusUnauthorized, err.Error())
		} else if errors.Is(err, service.ErrTokenGeneration) {
			// Log internal error?
			abortWithError(c, http.StatusInternalServerError, "Could not process login")
		} else {
			// Log internal error?
			abortWithError(c, http.StatusInternalServerError, "An unexpected error occurred during login")
		}
		return
	}

	// Return the JWT token and user details
	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  MapUserToResponse(user),
	})
}

// MapUserToResponse converts a domain User to a UserResponse DTO.
// Crucially excludes PasswordHash and converts ObjectIDs to strings.
func MapUserToResponse(user *domain.User) UserResponse {
	if user == nil {
		return UserResponse{} // Or handle appropriately
	}

	resp := UserResponse{
		ID:        user.ID.Hex(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}

	// Map ClientIDs if present
	if len(user.ClientIDs) > 0 {
		resp.ClientIDs = make([]string, len(user.ClientIDs))
		for i, id := range user.ClientIDs {
			resp.ClientIDs[i] = id.Hex()
		}
	}

	// Map TrainerID if present
	if user.TrainerID != nil && *user.TrainerID != primitive.NilObjectID {
		trainerIDHex := (*user.TrainerID).Hex()
		resp.TrainerID = &trainerIDHex
	}

	return resp
}
