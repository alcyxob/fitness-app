package service

import (
	"context"
	"errors"
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Error Definitions ---
var (
	ErrClientNotFound         = errors.New("client user not found")
	ErrClientNotRole          = errors.New("user found but is not a client")
	ErrClientAlreadyAssigned  = errors.New("client is already assigned to a trainer")
	ErrClientNotManaged       = errors.New("client is not managed by this trainer")
	ErrAssignmentNotFound     = errors.New("assignment not found") // Re-defined for context, same as repo?
	ErrAssignmentAccessDenied = errors.New("access denied to modify this assignment")
	// Re-use ErrExerciseNotFound, ErrExerciseAccessDenied from ExerciseService or define locally
)

// --- Service Interface (Optional) ---
type TrainerService interface {
	// Client Management
	AddClientByEmail(ctx context.Context, trainerID primitive.ObjectID, clientEmail string) (*domain.User, error)
	GetManagedClients(ctx context.Context, trainerID primitive.ObjectID) ([]domain.User, error)
	// RemoveClient(ctx context.Context, trainerID, clientID primitive.ObjectID) error // TODO: Implement later if needed

	// Assignment Management
	AssignExercise(ctx context.Context, trainerID, clientID, exerciseID primitive.ObjectID, dueDate *time.Time) (*domain.Assignment, error)
	GetAssignmentsByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Assignment, error)
	SubmitFeedback(ctx context.Context, trainerID, assignmentID primitive.ObjectID, feedback string, newStatus domain.AssignmentStatus) (*domain.Assignment, error)
	// UnassignExercise(...) // TODO: Implement later if needed (delete assignment?)
}

// --- Service Implementation ---

// trainerService implements the TrainerService interface.
type trainerService struct {
	userRepo       repository.UserRepository
	assignmentRepo repository.AssignmentRepository
	exerciseRepo   repository.ExerciseRepository
	// uploadRepo     repository.UploadRepository // Potentially needed later to get upload info for feedback
}

// NewTrainerService creates a new instance of trainerService.
func NewTrainerService(
	userRepo repository.UserRepository,
	assignmentRepo repository.AssignmentRepository,
	exerciseRepo repository.ExerciseRepository,
	// uploadRepo repository.UploadRepository,
) TrainerService {
	return &trainerService{
		userRepo:       userRepo,
		assignmentRepo: assignmentRepo,
		exerciseRepo:   exerciseRepo,
		// uploadRepo:     uploadRepo,
	}
}

// === Client Management ===

// AddClientByEmail finds a client by email and assigns them to the trainer.
func (s *trainerService) AddClientByEmail(ctx context.Context, trainerID primitive.ObjectID, clientEmail string) (*domain.User, error) {
	// 1. Validate Input
	if trainerID == primitive.NilObjectID || clientEmail == "" {
		return nil, errors.New("trainer ID and client email are required")
	}

	// 2. Find the potential client user
	client, err := s.userRepo.GetByEmail(ctx, clientEmail)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err // Propagate other errors
	}

	// 3. Verify the user is actually a client
	if client.Role != domain.RoleClient {
		return nil, ErrClientNotRole
	}

	// 4. Check if the client is already assigned to any trainer
	if client.TrainerID != nil && *client.TrainerID != primitive.NilObjectID {
		// Check if it's already assigned to THIS trainer (which might be okay)
		if *client.TrainerID == trainerID {
			// Already managed by this trainer, maybe just return success?
			return client, nil // Or return a specific "already managed" indicator if needed
		}
		// Assigned to a DIFFERENT trainer
		return nil, ErrClientAlreadyAssigned
	}

	// 5. Assign client to trainer (update both records)
	// Add client ID to trainer's list
	err = s.userRepo.AddClientIDToTrainer(ctx, trainerID, client.ID)
	if err != nil {
		// Handle potential repo errors (e.g., trainer not found)
		return nil, err
	}

	// Set trainer ID on client's record
	err = s.userRepo.SetTrainerForClient(ctx, client.ID, trainerID)
	if err != nil {
		// Attempt to rollback the previous step? Or log inconsistency?
		// For now, just return the error. Consider transactional logic if needed.
		return nil, err
	}

	// Return the updated client object (refetch if needed to get updated fields)
	client.TrainerID = &trainerID // Update in memory object for return
	return client, nil
}

