package service

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository"
	"context"
	"errors"
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
	ErrTrainingPlanCreationFailed = errors.New("failed to create training plan")
	ErrTrainingPlanNotFound      = errors.New("training plan not found") // If needed later
	ErrWorkoutCreationFailed = errors.New("failed to create workout")
	ErrWorkoutNotFound      = errors.New("workout not found") // If needed later
	ErrTrainingPlanAccessDenied = errors.New("access denied to this training plan")
)

// TrainerService Interface
type TrainerService interface {
	// Client Management
	AddClientByEmail(ctx context.Context, trainerID primitive.ObjectID, clientEmail string) (*domain.User, error)
	GetManagedClients(ctx context.Context, trainerID primitive.ObjectID) ([]domain.User, error)

	// --- Training Plan Methods ---
	CreateTrainingPlan(ctx context.Context, trainerID, clientID primitive.ObjectID, name, description string, startDate, endDate *time.Time, isActive bool) (*domain.TrainingPlan, error)
	GetTrainingPlansForClient(ctx context.Context, trainerID, clientID primitive.ObjectID) ([]domain.TrainingPlan, error)

	// --- NEW Workout Methods ---
	CreateWorkout(ctx context.Context, trainerID, planID primitive.ObjectID, name string, dayOfWeek *int, notes string, sequence int) (*domain.Workout, error)
	GetWorkoutsForPlan(ctx context.Context, trainerID, planID primitive.ObjectID) ([]domain.Workout, error)
	// --- NEW: Assign Exercise to Workout ---
	AssignExerciseToWorkout(ctx context.Context, trainerID, workoutID, exerciseID primitive.ObjectID, assignmentDetails domain.Assignment) (*domain.Assignment, error)
	GetAssignmentsForWorkout(ctx context.Context, trainerID, workoutID primitive.ObjectID) ([]domain.Assignment, error)

	// Existing Assignment Management (will be adapted or removed)
	//GetAssignmentsByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Assignment, error)
	SubmitFeedback(ctx context.Context, trainerID, assignmentID primitive.ObjectID, feedback string, newStatus domain.AssignmentStatus) (*domain.Assignment, error)
}

// --- Service Implementation ---

// trainerService implements the TrainerService interface.
type trainerService struct {
	userRepo          repository.UserRepository
	assignmentRepo    repository.AssignmentRepository
	exerciseRepo      repository.ExerciseRepository
	trainingPlanRepo  repository.TrainingPlanRepository
  workoutRepo repository.WorkoutRepository
}

// NewTrainerService creates a new instance of trainerService.
func NewTrainerService(
	userRepo repository.UserRepository,
	assignmentRepo repository.AssignmentRepository,
	exerciseRepo repository.ExerciseRepository,
	trainingPlanRepo repository.TrainingPlanRepository,
	workoutRepo repository.WorkoutRepository,
	) TrainerService {
		return &trainerService{
			userRepo:          userRepo,
			assignmentRepo:    assignmentRepo,
			exerciseRepo:      exerciseRepo,
			trainingPlanRepo:  trainingPlanRepo,
			workoutRepo:       workoutRepo,
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

// GetAssignmentsByTrainer retrieves assignments created by the trainer.
// func (s *trainerService) GetAssignmentsByTrainer(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Assignment, error) {
// 	// This method is now less meaningful. Assignments belong to workouts/plans.
// 	// Need to decide how trainers view assignments (e.g., fetch plan -> fetch workouts -> fetch assignments?)
// 	// Returning an error or empty slice for now.
// 	return nil, errors.New("GetAssignmentsByTrainer needs reimplementation based on new structure")
// }

func (s *trainerService) CreateWorkout(ctx context.Context, trainerID, planID primitive.ObjectID, name string, dayOfWeek *int, notes string, sequence int) (*domain.Workout, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || planID == primitive.NilObjectID || name == "" {
		return nil, errors.New("trainer ID, plan ID, and workout name are required")
	}
	// Sequence validation? DayOfWeek range?

	// 2. Validate Training Plan Access (Trainer owns the plan)
	plan, err := s.trainingPlanRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTrainingPlanNotFound // Use specific error if defined
		}
		return nil, err
	}
	if plan.TrainerID != trainerID {
		return nil, ErrTrainingPlanAccessDenied
	}

	// 3. Create domain object
	workout := &domain.Workout{
		TrainingPlanID: planID,
		TrainerID:      trainerID,    // Denormalize from plan
		ClientID:       plan.ClientID, // Denormalize from plan
		Name:           name,
		DayOfWeek:      dayOfWeek,
		Notes:          notes,
		Sequence:       sequence,
		// ID, CreatedAt, UpdatedAt set by repo
	}

	// 4. Call repository to save
	workoutID, err := s.workoutRepo.Create(ctx, workout)
	if err != nil {
		// log.Printf("Error saving workout: %v", err)
		return nil, ErrWorkoutCreationFailed
	}

	// 5. Fetch and return the full workout
	createdWorkout, err := s.workoutRepo.GetByID(ctx, workoutID)
	if err != nil {
		// log.Printf("Failed to fetch newly created workout %s: %v", workoutID.Hex(), err)
        workout.ID = workoutID
		return workout, errors.New("workout created, but failed to fetch full details")
	}
	return createdWorkout, nil
}

