package service

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository"
	"alcyxob/fitness-app/internal/storage" // Import storage package
	"context"
	"errors"
	"fmt"
	"path" // For constructing object keys
	"strings"
	"time"

	"github.com/google/uuid" // For generating unique identifiers for S3 keys
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Error Definitions ---
var (
	ErrAssignmentNotBelongToClient = errors.New("assignment does not belong to this client")
	ErrUploadNotAllowed            = errors.New("upload is not allowed for this assignment status")
	ErrUploadConfirmationFailed    = errors.New("failed to confirm upload")
	ErrUploadURLError              = errors.New("failed to generate upload URL")
	ErrDownloadURLError            = errors.New("failed to generate download URL")
	ErrUploadMetadataMissing       = errors.New("upload metadata is missing")
	ErrClientHasNoActivePlan = errors.New("client does not have an active training plan")
	ErrPlanNotAssignedToClient = errors.New("this training plan is not assigned to the client")
	ErrWorkoutNotBelongToPlan = errors.New("this workout does not belong to the specified plan for this client")
	ErrInvalidAssignmentStatusUpdate = errors.New("invalid status update for assignment")
)

// --- Service Interface (Optional) ---

// UploadURLResponse structure for returning URL and object key
type UploadURLResponse struct {
	UploadURL string `json:"uploadUrl"`
	ObjectKey string `json:"objectKey"` // The key client needs to report back on confirm
}

// AssignmentDetails combines assignment with exercise and potentially download URL
type AssignmentDetails struct {
	domain.Assignment
	Exercise       *domain.Exercise `json:"exercise"`       // Details of the assigned exercise
	VideoUploadURL *string          `json:"videoUploadUrl"` // Temporary URL to view the client's upload
}

type ClientService interface {
	// Assignment Viewing
	GetMyAssignments(ctx context.Context, clientID primitive.ObjectID) ([]AssignmentDetails, error)

	// Upload Process
	RequestUploadURL(ctx context.Context, clientID, assignmentID primitive.ObjectID, contentType string) (*UploadURLResponse, error)
	ConfirmUpload(ctx context.Context, clientID, assignmentID primitive.ObjectID, objectKey, fileName string, fileSize int64, contentType string) (*domain.Assignment, error)

	// Optional: Get download URL for client's own video
	GetMyVideoDownloadURL(ctx context.Context, clientID, assignmentID primitive.ObjectID) (string, error)

	GetMyActiveTrainingPlans(ctx context.Context, clientID primitive.ObjectID) ([]domain.TrainingPlan, error) // Could also be GetMyTrainingPlans
	GetWorkoutsForMyPlan(ctx context.Context, clientID, planID primitive.ObjectID) ([]domain.Workout, error)
	GetAssignmentsForMyWorkout(ctx context.Context, clientID, workoutID primitive.ObjectID) ([]domain.Assignment, error)
	UpdateMyAssignmentStatus(ctx context.Context, clientID, assignmentID primitive.ObjectID, newStatus domain.AssignmentStatus) (*domain.Assignment, error)
	LogPerformanceForMyAssignment(ctx context.Context, clientID, assignmentID primitive.ObjectID, performanceData domain.Assignment) (*domain.Assignment, error)
}

// --- Service Implementation ---

// clientService implements the ClientService interface.
type clientService struct {
	userRepo          repository.UserRepository
	assignmentRepo    repository.AssignmentRepository
	uploadRepo        repository.UploadRepository
	exerciseRepo      repository.ExerciseRepository // Still needed to enrich assignments with exercise details
	workoutRepo       repository.WorkoutRepository
	trainingPlanRepo  repository.TrainingPlanRepository 
	fileStorage       storage.FileStorage
}

// NewClientService creates a new instance of clientService.
func NewClientService(
	userRepo         repository.UserRepository,
	assignmentRepo repository.AssignmentRepository,
	uploadRepo repository.UploadRepository,
	exerciseRepo repository.ExerciseRepository, // Added dependency
	workoutRepo    repository.WorkoutRepository,
	trainingPlanRepo repository.TrainingPlanRepository,
	fileStorage storage.FileStorage,
) ClientService {
	return &clientService{
		userRepo:         userRepo,
		assignmentRepo: assignmentRepo,
		uploadRepo:     uploadRepo,
		exerciseRepo:   exerciseRepo,
		workoutRepo:    workoutRepo,
		trainingPlanRepo:  trainingPlanRepo,
		fileStorage:    fileStorage,
	}
}

