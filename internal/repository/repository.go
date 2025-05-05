package repository

import (
	"alcyxob/fitness-app/internal/domain" // Import our defined domain models
	"context"                             // Standard for request-scoped deadlines, cancellation signals, etc.

	"go.mongodb.org/mongo-driver/bson/primitive" // For using ObjectIDs
)

// Error constants for repository layer (optional but good practice)
var (
	ErrNotFound     = RepositoryError("not found")
	ErrUpdateFailed = RepositoryError("update failed")
	ErrDeleteFailed = RepositoryError("delete failed")
	// Add more specific errors as needed
)

// RepositoryError helps distinguish repository errors
type RepositoryError string

func (e RepositoryError) Error() string {
	return string(e)
}

// UserRepository defines the interface for interacting with user data.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (primitive.ObjectID, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*domain.User, error)
	AddClientIDToTrainer(ctx context.Context, trainerID, clientID primitive.ObjectID) error
	GetClientsByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.User, error)
	SetTrainerForClient(ctx context.Context, clientID, trainerID primitive.ObjectID) error
	// Update(ctx context.Context, user *domain.User) error // Maybe needed later
	// Delete(ctx context.Context, id primitive.ObjectID) error // Maybe needed later
}

// ExerciseRepository defines the interface for interacting with exercise data.
type ExerciseRepository interface {
	Create(ctx context.Context, exercise *domain.Exercise) (primitive.ObjectID, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Exercise, error)
	GetByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Exercise, error)
	Update(ctx context.Context, exercise *domain.Exercise) error
	Delete(ctx context.Context, id primitive.ObjectID, trainerID primitive.ObjectID) error // Ensure trainer owns the exercise
}

// AssignmentRepository defines the interface for interacting with assignment data.
type AssignmentRepository interface {
	Create(ctx context.Context, assignment *domain.Assignment) (primitive.ObjectID, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Assignment, error)
	GetByClientID(ctx context.Context, clientID primitive.ObjectID) ([]domain.Assignment, error)
	GetByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Assignment, error)
	// Maybe GetByExerciseID ?
	Update(ctx context.Context, assignment *domain.Assignment) error // General update
	// Or more specific updates:
	// UpdateStatus(ctx context.Context, id primitive.ObjectID, status domain.AssignmentStatus) error
	// AddUploadID(ctx context.Context, id primitive.ObjectID, uploadID primitive.ObjectID) error
	// AddFeedback(ctx context.Context, id primitive.ObjectID, feedback string) error
	// Delete(ctx context.Context, id primitive.ObjectID) error // Should assignments be deletable? Or just archived?
}

// UploadRepository defines the interface for interacting with upload metadata.
type UploadRepository interface {
	Create(ctx context.Context, upload *domain.Upload) (primitive.ObjectID, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Upload, error)
	GetByAssignmentID(ctx context.Context, assignmentID primitive.ObjectID) (*domain.Upload, error) // Assuming one upload per assignment? Adjust if multiple allowed.
	// Delete(ctx context.Context, id primitive.ObjectID) error // Usually delete metadata when S3 object is deleted
}