func (s *trainerService) GetWorkoutsForPlan(ctx context.Context, trainerID, planID primitive.ObjectID) ([]domain.Workout, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || planID == primitive.NilObjectID {
		return nil, errors.New("trainer ID and plan ID are required")
	}

	// 2. Validate Training Plan Access (Trainer owns the plan) - IMPORTANT
	plan, err := s.trainingPlanRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTrainingPlanNotFound
		}
		return nil, err
	}
	if plan.TrainerID != trainerID {
		// This prevents a trainer seeing workouts for a plan they don't own,
        // even if they somehow guess the planID.
		return nil, ErrTrainingPlanAccessDenied
	}

	// 3. Call repository
	workouts, err := s.workoutRepo.GetByPlanID(ctx, planID)
	if err != nil {
		// log.Printf("Error fetching workouts for plan %s: %v", planID.Hex(), err)
		return nil, errors.New("failed to retrieve workouts")
	}
	return workouts, nil
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
	// Fetch the associated Workout to check the TrainerID
	workout, err := s.workoutRepo.GetByID(ctx, assignment.WorkoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					// This implies inconsistent data if assignment exists but workout doesn't
					// log.Printf("Data inconsistency: Assignment %s found, but Workout %s not found", assignmentID.Hex(), assignment.WorkoutID.Hex())
					return nil, ErrWorkoutNotFound // Or a generic server error
			}
			return nil, err // Other repo error
	}

	// Check if the trainer making the request owns the workout associated with the assignment
	if workout.TrainerID != trainerID {
			return nil, ErrAssignmentAccessDenied // Trainer doesn't own the workout this assignment belongs to
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

func (s *trainerService) CreateTrainingPlan(ctx context.Context, trainerID, clientID primitive.ObjectID, name, description string, startDate, endDate *time.Time, isActive bool) (*domain.TrainingPlan, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || clientID == primitive.NilObjectID || name == "" {
		return nil, errors.New("trainer ID, client ID, and plan name are required")
	}

	// 2. Validate Client Relationship (Crucial security/logic check)
	client, err := s.userRepo.GetByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err // Propagate other errors
	}
	if client.Role != domain.RoleClient {
		return nil, errors.New("cannot assign plan: specified user is not a client")
	}
	if client.TrainerID == nil || *client.TrainerID != trainerID {
		return nil, ErrClientNotManaged // Trainer does not manage this client
	}

    // 3. Optional: Logic for isActive flag
    // If setting this plan to active, ensure no other plan for this client is active.
    // This requires an extra repository method like `DeactivateAllPlansForClient` or `GetActivePlanForClient`.
    // For simplicity now, we'll just set it as provided. Add this logic later if needed.
    // if isActive {
    //     err := s.trainingPlanRepo.DeactivateAllPlansForClient(ctx, clientID, trainerID)
    //     // handle error
    // }


	// 4. Create domain object
	plan := &domain.TrainingPlan{
		TrainerID:   trainerID,
		ClientID:    clientID,
		Name:        name,
		Description: description,
		StartDate:   startDate,
		EndDate:     endDate,
		IsActive:    isActive,
		// ID, CreatedAt, UpdatedAt set by repo
	}

	// 5. Call repository to save
	planID, err := s.trainingPlanRepo.Create(ctx, plan)
	if err != nil {
		// log.Printf("Error saving training plan: %v", err)
		return nil, ErrTrainingPlanCreationFailed
	}

	// 6. Fetch and return the full plan with generated fields
	createdPlan, err := s.trainingPlanRepo.GetByID(ctx, planID)
	if err != nil {
		// Log error, but maybe return the partially created plan object?
		// log.Printf("Failed to fetch newly created plan %s: %v", planID.Hex(), err)
        plan.ID = planID // At least set the ID
		return plan, errors.New("plan created, but failed to fetch full details")
	}
	return createdPlan, nil
}