// This method is now superseded by the new granular methods.
// It needs to be updated or removed to avoid confusion.
func (s *clientService) GetMyAssignments(ctx context.Context, clientID primitive.ObjectID) ([]AssignmentDetails, error) {
	fmt.Printf("WARNING: GetMyAssignments in clientService is DEPRECATED and needs complete reimplementation based on new TrainingPlan -> Workout -> Assignment structure.\n")
	// This old implementation is likely broken.
	return []AssignmentDetails{}, errors.New("GetMyAssignments is deprecated; use GetAssignmentsForMyWorkout")
}

// === Upload Process ===

// RequestUploadURL generates a pre-signed URL for a client to upload a video for an assignment.
func (s *clientService) RequestUploadURL(ctx context.Context, clientID, assignmentID primitive.ObjectID, contentType string) (*UploadURLResponse, error) {
	// 1. Validate Inputs
	if clientID == primitive.NilObjectID || assignmentID == primitive.NilObjectID {
		return nil, errors.New("client ID and assignment ID are required")
	}
	if contentType == "" || !strings.HasPrefix(strings.ToLower(contentType), "video/") {
		return nil, errors.New("invalid or missing video content type")
	}

	// 2. Get the assignment
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

    // --- CORRECTED AUTHORIZATION CHECK ---
    // Fetch the associated Workout using WorkoutID from the assignment
    workout, err := s.workoutRepo.GetByID(ctx, assignment.WorkoutID)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            // log.Printf("Data inconsistency: Assignment %s found, but Workout %s not found", assignmentID.Hex(), assignment.WorkoutID.Hex())
            return nil, ErrWorkoutNotFound
        }
        return nil, errors.New("failed to verify workout for assignment")
    }
    // Check if the ClientID on the Workout matches the requesting ClientID
    if workout.ClientID != clientID {
        return nil, ErrAssignmentNotBelongToClient
    }
    // --- END CORRECTED AUTHORIZATION CHECK ---


	// 3. Check if upload is allowed based on status
	if assignment.Status != domain.StatusAssigned && 
		 assignment.Status != domain.StatusReviewed &&
		 assignment.Status != domain.StatusCompleted {
		return nil, ErrUploadNotAllowed
	}

	// ... (rest of the function: generate key, generate URL) ...
	// 4. Generate a unique object key for S3
	uniqueID := uuid.NewString()
	fileExtension := ""
	parts := strings.Split(contentType, "/")
	if len(parts) == 2 { fileExtension = parts[1] }
	objectKey := path.Join("uploads", clientID.Hex(), assignmentID.Hex(), fmt.Sprintf("%s.%s", uniqueID, fileExtension))

	// 5. Generate the pre-signed URL
	uploadURL, err := s.fileStorage.GeneratePresignedUploadURL(ctx, objectKey, contentType, storage.DefaultPresignedURLExpiry)
	if err != nil {
		return nil, ErrUploadURLError
	}

	response := &UploadURLResponse{
		UploadURL: uploadURL,
		ObjectKey: objectKey,
	}
	return response, nil
}

