package mongo

import (
	"context"
	"errors"
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	// "log" // Uncomment if adding logging for indexes
)

const assignmentCollectionName = "assignments"

// mongoAssignmentRepository implements repository.AssignmentRepository
type mongoAssignmentRepository struct {
	collection *mongo.Collection
}

// NewMongoAssignmentRepository creates a new Assignment repository backed by MongoDB.
func NewMongoAssignmentRepository(db *mongo.Database) repository.AssignmentRepository {
	// EnsureAssignmentIndexes(context.Background(), db.Collection(assignmentCollectionName)) // Call during startup
	return &mongoAssignmentRepository{
		collection: db.Collection(assignmentCollectionName),
	}
}

// Create inserts a new assignment into the database.
func (r *mongoAssignmentRepository) Create(ctx context.Context, assignment *domain.Assignment) (primitive.ObjectID, error) {
	// Basic validation
	if assignment.ExerciseID == primitive.NilObjectID ||
		assignment.ClientID == primitive.NilObjectID ||
		assignment.TrainerID == primitive.NilObjectID {
		return primitive.NilObjectID, errors.New("assignment requires exerciseId, clientId, and trainerId")
	}

	assignment.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	assignment.AssignedAt = now  // Set assignment time
	assignment.UpdatedAt = now   // Set initial update time
	if assignment.Status == "" { // Default status if not provided
		assignment.Status = domain.StatusAssigned
	}

	result, err := r.collection.InsertOne(ctx, assignment)
	if err != nil {
		return primitive.NilObjectID, err
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted ID")
	}

	return insertedID, nil
}

// GetByID retrieves an assignment by its ID.
func (r *mongoAssignmentRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Assignment, error) {
	var assignment domain.Assignment
	filter := bson.M{"_id": id}

	err := r.collection.FindOne(ctx, filter).Decode(&assignment)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &assignment, nil
}

// GetByClientID retrieves all assignments for a specific client.
func (r *mongoAssignmentRepository) GetByClientID(ctx context.Context, clientID primitive.ObjectID) ([]domain.Assignment, error) {
	var assignments []domain.Assignment
	filter := bson.M{"clientId": clientID}
	// Sort by assigned date, newest first perhaps? Or due date?
	findOptions := options.Find().SetSort(bson.D{{Key: "assignedAt", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &assignments); err != nil {
		return nil, err
	}
	// Check cursor errors
	if err = cursor.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

// GetByTrainerID retrieves all assignments managed by a specific trainer.
func (r *mongoAssignmentRepository) GetByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Assignment, error) {
	var assignments []domain.Assignment
	filter := bson.M{"trainerId": trainerID}
	// Sort by assigned date or maybe client ID then assigned date
	findOptions := options.Find().SetSort(bson.D{{Key: "assignedAt", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &assignments); err != nil {
		return nil, err
	}
	// Check cursor errors
	if err = cursor.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

// Update modifies an existing assignment. This is a general update method.
// Be cautious when using this; specific update methods (like UpdateStatus) might be safer.
func (r *mongoAssignmentRepository) Update(ctx context.Context, assignment *domain.Assignment) error {
	if assignment.ID == primitive.NilObjectID {
		return errors.New("assignment ID is required for update")
	}

	filter := bson.M{"_id": assignment.ID}

	// Construct the update document carefully to only set fields that should change
	// We don't want to overwrite `assignedAt` or potentially `exerciseId`, `clientId`, `trainerId`
	updateFields := bson.M{
		"status":      assignment.Status,
		"clientNotes": assignment.ClientNotes,
		"feedback":    assignment.Feedback,
		"updatedAt":   time.Now().UTC(), // Always update the timestamp
	}

	// Handle optional fields like DueDate and UploadID correctly
	if assignment.DueDate != nil {
		updateFields["dueDate"] = *assignment.DueDate
	} else {
		// If you want to explicitly unset the due date, use $unset
		// updateFields["$unset"] = bson.M{"dueDate": ""} // Requires adjustment to update structure
		// For simplicity now, we assume nil means "no change" or it was already nil
		// Or maybe the logic should be in the service layer to decide whether to set/unset
	}

	if assignment.UploadID != nil {
		updateFields["uploadId"] = *assignment.UploadID
	} else {
		// Handle unsetting if necessary
		// updateFields["$unset"] = bson.M{"uploadId": ""}
	}

	update := bson.M{"$set": updateFields}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return repository.ErrNotFound // Assignment with that ID didn't exist
	}

	// ModifiedCount == 0 is okay if the data didn't change

	return nil
}

/*
// Example of a more specific update method (Alternative to general Update)
func (r *mongoAssignmentRepository) UpdateStatusAndUpload(ctx context.Context, id primitive.ObjectID, status domain.AssignmentStatus, uploadID primitive.ObjectID, clientNotes string) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"uploadId":    uploadID,
			"clientNotes": clientNotes, // Assuming notes are submitted with upload
			"updatedAt":   time.Now().UTC(),
		},
	}
	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return repository.ErrNotFound
	}
	return nil
}
*/

// EnsureAssignmentIndexes creates necessary indexes for the assignments collection.
func EnsureAssignmentIndexes(ctx context.Context, collection *mongo.Collection) {
	indexes := []mongo.IndexModel{
		{
			// Index for finding assignments by client
			Keys:    bson.D{{Key: "clientId", Value: 1}},
			Options: options.Index(),
		},
		{
			// Index for finding assignments by trainer
			Keys:    bson.D{{Key: "trainerId", Value: 1}},
			Options: options.Index(),
		},
		{
			// Index for potentially finding assignments related to a specific exercise
			Keys:    bson.D{{Key: "exerciseId", Value: 1}},
			Options: options.Index(),
		},
		{
			// Compound index example: finding a specific client's assignments sorted by date
			Keys:    bson.D{{Key: "clientId", Value: 1}, {Key: "assignedAt", Value: -1}},
			Options: options.Index(),
		},
		{
			// Index on status might be useful for dashboards/filtering
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index(),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}