// GetManagedClients retrieves the list of clients managed by the trainer.
func (s *trainerService) GetManagedClients(ctx context.Context, trainerID primitive.ObjectID) ([]domain.User, error) {
	if trainerID == primitive.NilObjectID {
		return nil, errors.New("trainer ID is required")
	}
	// The repository method handles fetching the trainer and then the clients
	clients, err := s.userRepo.GetClientsByTrainerID(ctx, trainerID)
	if err != nil {
		// Handle specific errors if the repo distinguishes them (e.g., trainer not found)
		return nil, err
	}
	// Clear password hashes before returning
	for i := range clients {
		clients[i].PasswordHash = ""
	}
	return clients, nil
}

// === Assignment Management ===

// AssignExercise creates a new assignment linking an exercise to a client.
func (s *trainerService) AssignExercise(ctx context.Context, trainerID, clientID, exerciseID primitive.ObjectID, dueDate *time.Time) (*domain.Assignment, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || clientID == primitive.NilObjectID || exerciseID == primitive.NilObjectID {
		return nil, errors.New("trainer ID, client ID, and exercise ID are required")
	}

	// 2. Verify Exercise Ownership and Existence
	exercise, err := s.exerciseRepo.GetByID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrExerciseNotFound // Use shared error if defined elsewhere
		}
		return nil, err
	}
	if exercise.TrainerID != trainerID {
		return nil, ErrExerciseAccessDenied // Use shared error if defined elsewhere
	}

	// 3. Verify Client is Managed by this Trainer
	client, err := s.userRepo.GetByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	if client.TrainerID == nil || *client.TrainerID != trainerID {
		return nil, ErrClientNotManaged
	}

	// 4. Create Assignment domain object
	assignment := &domain.Assignment{
		ExerciseID: exerciseID,
		ClientID:   clientID,
		TrainerID:  trainerID, // Denormalized for easier queries by trainer
		Status:     domain.StatusAssigned,
		DueDate:    dueDate,
		// ID, AssignedAt, UpdatedAt set by repository
	}

	// 5. Save assignment
	assignmentID, err := s.assignmentRepo.Create(ctx, assignment)
	if err != nil {
		return nil, err
	}
	assignment.ID = assignmentID
	// Refetch if timestamps needed: return s.assignmentRepo.GetByID(ctx, assignmentID)
	return assignment, nil
}

// GetAssignmentsByTrainer retrieves assignments created by the trainer.
func (s *trainerService) GetAssignmentsByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Assignment, error) {
	if trainerID == primitive.NilObjectID {
		return nil, errors.New("trainer ID is required")
	}
	assignments, err := s.assignmentRepo.GetByTrainerID(ctx, trainerID)
	if err != nil {
		return nil, err
	}
	return assignments, nil
}

// SubmitFeedback updates an assignment with feedback and potentially a new status.
func (s *trainerService) SubmitFeedback(ctx context.Context, trainerID, assignmentID primitive.ObjectID, feedback string, newStatus domain.AssignmentStatus) (*domain.Assignment, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || assignmentID == primitive.NilObjectID {
		return nil, errors.New("trainer ID and assignment ID are required")
	}
	// Validate feedback length, content? Validate status transition?
	if newStatus != "" && !(newStatus == domain.StatusReviewed || newStatus == domain.StatusCompleted /*|| other valid transitions*/) {
		return nil, errors.New("invalid status transition for feedback")
	}

	// 2. Get the assignment
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

	// 3. Authorization Check: Ensure trainer owns this assignment
	if assignment.TrainerID != trainerID {
		return nil, ErrAssignmentAccessDenied
	}

	// 4. Update fields
	assignment.Feedback = feedback
	if newStatus != "" {
		assignment.Status = newStatus
	}
	// UpdatedAt will be set by repository

	// 5. Save changes
	err = s.assignmentRepo.Update(ctx, assignment)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) { // Should not happen due to prior Get
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

	return assignment, nil
}