// ConfirmUpload creates the Upload metadata record and updates the Assignment status.
// This is called AFTER the client has successfully uploaded the file to S3 using the pre-signed URL.
func (s *clientService) ConfirmUpload(ctx context.Context, clientID, assignmentID primitive.ObjectID, objectKey, fileName string, fileSize int64, contentType string) (*domain.Assignment, error) {
	// 1. Validate Inputs
	if clientID == primitive.NilObjectID || assignmentID == primitive.NilObjectID || objectKey == "" {
		return nil, errors.New("client ID, assignment ID, and object key are required")
	}

	// 2. Get assignment
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

    // --- CORRECTED AUTHORIZATION CHECK ---
    // Fetch the associated Workout using WorkoutID from the assignment
    workout, err := s.workoutRepo.GetByID(ctx, assignment.WorkoutID)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            // log.Printf("Data inconsistency: Assignment %s found, but Workout %s not found", assignmentID.Hex(), assignment.WorkoutID.Hex())
            return nil, ErrWorkoutNotFound
        }
        return nil, errors.New("failed to verify workout for assignment")
    }
    // Check if the ClientID on the Workout matches the requesting ClientID
    if workout.ClientID != clientID {
        return nil, ErrAssignmentNotBelongToClient
    }
    // --- END CORRECTED AUTHORIZATION CHECK ---


	// Check status? Only allow confirm if 'assigned' or 'reviewed'?
	// if assignment.Status != domain.StatusAssigned && assignment.Status != domain.StatusReviewed { ... }


	// --- CORRECTED Upload metadata object Creation ---
	// Get TrainerID from the fetched workout, not the (now non-existent) field on assignment
	upload := &domain.Upload{
		AssignmentID: assignmentID,
		ClientID:     clientID,       // This is the client confirming the upload
		TrainerID:    workout.TrainerID, // <<< Get TrainerID from the Workout
		S3ObjectKey:  objectKey,
		FileName:     fileName,
		ContentType:  contentType,
		Size:         fileSize,
		// ID, UploadedAt set by repository
	}
    // --- END CORRECTION ---


	// 4. Save Upload metadata
	uploadID, err := s.uploadRepo.Create(ctx, upload)
	if err != nil {
		// log.Printf("Error saving upload metadata: %v", err)
		return nil, ErrUploadConfirmationFailed
	}

	// 5. Update the Assignment: set UploadID and change Status
	assignment.UploadID = &uploadID
	assignment.Status = domain.StatusSubmitted // Set status to submitted

	err = s.assignmentRepo.Update(ctx, assignment)
	if err != nil {
		// CRITICAL: Attempt compensation - delete the upload metadata we just created
        // log.Printf("CRITICAL: Failed to update assignment %s after creating upload %s. Attempting to delete upload.", assignmentID.Hex(), uploadID.Hex())
        // deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Short timeout for cleanup
        // defer cancel()
        // if deleteErr := s.uploadRepo.Delete(deleteCtx, uploadID); deleteErr != nil {
        //      log.Printf("CRITICAL: Failed to delete upload metadata %s during compensation: %v", uploadID.Hex(), deleteErr)
        // }
		return nil, ErrUploadConfirmationFailed
	}

	// 6. Return the updated assignment
	return assignment, nil
}

// GetMyVideoDownloadURL generates a temporary URL for the client to view their own uploaded video.
func (s *clientService) GetMyVideoDownloadURL(ctx context.Context, clientID, assignmentID primitive.ObjectID) (string, error) {
	// 1. Get assignment & verify ownership
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return "", ErrAssignmentNotFound // Use specific error
			}
			return "", err // Other repo error
	}

	// 2. Check if an upload exists for this assignment
	workout, err := s.workoutRepo.GetByID(ctx, assignment.WorkoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					// Data inconsistency or invalid assignmentID
					// log.Printf("Data inconsistency: Assignment %s found, but Workout %s not found", assignmentID.Hex(), assignment.WorkoutID.Hex())
					return "", ErrWorkoutNotFound // Use specific workout error
			}
			// Log other workout repo errors
			return "", errors.New("failed to verify workout for assignment")
	}

	// 3. Check if the ClientID on the Workout matches the requesting ClientID
	if workout.ClientID != clientID {
		// The user making the request is not the client this workout (and thus assignment) belongs to
				return "", ErrAssignmentNotBelongToClient // Keep this error or use a generic auth error
		}

	// 4. Check if an upload exists for this assignment
	if assignment.UploadID == nil || *assignment.UploadID == primitive.NilObjectID {
			return "", ErrUploadMetadataMissing // No upload linked
	}

	// 5. Get Upload metadata to find the S3 object key
	upload, err := s.uploadRepo.GetByID(ctx, *assignment.UploadID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return "", ErrUploadMetadataMissing // Linked upload record not found
			}
			return "", err // Other repo error
	}

	// 6. Generate pre-signed download URL
	downloadURL, err := s.fileStorage.GeneratePresignedDownloadURL(ctx, upload.S3ObjectKey, storage.DefaultPresignedURLExpiry)
	if err != nil {
			return "", ErrDownloadURLError
	}

	return downloadURL, nil
}

