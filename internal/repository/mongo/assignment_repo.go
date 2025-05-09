package mongo

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository"
	"context"
	"errors"
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
	// Basic validation - check IDs that ARE part of the struct
	if assignment.WorkoutID == primitive.NilObjectID || // Check WorkoutID
		assignment.ExerciseID == primitive.NilObjectID { // Check ExerciseID
		return primitive.NilObjectID, errors.New("assignment requires workoutId and exerciseId")
        // REMOVED: Checks for ClientID, TrainerID
	}

	assignment.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	assignment.AssignedAt = now // Set assignment time
	assignment.UpdatedAt = now  // Set initial update time
	if assignment.Status == "" { // Default status if not provided
		assignment.Status = domain.StatusAssigned
	}

	result, err := r.collection.InsertOne(ctx, assignment)
	if err != nil {
		return primitive.NilObjectID, err
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted assignment ID")
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
// func (r *mongoAssignmentRepository) GetByClientID(ctx context.Context, clientID primitive.ObjectID) ([]domain.Assignment, error) {
// 	var assignments []domain.Assignment
// 	filter := bson.M{"clientId": clientID}
// 	// Sort by assigned date, newest first perhaps? Or due date?
// 	findOptions := options.Find().SetSort(bson.D{{Key: "assignedAt", Value: -1}})

// 	cursor, err := r.collection.Find(ctx, filter, findOptions)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer cursor.Close(ctx)

// 	if err = cursor.All(ctx, &assignments); err != nil {
// 		return nil, err
// 	}
// 	// Check cursor errors
// 	if err = cursor.Err(); err != nil {
// 		return nil, err
// 	}

// 	return assignments, nil
// }

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

	// Construct the update document carefully
	// Only include fields that should actually be updatable on an Assignment itself
	updateFields := bson.M{
		"status":       assignment.Status,
		"clientNotes":  assignment.ClientNotes,
		"feedback":     assignment.Feedback,
		"updatedAt":    time.Now().UTC(), // Always update the timestamp
        "sets":         assignment.Sets, // Assuming these can be updated
        "reps":         assignment.Reps,
        "rest":         assignment.Rest,
        "tempo":        assignment.Tempo,
        "weight":       assignment.Weight,
        "duration":     assignment.Duration,
        "sequence":     assignment.Sequence,
        "trainerNotes": assignment.TrainerNotes,
	}

    // Handle optional UploadID correctly
	if assignment.UploadID != nil {
        // If UploadID is being explicitly set (even to NilObjectID to unset?)
        if *assignment.UploadID == primitive.NilObjectID {
             // If we need to unset the field in MongoDB
            // updateFields["$unset"] = bson.M{"uploadId": ""} // Requires restructure of update
        } else {
		    updateFields["uploadId"] = *assignment.UploadID
        }
	} // If assignment.UploadID is nil in the input struct, we simply don't include it in $set


    // REMOVED Handling for DueDate
	// if assignment.DueDate != nil { ... }


	update := bson.M{"$set": updateFields}
    // If you implemented $unset logic for uploadId, merge the $unset map into the update doc

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return repository.ErrNotFound
	}
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

// GetByWorkoutID retrieves all assignments for a specific workout.
func (r *mongoAssignmentRepository) GetByWorkoutID(ctx context.Context, workoutID primitive.ObjectID) ([]domain.Assignment, error) {
	var assignments []domain.Assignment
	filter := bson.M{"workoutId": workoutID}
	// Sort by sequence number of the exercise within the workout
	findOptions := options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
			return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &assignments); err != nil {
			return nil, err
	}
	if err = cursor.Err(); err != nil {
			return nil, err
	}
	return assignments, nil
}