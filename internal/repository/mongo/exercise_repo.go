// internal/repository/mongo/exercise_repo.go
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
)

const exerciseCollectionName = "exercises" // Ensure this constant is defined if not already

type mongoExerciseRepository struct {
	collection *mongo.Collection
}

func NewMongoExerciseRepository(db *mongo.Database) repository.ExerciseRepository {
	return &mongoExerciseRepository{
		collection: db.Collection(exerciseCollectionName),
	}
}

func (r *mongoExerciseRepository) Create(ctx context.Context, exercise *domain.Exercise) (primitive.ObjectID, error) {
	// ... (your existing Create method)
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

func (r *mongoExerciseRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Exercise, error) {
	// ... (your existing GetByID method)
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

func (r *mongoExerciseRepository) GetByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.Exercise, error) {
	// ... (your existing GetByTrainerID method)
	var exercises []domain.Exercise
	filter := bson.M{"trainerId": trainerID}
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &exercises); err != nil {
		return nil, err
	}
    if err = cursor.Err(); err != nil {
        return nil, err
    }
	return exercises, nil
}

// --- THIS IS THE METHOD TO FIX ---
func (r *mongoExerciseRepository) Update(ctx context.Context, exercise *domain.Exercise) error {
	if exercise.ID == primitive.NilObjectID {
		return errors.New("exercise ID is required for update")
	}
	// Add other necessary validations if needed, e.g., non-empty name
	// if exercise.Name == "" {
	// 	return errors.New("exercise name cannot be empty for update")
	// }

	filter := bson.M{"_id": exercise.ID}
	// Prevent changing the owner (TrainerID) during a simple update
	update := bson.M{
		"$set": bson.M{
			"name":             exercise.Name,
			"description":      exercise.Description,
			"muscleGroup":      exercise.MuscleGroup,      // ADDED/VERIFIED
			"executionTechnic": exercise.ExecutionTechnic,  // ADDED/VERIFIED
			"applicability":    exercise.Applicability,     // ADDED/VERIFIED
			"difficulty":       exercise.Difficulty,        // ADDED/VERIFIED
			"videoUrl":         exercise.VideoURL,
			"updatedAt":        time.Now().UTC(),
			// REMOVED: "instructions": exercise.Instructions,
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
// --- END OF METHOD TO FIX ---

func (r *mongoExerciseRepository) Delete(ctx context.Context, id primitive.ObjectID, trainerID primitive.ObjectID) error {
	// ... (your existing Delete method)
	filter := bson.M{
		"_id":       id,
		"trainerId": trainerID,
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// EnsureExerciseIndexes function (if you have it)
func EnsureExerciseIndexes(ctx context.Context, collection *mongo.Collection) {
	// ... (your existing index definitions)
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "trainerId", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "name", Value: "text"}, {Key: "description", Value: "text"}},
			Options: options.Index().SetName("exercise_text_search"),
		},
		// Add new indexes if needed for muscleGroup, applicability, difficulty
		// { Keys: bson.D{{Key: "muscleGroup", Value: 1}}, Options: options.Index() },
		// { Keys: bson.D{{Key: "difficulty", Value: 1}}, Options: options.Index() },
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}