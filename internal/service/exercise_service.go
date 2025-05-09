package service

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository" // Import repository package
	"context"
	"errors"

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
	CreateExercise(ctx context.Context, trainerID primitive.ObjectID, name, description, muscleGroup, executionTechnic, applicability, difficulty, videoURL string) (*domain.Exercise, error)
	GetExerciseByID(ctx context.Context, exerciseID primitive.ObjectID) (*domain.Exercise, error)
	GetExercisesByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Exercise, error)
	UpdateExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID, name, description, muscleGroup, executionTechnic, applicability, difficulty, videoURL string) (*domain.Exercise, error)
	DeleteExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID) error
}

// --- Service Implementation ---

// exerciseService implements the ExerciseService interface.
type exerciseService struct {
	exerciseRepo repository.ExerciseRepository
}


// NewExerciseService creates a new instance of exerciseService.
func NewExerciseService(exerciseRepo repository.ExerciseRepository) ExerciseService {
	return &exerciseService{
		exerciseRepo: exerciseRepo,
	}
}


// CreateExercise handles the creation of a new exercise by a trainer.
func (s *exerciseService) CreateExercise(ctx context.Context, trainerID primitive.ObjectID, name, description, muscleGroup, executionTechnic, applicability, difficulty, videoURL string) (*domain.Exercise, error) {
	if name == "" {
		return nil, ErrValidationFailed // Example: Name is required
	}
	if trainerID == primitive.NilObjectID {
		return nil, errors.New("trainer ID is required to create an exercise")
	}
	// Add more specific validation for new fields if needed (e.g., difficulty is one of "Novice", "Medium", "Advanced")

	exercise := &domain.Exercise{
		TrainerID:    trainerID,
		Name:         name,
		Description:  description,
		MuscleGroup:  muscleGroup,
		ExecutionTechnic: executionTechnic,
		Applicability: applicability,
		Difficulty:   difficulty,
		VideoURL:     videoURL, // Optional, can be empty
	}

	exerciseID, err := s.exerciseRepo.Create(ctx, exercise)
	if err != nil {
		return nil, err
	}
	exercise.ID = exerciseID
	// To get CreatedAt/UpdatedAt populated by the DB back into the returned object:
	return s.exerciseRepo.GetByID(ctx, exerciseID) // Fetch again to get all fields
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
func (s *exerciseService) UpdateExercise(ctx context.Context, trainerID, exerciseID primitive.ObjectID, name, description, muscleGroup, executionTechnic, applicability, difficulty, videoURL string) (*domain.Exercise, error) {
	if name == "" {
		return nil, ErrValidationFailed
	}
	if trainerID == primitive.NilObjectID || exerciseID == primitive.NilObjectID {
		return nil, errors.New("trainer ID and exercise ID are required")
	}

	existingExercise, err := s.exerciseRepo.GetByID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExerciseNotFound
		}
		return nil, err
	}

	if existingExercise.TrainerID != trainerID {
		return nil, ErrExerciseAccessDenied
	}

	// Update fields
	existingExercise.Name = name
	existingExercise.Description = description
	existingExercise.MuscleGroup = muscleGroup
	existingExercise.ExecutionTechnic = executionTechnic
	existingExercise.Applicability = applicability
	existingExercise.Difficulty = difficulty
	existingExercise.VideoURL = videoURL // Allow updating the video URL

	err = s.exerciseRepo.Update(ctx, existingExercise)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
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