// GetMyActiveTrainingPlans fetches active (or all) training plans for the given client.
func (s *clientService) GetMyActiveTrainingPlans(ctx context.Context, clientID primitive.ObjectID) ([]domain.TrainingPlan, error) {
	if clientID == primitive.NilObjectID {
			return nil, errors.New("client ID is required")
	}
	// The repository method GetByClientAndTrainerID could be adapted or a new one GetByClientID
	// For now, let's assume a client can only have plans from ONE trainer at a time.
	// We'd need to fetch the client to find their trainer if the repo needs trainerId.
	clientUser, err := s.userRepo.GetByID(ctx, clientID) // Assuming userRepo is added to clientService, or get trainerID differently
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrClientNotFound
			}
			return nil, err
	}
	if clientUser.TrainerID == nil || *clientUser.TrainerID == primitive.NilObjectID {
			return []domain.TrainingPlan{}, nil // No trainer, so no plans from a trainer
	}

	// Fetch plans assigned by this client's current trainer
	// The GetByClientAndTrainerID already filters appropriately.
	plans, err := s.trainingPlanRepo.GetByClientAndTrainerID(ctx, clientID, *clientUser.TrainerID)
	if err != nil {
			// log.Printf("Error fetching plans for client %s: %v", clientID.Hex(), err)
			return nil, errors.New("failed to retrieve training plans")
	}
	// Optionally filter for IsActive == true here if the method name implies only active
	// var activePlans []domain.TrainingPlan
	// for _, p := range plans {
	//     if p.IsActive {
	//         activePlans = append(activePlans, p)
	//     }
	// }
	// return activePlans, nil
	return plans, nil // Returning all plans for now, client can see active flag
}

// GetWorkoutsForMyPlan fetches workouts for a specific plan IF that plan belongs to the client.
func (s *clientService) GetWorkoutsForMyPlan(ctx context.Context, clientID, planID primitive.ObjectID) ([]domain.Workout, error) {
	if clientID == primitive.NilObjectID || planID == primitive.NilObjectID {
			return nil, errors.New("client ID and plan ID are required")
	}

	// 1. Verify the plan exists and is actually assigned to this client
	plan, err := s.trainingPlanRepo.GetByID(ctx, planID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrTrainingPlanNotFound
			}
			return nil, err
	}
	if plan.ClientID != clientID {
			return nil, ErrPlanNotAssignedToClient // Security check
	}

	// 2. Fetch workouts for this validated plan
	workouts, err := s.workoutRepo.GetByPlanID(ctx, planID)
	if err != nil {
			// log.Printf("Error fetching workouts for client's plan %s: %v", planID.Hex(), err)
			return nil, errors.New("failed to retrieve workouts for the plan")
	}
	return workouts, nil
}

// GetAssignmentsForMyWorkout fetches assignments for a workout IF it belongs to client's plan.
func (s *clientService) GetAssignmentsForMyWorkout(ctx context.Context, clientID, workoutID primitive.ObjectID) ([]domain.Assignment, error) {
	if clientID == primitive.NilObjectID || workoutID == primitive.NilObjectID {
			return nil, errors.New("client ID and workout ID are required")
	}

	// 1. Verify the workout exists and is actually assigned to this client
	workout, err := s.workoutRepo.GetByID(ctx, workoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrWorkoutNotFound
			}
			return nil, err
	}
	if workout.ClientID != clientID { // Check via the denormalized ClientID on Workout
			return nil, ErrWorkoutNotBelongToPlan // Or a more generic auth error
	}

	// 2. Fetch assignments for this validated workout
	assignments, err := s.assignmentRepo.GetByWorkoutID(ctx, workoutID)
	if err != nil {
			// log.Printf("Error fetching assignments for client's workout %s: %v", workoutID.Hex(), err)
			return nil, errors.New("failed to retrieve assignments for the workout")
	}
	return assignments, nil
}