func (s *trainerService) GetTrainingPlansForClient(ctx context.Context, trainerID, clientID primitive.ObjectID) ([]domain.TrainingPlan, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || clientID == primitive.NilObjectID {
		return nil, errors.New("trainer ID and client ID are required")
	}

	// 2. Optional: Re-verify client relationship (already handled by repo query filter, but adds safety)
	// client, err := s.userRepo.GetByID(ctx, clientID)
	// if err != nil { ... handle client not found ... }
	// if client.TrainerID == nil || *client.TrainerID != trainerID { return nil, ErrClientNotManaged }

	// 3. Call repository (repo method already filters by trainerID and clientID)
	plans, err := s.trainingPlanRepo.GetByClientAndTrainerID(ctx, clientID, trainerID)
	if err != nil {
		// log.Printf("Error fetching training plans for client %s by trainer %s: %v", clientID.Hex(), trainerID.Hex(), err)
		return nil, errors.New("failed to retrieve training plans")
	}

	// Repo returns empty slice if none found, not an error
	return plans, nil
}

func (s *trainerService) AssignExerciseToWorkout(ctx context.Context, trainerID, workoutID, exerciseID primitive.ObjectID, assignmentDetails domain.Assignment) (*domain.Assignment, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || workoutID == primitive.NilObjectID || exerciseID == primitive.NilObjectID {
			return nil, errors.New("trainer ID, workout ID, and exercise ID are required")
	}
	// TODO: Add validation for assignmentDetails fields (e.g., sets > 0, valid reps format?)

	// 2. Validate Workout Access (Trainer owns the workout)
	workout, err := s.workoutRepo.GetByID(ctx, workoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrWorkoutNotFound // Use specific error
			}
			return nil, err // Other repo errors
	}
	if workout.TrainerID != trainerID {
			// Check if the trainer owns the workout they are assigning to
			return nil, errors.New("access denied: trainer does not own this workout") // More specific error?
	}

	// 3. Validate Exercise Access (Trainer owns the exercise)
	// Optional but good practice: Ensure the trainer also owns the exercise being assigned.
	exercise, err := s.exerciseRepo.GetByID(ctx, exerciseID)
	if err != nil {
			 if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrExerciseNotFound // Use specific error
			}
			return nil, err
	}
	 if exercise.TrainerID != trainerID {
			return nil, ErrExerciseAccessDenied // Trainer doesn't own the exercise
	 }

	// 4. Populate the full assignment object
	// The caller provides details like Sets, Reps, etc., in assignmentDetails.
	// We just need to ensure the core IDs and potentially sequence are set correctly.
	assignmentDetails.WorkoutID = workoutID
	assignmentDetails.ExerciseID = exerciseID
	// We could potentially fetch existing assignments for the workout to auto-increment sequence,
	// or rely on the caller providing it. Let's assume caller provides it for now.
	// if assignmentDetails.Sequence <= 0 { ... handle default sequence ... }


	// 5. Call repository to save the assignment
	// Assuming assignmentRepo.Create takes the full assignment struct now
	createdAssignmentID, err := s.assignmentRepo.Create(ctx, &assignmentDetails) // Pass pointer
	if err != nil {
			// log.Printf("Error saving assignment in service: %v", err)
			return nil, errors.New("failed to create assignment record")
	}

	// 6. Fetch and return the full assignment with generated fields
	fullAssignment, err := s.assignmentRepo.GetByID(ctx, createdAssignmentID)
	if err != nil {
			// log.Printf("Failed to fetch newly created assignment %s: %v", createdAssignmentID.Hex(), err)
			assignmentDetails.ID = createdAssignmentID // Set ID at least
			return &assignmentDetails, errors.New("assignment created, but failed to retrieve full details")
	}

	return fullAssignment, nil
}

// === NEW GetAssignmentsForWorkout Implementation ===
func (s *trainerService) GetAssignmentsForWorkout(ctx context.Context, trainerID, workoutID primitive.ObjectID) ([]domain.Assignment, error) {
	// 1. Validate Inputs
	if trainerID == primitive.NilObjectID || workoutID == primitive.NilObjectID {
			return nil, errors.New("trainer ID and workout ID are required")
	}

	// 2. Validate Workout Access (Trainer owns the workout)
	workout, err := s.workoutRepo.GetByID(ctx, workoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrWorkoutNotFound
			}
			return nil, err
	}
	if workout.TrainerID != trainerID {
			return nil, errors.New("access denied: trainer does not own this workout")
	}

	// 3. Call repository to get assignments for this workout
	assignments, err := s.assignmentRepo.GetByWorkoutID(ctx, workoutID)
	if err != nil {
			// log.Printf("Error fetching assignments for workout %s: %v", workoutID.Hex(), err)
			return nil, errors.New("failed to retrieve assignments for workout")
	}
	return assignments, nil
}