// internal/repository/mongo/training_plan_repo.go
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

const trainingPlanCollectionName = "training_plans"

// mongoTrainingPlanRepository implements repository.TrainingPlanRepository
type mongoTrainingPlanRepository struct {
	collection *mongo.Collection
}

// NewMongoTrainingPlanRepository creates a new TrainingPlan repository.
func NewMongoTrainingPlanRepository(db *mongo.Database) repository.TrainingPlanRepository {
	// EnsureTrainingPlanIndexes(context.Background(), db.Collection(trainingPlanCollectionName))
	return &mongoTrainingPlanRepository{
		collection: db.Collection(trainingPlanCollectionName),
	}
}

// Create inserts a new training plan.
func (r *mongoTrainingPlanRepository) Create(ctx context.Context, plan *domain.TrainingPlan) (primitive.ObjectID, error) {
	if plan.ClientID == primitive.NilObjectID || plan.TrainerID == primitive.NilObjectID || plan.Name == "" {
		return primitive.NilObjectID, errors.New("plan requires clientId, trainerId, and name")
	}
	plan.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	result, err := r.collection.InsertOne(ctx, plan)
	if err != nil {
		return primitive.NilObjectID, err
	}
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted plan ID")
	}
	return insertedID, nil
}

// GetByID retrieves a single training plan by its ID.
func (r *mongoTrainingPlanRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.TrainingPlan, error) {
	var plan domain.TrainingPlan
	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&plan)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// GetByClientAndTrainerID retrieves all plans for a specific client created by a specific trainer.
func (r *mongoTrainingPlanRepository) GetByClientAndTrainerID(ctx context.Context, clientID, trainerID primitive.ObjectID) ([]domain.TrainingPlan, error) {
	var plans []domain.TrainingPlan
	// Filter ensures trainer ownership and correct client association
	filter := bson.M{
		"clientId":  clientID,
		"trainerId": trainerID,
	}
	// Sort by creation date, newest first
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &plans); err != nil {
		return nil, err
	}
    if err = cursor.Err(); err != nil {
        return nil, err
    }
	// Return empty slice if no plans found (not an error)
	return plans, nil
}

// EnsureTrainingPlanIndexes creates necessary indexes. Call during startup.
func EnsureTrainingPlanIndexes(ctx context.Context, collection *mongo.Collection) {
	indexes := []mongo.IndexModel{
		{
			// Compound index for the main query pattern: finding plans for a client by a trainer
			Keys:    bson.D{{Key: "trainerId", Value: 1}, {Key: "clientId", Value: 1}},
			Options: options.Index(),
		},
		{
			// Index to quickly find active plans for a client (if needed)
			Keys:    bson.D{{Key: "clientId", Value: 1}, {Key: "isActive", Value: 1}},
			Options: options.Index().SetSparse(true), // Sparse if not all docs might have isActive? Or if only true matters?
		},
		{
			Keys:    bson.D{{Key: "trainerId", Value: 1}}, // Simple index on trainerId
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "clientId", Value: 1}}, // Simple index on clientId
			Options: options.Index(),
		},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}