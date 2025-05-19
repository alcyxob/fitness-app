// internal/repository/mongo/workout_repo.go
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
	// "log" // If needed for index logging
)

const workoutCollectionName = "workouts"

// mongoWorkoutRepository implements repository.WorkoutRepository
type mongoWorkoutRepository struct {
	collection *mongo.Collection
}

// NewMongoWorkoutRepository creates a new Workout repository.
func NewMongoWorkoutRepository(db *mongo.Database) repository.WorkoutRepository {
	// EnsureWorkoutIndexes(context.Background(), db.Collection(workoutCollectionName))
	return &mongoWorkoutRepository{
		collection: db.Collection(workoutCollectionName),
	}
}

// Create inserts a new workout.
func (r *mongoWorkoutRepository) Create(ctx context.Context, workout *domain.Workout) (primitive.ObjectID, error) {
	if workout.TrainingPlanID == primitive.NilObjectID || workout.TrainerID == primitive.NilObjectID || workout.ClientID == primitive.NilObjectID || workout.Name == "" {
		return primitive.NilObjectID, errors.New("workout requires trainingPlanId, trainerId, clientId, and name")
	}
	workout.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	workout.CreatedAt = now
	workout.UpdatedAt = now
	// Default sequence if not set? Or enforce it? For now, assume it's set by service.

	result, err := r.collection.InsertOne(ctx, workout)
	if err != nil {
		return primitive.NilObjectID, err
	}
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted workout ID")
	}
	return insertedID, nil
}

// GetByID retrieves a single workout by its ID.
func (r *mongoWorkoutRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Workout, error) {
	var workout domain.Workout
	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&workout)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &workout, nil
}

// GetByPlanID retrieves all workouts associated with a specific training plan.
func (r *mongoWorkoutRepository) GetByPlanID(ctx context.Context, planID primitive.ObjectID) ([]domain.Workout, error) {
	var workouts []domain.Workout
	filter := bson.M{"trainingPlanId": planID}
	// Sort by sequence number
	findOptions := options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}, {Key: "dayOfWeek", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &workouts); err != nil {
		return nil, err
	}
    if err = cursor.Err(); err != nil {
        return nil, err
    }
	// Return empty slice if no workouts found
	return workouts, nil
}

// EnsureWorkoutIndexes creates necessary indexes. Call during startup.
func EnsureWorkoutIndexes(ctx context.Context, collection *mongo.Collection) {
	indexes := []mongo.IndexModel{
		{
			// Index for finding workouts within a specific plan, potentially sorted
			Keys:    bson.D{{Key: "trainingPlanId", Value: 1}, {Key: "sequence", Value: 1}},
			Options: options.Index(),
		},
		{
            // Index by trainer might be useful for overview screens?
			Keys:    bson.D{{Key: "trainerId", Value: 1}},
			Options: options.Index(),
		},
        {
            // Index by client might be useful for client-side fetching?
			Keys:    bson.D{{Key: "clientId", Value: 1}},
			Options: options.Index(),
		},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}

func (r *mongoWorkoutRepository) Update(ctx context.Context, workout *domain.Workout) error {
	if workout.ID == primitive.NilObjectID {
			return errors.New("workout ID is required for update")
	}

	// TrainerID, ClientID, TrainingPlanID should generally not be changed via this simple update.
	// If they need to change, it's a more complex "move" operation.
	filter := bson.M{"_id": workout.ID}
	updateDoc := bson.M{
			"$set": bson.M{
					"name":      workout.Name,
					"dayOfWeek": workout.DayOfWeek,
					"notes":     workout.Notes,
					"sequence":  workout.Sequence,
					"updatedAt": time.Now().UTC(),
			},
	}

	result, err := r.collection.UpdateOne(ctx, filter, updateDoc)
	if err != nil {
			return err
	}
	if result.MatchedCount == 0 {
			return repository.ErrNotFound // Workout with that ID didn't exist
	}
	return nil
}

func (r *mongoWorkoutRepository) Delete(ctx context.Context, workoutID primitive.ObjectID, trainerID primitive.ObjectID) error {
	if workoutID == primitive.NilObjectID || trainerID == primitive.NilObjectID {
			return errors.New("workout ID and trainer ID are required for deletion")
	}

	// Filter ensures the workout exists AND belongs to the specified trainer.
	filter := bson.M{
			"_id":       workoutID,
			"trainerId": trainerID,
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
			return err
	}
	if result.DeletedCount == 0 {
			// Workout not found OR not owned by this trainer.
			return repository.ErrNotFound
	}
	return nil
}
