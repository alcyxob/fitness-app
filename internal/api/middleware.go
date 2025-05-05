package api

import (
	"errors"
	"alcyxob/fitness-app/internal/domain" // For domain.Role
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// Constants for context keys
const (
	ContextUserIDKey   = "userID"
	ContextUserRoleKey = "userRole"
)

// jwtClaims defines the structure we expect in the JWT payload.
// Mirroring the structure used in authService.generateJWT
type jwtClaims struct {
	UserID string      `json:"uid"`
	Role   domain.Role `json:"role"`
	jwt.RegisteredClaims
}

// AuthMiddleware creates a Gin middleware for JWT authentication.
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortWithError(c, http.StatusUnauthorized, "Authorization header is missing")
			return
		}

		// Expecting "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			abortWithError(c, http.StatusUnauthorized, "Authorization header format must be Bearer {token}")
			return
		}
		tokenString := parts[1]

		// Parse and validate the token
		claims := &jwtClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Validate the alg is what we expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Return the secret key
			return []byte(jwtSecret), nil
		})

		// Handle errors during parsing/validation
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				abortWithError(c, http.StatusUnauthorized, "Token has expired")
			} else {
				abortWithError(c, http.StatusUnauthorized, fmt.Sprintf("Invalid token: %v", err))
			}
			return
		}

		if !token.Valid || claims.UserID == "" || claims.Role == "" {
			abortWithError(c, http.StatusUnauthorized, "Invalid token or missing claims")
			return
		}

		// Check if token expiry is reasonable (within configured lifetime, though ParseWithClaims checks this)
		// Redundant check usually, but doesn't hurt
		if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
			abortWithError(c, http.StatusUnauthorized, "Token has expired (claim check)")
			return
		}

		// --- Token is valid ---
		// Set user information in the context for downstream handlers
		c.Set(ContextUserIDKey, claims.UserID) // Store UserID as string (Hex representation)
		c.Set(ContextUserRoleKey, claims.Role)

		// Continue to the next handler
		c.Next()
	}
}

// Helper to return JSON error response and abort request
func abortWithError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

// RoleMiddleware creates middleware to check if user has the required role(s).
// Must run AFTER AuthMiddleware.
func RoleMiddleware(allowedRoles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleRaw, exists := c.Get(ContextUserRoleKey)
		if !exists {
			// This should not happen if AuthMiddleware ran correctly
			abortWithError(c, http.StatusInternalServerError, "User role not found in context")
			return
		}

		userRole, ok := roleRaw.(domain.Role)
		if !ok {
			// This indicates a programming error (wrong type set in context)
			abortWithError(c, http.StatusInternalServerError, "Invalid user role type in context")
			return
		}

		// Check if the user's role is in the allowed list
		allowed := false
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				allowed = true
				break
			}
		}

		if !allowed {
			abortWithError(c, http.StatusForbidden, fmt.Sprintf("Access denied: Role '%s' does not have permission", userRole))
			return
		}

		// Role is allowed, continue
		c.Next()
	}
}

// Helper function to get User ID from context (used by handlers)
func getUserIDFromContext(c *gin.Context) (string, error) {
	idRaw, exists := c.Get(ContextUserIDKey)
	if !exists {
		return "", errors.New("user ID not found in context")
	}
	idStr, ok := idRaw.(string)
	if !ok {
		return "", errors.New("invalid user ID type in context")
	}
	return idStr, nil
}

// Helper function to get User Role from context (used by handlers)
func getUserRoleFromContext(c *gin.Context) (domain.Role, error) {
	roleRaw, exists := c.Get(ContextUserRoleKey)
	if !exists {
		return "", errors.New("user role not found in context")
	}
	role, ok := roleRaw.(domain.Role)
	if !ok {
		return "", errors.New("invalid user role type in context")
	}
	return role, nil
}
