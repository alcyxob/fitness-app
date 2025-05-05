package service

import (
	"context"
	"errors"
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository"
	"alcyxob/fitness-app/internal/storage" // Import storage package
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
	fileStorage    storage.FileStorage
}

// NewClientService creates a new instance of clientService.
func NewClientService(
	assignmentRepo repository.AssignmentRepository,
	uploadRepo repository.UploadRepository,
	exerciseRepo repository.ExerciseRepository, // Added dependency
	fileStorage storage.FileStorage,
) ClientService {
	return &clientService{
		assignmentRepo: assignmentRepo,
		uploadRepo:     uploadRepo,
		exerciseRepo:   exerciseRepo,
		fileStorage:    fileStorage,
	}
}

// === Assignment Viewing ===

// GetMyAssignments retrieves assignments for the client, enriching them with exercise details.
func (s *clientService) GetMyAssignments(ctx context.Context, clientID primitive.ObjectID) ([]AssignmentDetails, error) {
	if clientID == primitive.NilObjectID {
		return nil, errors.New("client ID is required")
	}

	assignments, err := s.assignmentRepo.GetByClientID(ctx, clientID)
	if err != nil {
		// Repo returns empty slice and nil error if none found
		return nil, err
	}

	if len(assignments) == 0 {
		return []AssignmentDetails{}, nil // Return empty slice explicitly
	}

	// Enrich assignments with exercise details
	// Create a map for efficient lookup if many assignments share exercises
	exerciseIDs := make(map[primitive.ObjectID]struct{})
	for _, a := range assignments {
		exerciseIDs[a.ExerciseID] = struct{}{}
	}

	exerciseMap := make(map[primitive.ObjectID]*domain.Exercise)
	for id := range exerciseIDs {
		// In a real app, consider fetching these in bulk if performance is critical
		exercise, err := s.exerciseRepo.GetByID(ctx, id)
		if err == nil { // Ignore errors for individual exercises? Or fail the whole request?
			exerciseMap[id] = exercise
		} else {
			// Log error: fmt.Printf("WARN: Failed to get exercise %s for assignment enrichment: %v\n", id.Hex(), err)
		}
	}

	// Build the result with enriched data
	detailsList := make([]AssignmentDetails, 0, len(assignments))
	for _, a := range assignments {
		detail := AssignmentDetails{
			Assignment: a,
			Exercise:   exerciseMap[a.ExerciseID], // Will be nil if exercise fetch failed or not found
		}
		detailsList = append(detailsList, detail)
	}

	return detailsList, nil
}

// === Upload Process ===

