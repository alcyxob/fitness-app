package api

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/service"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ExerciseHandler holds the exercise service dependency.
type ExerciseHandler struct {
	exerciseService service.ExerciseService
	// authService service.AuthService // Might be needed if not relying solely on middleware for user ID
}

// NewExerciseHandler creates a new ExerciseHandler.
func NewExerciseHandler(exerciseService service.ExerciseService) *ExerciseHandler {
	return &ExerciseHandler{exerciseService: exerciseService}
}

// --- DTOs for API (Data Transfer Objects) ---

// CreateExerciseRequest defines the expected JSON for creating an exercise.
type CreateExerciseRequest struct {
	Name             string `json:"name" binding:"required"`
	Description      string `json:"description"`
	MuscleGroup      string `json:"muscleGroup" binding:"omitempty"`        // e.g., "Chest", "Legs"
	ExecutionTechnic string `json:"executionTechnic" binding:"omitempty"`  // How to do it
	Applicability    string `json:"applicability" binding:"omitempty"`     // e.g., "Home", "Gym"
	Difficulty       string `json:"difficulty" binding:"omitempty"`        // e.g., "Novice", "Medium", "Advanced"
	VideoURL         string `json:"videoUrl" binding:"omitempty,url"` // Optional, validated as URL if provided
}

// ExerciseResponse is the DTO for returning exercise details.
// Matches the Swift Exercise struct.
type ExerciseResponse struct {
	ID               string    `json:"id"`
	TrainerID        string    `json:"trainerId"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	MuscleGroup      string    `json:"muscleGroup,omitempty"`
	ExecutionTechnic string    `json:"executionTechnic,omitempty"`
	Applicability    string    `json:"applicability,omitempty"`
	Difficulty       string    `json:"difficulty,omitempty"`
	VideoURL         string    `json:"videoUrl,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// MapExerciseToResponse converts a domain.Exercise to ExerciseResponse DTO.
func MapExerciseToResponse(ex *domain.Exercise) ExerciseResponse {
	if ex == nil {
		return ExerciseResponse{}
	}
	return ExerciseResponse{
		ID:               ex.ID.Hex(),
		TrainerID:        ex.TrainerID.Hex(),
		Name:             ex.Name,
		Description:      ex.Description,
		MuscleGroup:      ex.MuscleGroup,
		ExecutionTechnic: ex.ExecutionTechnic,
		Applicability:    ex.Applicability,
		Difficulty:       ex.Difficulty,
		VideoURL:         ex.VideoURL,
		CreatedAt:        ex.CreatedAt,
		UpdatedAt:        ex.UpdatedAt,
	}
}

// MapExercisesToResponse converts a slice of domain.Exercise to a slice of ExerciseResponse DTO.
func MapExercisesToResponse(exercises []domain.Exercise) []ExerciseResponse {
	responses := make([]ExerciseResponse, len(exercises))
	for i, ex := range exercises {
		responses[i] = MapExerciseToResponse(&ex)
	}
	return responses
}


// --- Handler Methods ---

// CreateExercise godoc
// @Summary Create a new exercise
// @Description Creates a new exercise for the authenticated trainer.
// @Tags Exercises
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param exercise body CreateExerciseRequest true "Exercise details"
// @Success 201 {object} ExerciseResponse "Exercise created successfully"
// @Failure 400 {object} gin.H "Invalid input (validation error)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer)"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /exercises [post]
func (h *ExerciseHandler) CreateExercise(c *gin.Context) {
	var req CreateExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}

	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	exercise, err := h.exerciseService.CreateExercise(
		c.Request.Context(),
		trainerID,
		req.Name,
		req.Description,
		req.MuscleGroup,      // Pass new field
		req.ExecutionTechnic, // Pass new field
		req.Applicability,    // Pass new field
		req.Difficulty,       // Pass new field
		req.VideoURL,         // Pass new field (optional)
	)
	if err != nil {
		if errors.Is(err, service.ErrValidationFailed) {
			abortWithError(c, http.StatusBadRequest, err.Error())
		} else {
			// Log the actual error for debugging on the server
			// log.Printf("Error creating exercise: %v", err)
			abortWithError(c, http.StatusInternalServerError, "Failed to create exercise.")
		}
		return
	}

	c.JSON(http.StatusCreated, MapExerciseToResponse(exercise))
}


// GetTrainerExercises godoc
// @Summary Get exercises for the authenticated trainer
// @Description Retrieves all exercises created by the currently authenticated trainer.
// @Tags Exercises
// @Produce json
// @Security BearerAuth
// @Success 200 {array} ExerciseResponse "List of exercises"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer)"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /exercises [get]  <--- THIS IS THE ONE YOUR IOS APP IS CALLING
func (h *ExerciseHandler) GetTrainerExercises(c *gin.Context) {
	// Get trainer ID from JWT claims
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	// Call service
	exercises, err := h.exerciseService.GetExercisesByTrainer(c.Request.Context(), trainerID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve exercises.")
		return
	}

	if exercises == nil { // Service might return nil slice if no error but no exercises
		c.JSON(http.StatusOK, []ExerciseResponse{}) // Return empty array
		return
	}

	c.JSON(http.StatusOK, MapExercisesToResponse(exercises))
}

// TODO: Implement other handlers:
// GetExerciseByID(c *gin.Context)
// UpdateExercise(c *gin.Context)
// DeleteExercise(c *gin.Context)