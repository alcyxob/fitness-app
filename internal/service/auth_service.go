package service

import (
	"context"
	"errors"
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository" // Import repository package
	"time"

	"github.com/golang-jwt/jwt/v4" // Import JWT library
	"golang.org/x/crypto/bcrypt"   // Import bcrypt
)

// --- Error Definitions ---
var (
	ErrUserAlreadyExists    = errors.New("user with this email already exists")
	ErrAuthenticationFailed = errors.New("authentication failed: invalid email or password")
	ErrHashingFailed        = errors.New("failed to hash password")
	ErrTokenGeneration      = errors.New("failed to generate authentication token")
)

// --- Service Interface (Optional but good practice) ---
type AuthService interface {
	Register(ctx context.Context, name, email, password string, role domain.Role) (*domain.User, error)
	Login(ctx context.Context, email, password string) (token string, user *domain.User, err error)
	GetJWTSecret() string
}

// --- Service Implementation ---

// authService implements the AuthService interface.
type authService struct {
	userRepo      repository.UserRepository
	jwtSecret     string
	jwtExpiration time.Duration
}

// NewAuthService creates a new instance of authService.
func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtExpiration time.Duration) AuthService {
	if jwtSecret == "" {
		panic("JWT secret cannot be empty") // Critical configuration
	}
	if jwtExpiration <= 0 {
		jwtExpiration = time.Hour * 1 // Default to 1 hour if not set properly
	}
	return &authService{
		userRepo:      userRepo,
		jwtSecret:     jwtSecret,
		jwtExpiration: jwtExpiration,
	}
}

// Register handles new user registration.
func (s *authService) Register(ctx context.Context, name, email, password string, role domain.Role) (*domain.User, error) {
	// 1. Basic Input Validation (can be expanded)
	if name == "" || email == "" || password == "" || role == "" {
		return nil, errors.New("name, email, password, and role cannot be empty")
	}
	// Add email format validation if desired

	// 2. Check if user already exists
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		// User found, means email is already taken
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, repository.ErrNotFound) {
		// A different error occurred during the check
		return nil, err // Propagate unexpected repository errors
	}
	// If err is ErrNotFound, we can proceed

	// 3. Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, ErrHashingFailed
	}

	// 4. Create the user domain object
	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         role,
		// ID, CreatedAt, UpdatedAt will be set by the repository layer
	}

	// 5. Save the user to the database
	userID, err := s.userRepo.Create(ctx, user)
	if err != nil {
		// Handle potential race condition if another request registered the same email
		// between the GetByEmail check and the Create call, although unique index helps.
		if errors.Is(err, errors.New("user with this email already exists")) { // Adjust if repo returns specific error
			return nil, ErrUserAlreadyExists
		}
		return nil, err // Propagate other creation errors
	}
	user.ID = userID // Set the generated ID back to the user object

	// Optionally fetch the full user object again if Create doesn't return it fully populated
	// newUser, err := s.userRepo.GetByID(ctx, userID)
	// if err != nil { ... handle error ...}
	// return newUser, nil

	// Remove password hash before returning
	user.PasswordHash = ""
	return user, nil
}

// Login handles user authentication and JWT generation.
func (s *authService) Login(ctx context.Context, email, password string) (token string, user *domain.User, err error) {
	// 1. Basic Input Validation
	if email == "" || password == "" {
		err = errors.New("email and password cannot be empty")
		return
	}

	// 2. Fetch user by email
	user, err = s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			err = ErrAuthenticationFailed // User not found maps to auth failure
			return
		}
		// Propagate other repository errors
		return
	}

	// 3. Compare the provided password with the stored hash
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		// Password mismatch (bcrypt returns specific error, but we map to general auth failure)
		err = ErrAuthenticationFailed
		user = nil // Clear user object on failure
		return
	}

	// 4. Authentication successful - Generate JWT
	token, err = s.generateJWT(user)
	if err != nil {
		user = nil // Clear user object on token generation failure
		return "", nil, ErrTokenGeneration
	}

	// Clear password hash before returning user object
	user.PasswordHash = ""
	return token, user, nil // Return token, user details (without hash), and nil error
}

// --- JWT Helper ---

// jwtClaims defines the structure of the JWT payload.
type jwtClaims struct {
	UserID string      `json:"uid"`  // User ID
	Role   domain.Role `json:"role"` // User Role
	jwt.RegisteredClaims
}

// generateJWT creates a new JWT token for the given user.
func (s *authService) generateJWT(user *domain.User) (string, error) {
	// Create the claims
	expirationTime := time.Now().Add(s.jwtExpiration)
	claims := &jwtClaims{
		UserID: user.ID.Hex(), // Convert ObjectID to hex string
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.Hex(), // Subject is often the user ID
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "fitness-app", // Optional: Identify who issued the token
			// Audience: []string{"fitness-clients"}, // Optional: Specify intended recipients
		},
	}

	// Create the token object with the claims and signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	signedToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

// GetJWTSecret returns the JWT secret for middleware authentication
func (s *authService) GetJWTSecret() string {
	return s.jwtSecret
}