// RequestUploadURL generates a pre-signed URL for a client to upload a video for an assignment.
func (s *clientService) RequestUploadURL(ctx context.Context, clientID, assignmentID primitive.ObjectID, contentType string) (*UploadURLResponse, error) {
	// 1. Validate Inputs
	if clientID == primitive.NilObjectID || assignmentID == primitive.NilObjectID {
		return nil, errors.New("client ID and assignment ID are required")
	}
	if contentType == "" || !strings.HasPrefix(strings.ToLower(contentType), "video/") {
		// Basic check, more robust validation might be needed
		return nil, errors.New("invalid or missing video content type")
	}

	// 2. Get the assignment & verify ownership and status
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

	if assignment.ClientID != clientID {
		return nil, ErrAssignmentNotBelongToClient
	}

	// Check if upload is allowed based on status (e.g., only if "assigned")
	if assignment.Status != domain.StatusAssigned {
		// Maybe allow re-upload if status is "reviewed"? Depends on business logic.
		return nil, ErrUploadNotAllowed
	}

	// Check if an upload already exists (prevent generating new URL if one is pending/done?)
	if assignment.UploadID != nil && *assignment.UploadID != primitive.NilObjectID {
		// Optionally check upload status if available, or just block/allow overwrite
		// return nil, errors.New("upload already exists for this assignment")
	}

	// 3. Generate a unique object key for S3
	// Format: uploads/{clientId}/{assignmentId}/{uuid}.{extension}
	// Extract extension cautiously, default if needed
	fileExtension := ""
	parts := strings.Split(contentType, "/")
	if len(parts) == 2 {
		fileExtension = parts[1] // e.g., "mp4", "webm"
	}
	uniqueID := uuid.NewString()
	objectKey := fmt.Sprintf("uploads/%s/%s/%s.%s",
		clientID.Hex(),
		assignmentID.Hex(),
		uniqueID,
		fileExtension, // Be cautious if content type doesn't map cleanly to extension
	)
	// Use path.Join for cleaner path construction if needed, though S3 keys are just strings
	objectKey = path.Join("uploads", clientID.Hex(), assignmentID.Hex(), fmt.Sprintf("%s.%s", uniqueID, fileExtension))

	// 4. Generate the pre-signed URL using the FileStorage service
	uploadURL, err := s.fileStorage.GeneratePresignedUploadURL(ctx, objectKey, contentType, storage.DefaultPresignedURLExpiry)
	if err != nil {
		// Log internal error if needed
		return nil, ErrUploadURLError
	}

	// 5. Return the URL and the generated object key
	response := &UploadURLResponse{
		UploadURL: uploadURL,
		ObjectKey: objectKey, // Client needs this to confirm upload later
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
	// Maybe validate filename, fileSize, contentType?

	// 2. Get assignment & verify ownership and status
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}
	if assignment.ClientID != clientID {
		return nil, ErrAssignmentNotBelongToClient
	}
	// Should we check status again? Maybe only allow confirm if 'assigned'?
	if assignment.Status != domain.StatusAssigned && assignment.UploadID == nil {
		// Allow confirm only if status is assigned and no upload exists yet?
		// return nil, ErrUploadNotAllowed // Or maybe allow confirm even if status changed?
	}
	// Optionally verify the provided objectKey format or prefix matches expectations

	// 3. Create Upload metadata object
	upload := &domain.Upload{
		AssignmentID: assignmentID,
		ClientID:     clientID,
		TrainerID:    assignment.TrainerID, // Copy from assignment
		S3ObjectKey:  objectKey,
		FileName:     fileName,
		ContentType:  contentType,
		Size:         fileSize,
		// ID, UploadedAt set by repository
	}

	// 4. Save Upload metadata
	uploadID, err := s.uploadRepo.Create(ctx, upload)
	if err != nil {
		// Log internal error
		// Handle potential errors (e.g., duplicate objectKey if unique index exists)
		return nil, ErrUploadConfirmationFailed // General error
	}

	// 5. Update the Assignment: set UploadID and change Status
	assignment.UploadID = &uploadID
	assignment.Status = domain.StatusSubmitted
	// Optionally clear client notes if submitting resets them?
	// assignment.ClientNotes = ""

	err = s.assignmentRepo.Update(ctx, assignment)
	if err != nil {
		// CRITICAL: Upload metadata created, but assignment update failed!
		// Requires compensation logic: Delete the upload metadata? Or mark assignment as inconsistent?
		// For now, log and return error.
		// log.Errorf("CRITICAL: Failed to update assignment %s after creating upload %s: %v", assignmentID.Hex(), uploadID.Hex(), err)
		// Consider attempting to delete the upload record: s.uploadRepo.Delete(ctx, uploadID)
		return nil, ErrUploadConfirmationFailed // Return error indicating the overall process failed
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
			return "", ErrAssignmentNotFound
		}
		return "", err
	}
	if assignment.ClientID != clientID {
		return "", ErrAssignmentNotBelongToClient
	}

	// 2. Check if an upload exists for this assignment
	if assignment.UploadID == nil || *assignment.UploadID == primitive.NilObjectID {
		return "", ErrUploadMetadataMissing // No upload linked
	}

	// 3. Get Upload metadata to find the S3 object key
	upload, err := s.uploadRepo.GetByID(ctx, *assignment.UploadID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrUploadMetadataMissing // Linked upload record not found
		}
		return "", err // Other repo error
	}

	// Defensive check - should match clientID from assignment, but check anyway
	if upload.ClientID != clientID {
		// Log inconsistency
		return "", ErrAssignmentAccessDenied // Or a different internal error
	}

	// 4. Generate pre-signed download URL
	downloadURL, err := s.fileStorage.GeneratePresignedDownloadURL(ctx, upload.S3ObjectKey, storage.DefaultPresignedURLExpiry)
	if err != nil {
		return "", ErrDownloadURLError
	}

	return downloadURL, nil
}
