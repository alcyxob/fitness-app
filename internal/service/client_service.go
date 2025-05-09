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
}

// --- Service Implementation ---

// clientService implements the ClientService interface.
type clientService struct {
	assignmentRepo repository.AssignmentRepository
	uploadRepo     repository.UploadRepository
	exerciseRepo   repository.ExerciseRepository // Needed to enrich assignment details
	workoutRepo    repository.WorkoutRepository
	fileStorage    storage.FileStorage
}

// NewClientService creates a new instance of clientService.
func NewClientService(
	assignmentRepo repository.AssignmentRepository,
	uploadRepo repository.UploadRepository,
	exerciseRepo repository.ExerciseRepository, // Added dependency
	workoutRepo    repository.WorkoutRepository,
	fileStorage storage.FileStorage,
) ClientService {
	return &clientService{
		assignmentRepo: assignmentRepo,
		uploadRepo:     uploadRepo,
		exerciseRepo:   exerciseRepo,
		workoutRepo:    workoutRepo,
		fileStorage:    fileStorage,
	}
}

// === Assignment Viewing ===

// GetMyAssignments retrieves assignments for the client, enriching them with exercise details.
func (s *clientService) GetMyAssignments(ctx context.Context, clientID primitive.ObjectID) ([]AssignmentDetails, error) {
	// TODO: Reimplement this method based on the new TrainingPlan -> Workout -> Assignment structure.
	// 1. Find active/relevant TrainingPlan(s) for clientID.
	// 2. Find Workout(s) for those plans.
	// 3. Find Assignment(s) for those workouts.
	// 4. Fetch related Exercise details.
	// 5. Combine into AssignmentDetails.
	fmt.Printf("WARNING: GetMyAssignments in clientService needs reimplementation for new data structure.\n")
	return []AssignmentDetails{}, nil // Return empty for now to avoid errors using old logic
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
	if assignment.Status != domain.StatusAssigned && assignment.Status != domain.StatusReviewed {
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
