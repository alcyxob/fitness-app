package service

import (
	"context"
	"errors"
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository" // Import repository package

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Error Definitions ---
var (
	ErrExerciseNotFound     = errors.New("exercise not found")
	ErrExerciseAccessDenied = errors.New("access denied to modify or delete this exercise")
	ErrValidationFailed     = errors.New("exercise validation failed")
)

// --- Service Interface (Optional) ---
type ExerciseService interface {
	CreateExercise(ctx context.Context, trainerID primitive.ObjectID, name, description, instructions, videoURL string) (*domain.Exercise, error)
	GetExerciseByID(ctx context.Context, exerciseID primitive.ObjectID) (*domain.Exercise, error)
	GetExercisesByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Exercise, error)
	UpdateExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID, name, description, instructions, videoURL string) (*domain.Exercise, error)
	DeleteExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID) error
}

// --- Service Implementation ---

// exerciseService implements the ExerciseService interface.
type exerciseService struct {
	exerciseRepo repository.ExerciseRepository
	// userRepo repository.UserRepository // Might be needed later for more complex validation/auth checks
}

// NewExerciseService creates a new instance of exerciseService.
func NewExerciseService(exerciseRepo repository.ExerciseRepository) ExerciseService {
	return &exerciseService{
		exerciseRepo: exerciseRepo,
	}
}

// CreateExercise handles the creation of a new exercise by a trainer.
func (s *exerciseService) CreateExercise(ctx context.Context, trainerID primitive.ObjectID, name, description, instructions, videoURL string) (*domain.Exercise, error) {
	// 1. Validation
	if name == "" {
		return nil, ErrValidationFailed // Example: Name is required
	}
	if trainerID == primitive.NilObjectID {
		return nil, errors.New("trainer ID is required to create an exercise")
	}
	// Add more validation as needed (e.g., length limits, URL format)

	// 2. Create domain object
	exercise := &domain.Exercise{
		TrainerID:    trainerID,
		Name:         name,
		Description:  description,
		Instructions: instructions,
		VideoURL:     videoURL,
		// ID, CreatedAt, UpdatedAt set by repository
	}

	// 3. Call repository to save
	exerciseID, err := s.exerciseRepo.Create(ctx, exercise)
	if err != nil {
		// Log internal errors if needed
		return nil, err // Propagate repository errors
	}
	exercise.ID = exerciseID // Set the generated ID

	// Optionally fetch the full object if Create doesn't return it fully populated
	// return s.GetExerciseByID(ctx, exerciseID) // If timestamps etc., are needed immediately

	return exercise, nil
}

// GetExerciseByID retrieves a single exercise.
// Note: Current implementation doesn't enforce access control here.
// Access control might be handled at the API layer or added here if needed
// (e.g., check if user requesting is the owner trainer or an assigned client).
func (s *exerciseService) GetExerciseByID(ctx context.Context, exerciseID primitive.ObjectID) (*domain.Exercise, error) {
	exercise, err := s.exerciseRepo.GetByID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExerciseNotFound
		}
		return nil, err // Propagate other repository errors
	}
	return exercise, nil
}

// GetExercisesByTrainer retrieves all exercises for a specific trainer.
func (s *exerciseService) GetExercisesByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Exercise, error) {
	if trainerID == primitive.NilObjectID {
		return nil, errors.New("trainer ID cannot be nil")
	}
	exercises, err := s.exerciseRepo.GetByTrainerID(ctx, trainerID)
	if err != nil {
		// Repository layer doesn't typically return ErrNotFound for list operations,
		// it would return an empty slice and nil error.
		return nil, err // Propagate repository errors
	}
	return exercises, nil
}

// UpdateExercise handles updating an existing exercise, ensuring ownership.
func (s *exerciseService) UpdateExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID, name, description, instructions, videoURL string) (*domain.Exercise, error) {
	// 1. Validation
	if name == "" {
		return nil, ErrValidationFailed
	}
	if trainerID == primitive.NilObjectID || exerciseID == primitive.NilObjectID {
		return nil, errors.New("trainer ID and exercise ID are required")
	}

	// 2. Fetch existing exercise to check ownership
	existingExercise, err := s.exerciseRepo.GetByID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExerciseNotFound
		}
		return nil, err // Propagate other errors
	}

	// 3. Authorization Check: Ensure the trainer owns this exercise
	if existingExercise.TrainerID != trainerID {
		return nil, ErrExerciseAccessDenied
	}

	// 4. Update fields on the fetched object
	existingExercise.Name = name
	existingExercise.Description = description
	existingExercise.Instructions = instructions
	existingExercise.VideoURL = videoURL
	// UpdatedAt will be set by the repository Update method

	// 5. Call repository to save changes
	err = s.exerciseRepo.Update(ctx, existingExercise)
	if err != nil {
		// Handle potential repository errors (like ErrNotFound if somehow deleted between Get and Update)
		if errors.Is(err, repository.ErrNotFound) { // Should ideally not happen due to prior check, but good practice
			return nil, ErrExerciseNotFound
		}
		return nil, err
	}

	return existingExercise, nil
}

// DeleteExercise handles deleting an exercise, ensuring ownership.
func (s *exerciseService) DeleteExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID) error {
	if trainerID == primitive.NilObjectID || exerciseID == primitive.NilObjectID {
		return errors.New("trainer ID and exercise ID are required")
	}

	// Note: The repository's Delete method already includes the trainerID check in its filter.
	// So, calling it directly enforces ownership at the DB level.
	// We could add an explicit GetByID check first (like in Update) for a more specific
	// error message ("not found" vs "access denied"), but relying on the repo's
	// combined filter is also efficient.

	err := s.exerciseRepo.Delete(ctx, exerciseID, trainerID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// This error from the repo could mean "not found" OR "found but wrong trainer".
			// Map to a consistent service error. ErrExerciseNotFound or ErrExerciseAccessDenied might be suitable.
			// Let's use NotFound for simplicity, assuming the API layer already confirmed the user is the trainer.
			return ErrExerciseNotFound // Or ErrExerciseAccessDenied if preferred
		}
		// Propagate other repository errors (e.g., db connection issues)
		return err
	}

	// Optional: Check if any assignments use this exercise and handle cleanup?
	// This adds complexity (would need AssignmentRepository dependency) and depends on business rules.
	// For now, we just delete the exercise itself.

	return nil
}