func (s *clientService) UpdateMyAssignmentStatus(ctx context.Context, clientID, assignmentID primitive.ObjectID, newStatus domain.AssignmentStatus) (*domain.Assignment, error) {
	if clientID == primitive.NilObjectID || assignmentID == primitive.NilObjectID {
			return nil, errors.New("client ID and assignment ID are required")
	}
	if newStatus == "" { // Or validate against a list of allowed client-settable statuses
			return nil, errors.New("new status cannot be empty")
	}
	// Example: Client can only set to "completed" or maybe "skipped"
	if newStatus != domain.StatusCompleted { // && newStatus != domain.StatusSkipped (if you add it)
			return nil, ErrInvalidAssignmentStatusUpdate // Client tries to set to "reviewed" etc.
	}


	// 1. Get the assignment
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					return nil, ErrAssignmentNotFound
			}
			return nil, err
	}

	// 2. Authorization: Verify assignment belongs to client (via workout)
	workout, err := s.workoutRepo.GetByID(ctx, assignment.WorkoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					// log.Printf("Data inconsistency: Assignment %s found, but Workout %s not found", assignmentID.Hex(), assignment.WorkoutID.Hex())
					return nil, ErrWorkoutNotFound
			}
			return nil, errors.New("failed to verify workout for assignment status update")
	}
	if workout.ClientID != clientID {
			return nil, ErrAssignmentNotBelongToClient
	}

	// 3. Update status and save
	// Optional: Add logic here to prevent updating status if already "reviewed" by trainer, etc.
	// if assignment.Status == domain.StatusReviewed { ... return error ... }

	assignment.Status = newStatus
	assignment.UpdatedAt = time.Now().UTC() // Ensure UpdatedAt is modified

	err = s.assignmentRepo.Update(ctx, assignment) // This update should persist all fields of assignment
	if err != nil {
			// log.Printf("Error updating assignment status for %s: %v", assignmentID.Hex(), err)
			return nil, errors.New("failed to update assignment status")
	}

	// Return the updated assignment (Update might not return the full object, so GetByID again if needed)
	// The current s.assignmentRepo.Update doesn't return the updated object.
	// For simplicity, we return the modified local 'assignment' object.
	// To be fully robust, fetch it again:
	// return s.assignmentRepo.GetByID(ctx, assignmentID)
	return assignment, nil
}

func (s *clientService) LogPerformanceForMyAssignment(ctx context.Context, clientID, assignmentID primitive.ObjectID, performanceData domain.Assignment) (*domain.Assignment, error) {
	if clientID == primitive.NilObjectID || assignmentID == primitive.NilObjectID {
			return nil, errors.New("client ID and assignment ID are required")
	}

	// 1. Get the existing assignment
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) { return nil, ErrAssignmentNotFound }
			return nil, err
	}

	// 2. Authorization: Verify assignment belongs to this client (via workout)
	workout, err := s.workoutRepo.GetByID(ctx, assignment.WorkoutID)
	if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
					// log.Printf("Data inconsistency: Workout %s not found for assignment %s", assignment.WorkoutID.Hex(), assignmentID.Hex())
					return nil, ErrWorkoutNotFound
			}
			return nil, errors.New("failed to verify workout for performance logging")
	}
	if workout.ClientID != clientID {
			return nil, ErrAssignmentNotBelongToClient
	}

	// 3. Update only the performance-related fields on the fetched assignment object
	// The 'performanceData' struct only carries the fields being updated.
	assignment.AchievedSets = performanceData.AchievedSets
	assignment.AchievedReps = performanceData.AchievedReps
	assignment.AchievedWeight = performanceData.AchievedWeight
	assignment.AchievedDuration = performanceData.AchievedDuration
	assignment.ClientPerformanceNotes = performanceData.ClientPerformanceNotes

	// 4. Optionally, update status here too.
	// For example, if logging performance implies it's at least "completed".
	// Or client might have already marked it "completed" and is now adding details.
	if assignment.Status == domain.StatusAssigned { // If it was just assigned, now it's at least completed
			 assignment.Status = domain.StatusCompleted
	}
	// If they log performance for an already "submitted" or "reviewed" item,
	// should the status change? Probably not unless explicitly requested.
	// For now, let's not change status if it's beyond "assigned".

	assignment.UpdatedAt = time.Now().UTC()

	// 5. Save changes
	err = s.assignmentRepo.Update(ctx, assignment)
	if err != nil {
			// log.Printf("Error saving performance log for assignment %s: %v", assignmentID.Hex(), err)
			return nil, errors.New("failed to log performance")
	}
	
	// Return the fully updated assignment.
	// The local 'assignment' object has been modified and then saved.
	// assignmentRepo.Update does not return the object, so if we want the absolute latest from DB (e.g. _rev field if using Couch/Pouch)
	// we would refetch. For Mongo, returning the modified local object is usually fine.
	return assignment, nil
}