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

const exerciseCollectionName = "exercises"

// mongoExerciseRepository implements repository.ExerciseRepository
type mongoExerciseRepository struct {
	collection *mongo.Collection
}

// NewMongoExerciseRepository creates a new Exercise repository backed by MongoDB.
func NewMongoExerciseRepository(db *mongo.Database) repository.ExerciseRepository {
	// EnsureExerciseIndexes(context.Background(), db.Collection(exerciseCollectionName)) // Call during startup
	return &mongoExerciseRepository{
		collection: db.Collection(exerciseCollectionName),
	}
}

// Create inserts a new exercise into the database.
func (r *mongoExerciseRepository) Create(ctx context.Context, exercise *domain.Exercise) (primitive.ObjectID, error) {
	if exercise.Name == "" || exercise.TrainerID == primitive.NilObjectID {
		return primitive.NilObjectID, errors.New("exercise name and trainer ID are required")
	}

	exercise.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	exercise.CreatedAt = now
	exercise.UpdatedAt = now

	result, err := r.collection.InsertOne(ctx, exercise)
	if err != nil {
		return primitive.NilObjectID, err
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted ID")
	}

	return insertedID, nil
}

// GetByID retrieves an exercise by its ID.
func (r *mongoExerciseRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Exercise, error) {
	var exercise domain.Exercise
	filter := bson.M{"_id": id}

	err := r.collection.FindOne(ctx, filter).Decode(&exercise)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &exercise, nil
}

// GetByTrainerID retrieves all exercises created by a specific trainer.
func (r *mongoExerciseRepository) GetByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Exercise, error) {
	var exercises []domain.Exercise
	filter := bson.M{"trainerId": trainerID}

	// Optional: Add sorting, e.g., by name or creation date
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}) // Sort by newest first

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &exercises); err != nil {
		return nil, err
	}

	// Check for cursor errors after iteration
	if err = cursor.Err(); err != nil {
		return nil, err
	}

	return exercises, nil
}

// Update modifies an existing exercise.
// It ensures the TrainerID is not changed and updates the UpdatedAt timestamp.
func (r *mongoExerciseRepository) Update(ctx context.Context, exercise *domain.Exercise) error {
	if exercise.ID == primitive.NilObjectID {
		return errors.New("exercise ID is required for update")
	}
	if exercise.Name == "" { // Add other necessary validations
		return errors.New("exercise name cannot be empty")
	}

	filter := bson.M{"_id": exercise.ID}
	// Prevent changing the owner (TrainerID) during a simple update
	// More complex ownership transfer would need a separate method/service logic
	update := bson.M{
		"$set": bson.M{
			"name":         exercise.Name,
			"description":  exercise.Description,
			"instructions": exercise.Instructions,
			"videoUrl":     exercise.VideoURL,
			"updatedAt":    time.Now().UTC(),
			// Note: We specifically DO NOT set trainerId here
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return repository.ErrNotFound // Exercise with that ID didn't exist
	}
	if result.ModifiedCount == 0 && result.MatchedCount == 1 {
		// Matched but didn't modify - data might be the same. Not necessarily an error.
		// You could choose to return nil or a specific indicator if needed.
	}

	return nil
}

// Delete removes an exercise, ensuring it belongs to the specified trainer.
func (r *mongoExerciseRepository) Delete(ctx context.Context, id primitive.ObjectID, trainerID primitive.ObjectID) error {
	// The filter ensures we only delete if the _id matches AND the trainerId matches.
	// This prevents a trainer from deleting another trainer's exercise.
	filter := bson.M{
		"_id":       id,
		"trainerId": trainerID,
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		// Could be because the exercise didn't exist OR it belonged to another trainer.
		// We might check existence first to give a more specific error,
		// but for now, ErrNotFound covers non-existence, and implicitly handles
		// the "not owned by this trainer" case as it won't match the filter.
		// A service layer check might be better for distinguishing these cases.
		// We could also return ErrDeleteFailed here instead of ErrNotFound.
		return repository.ErrNotFound
		// return repository.ErrDeleteFailed // Alternative error
	}

	return nil
}

// EnsureExerciseIndexes creates necessary indexes for the exercises collection.
func EnsureExerciseIndexes(ctx context.Context, collection *mongo.Collection) {
	indexes := []mongo.IndexModel{
		{
			// Index for finding exercises by the trainer who created them
			Keys:    bson.D{{Key: "trainerId", Value: 1}},
			Options: options.Index(),
		},
		{
			// Optional: Index name for faster text search if needed later
			Keys:    bson.D{{Key: "name", Value: "text"}, {Key: "description", Value: "text"}},
			Options: options.Index().SetName("exercise_text_search"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}
