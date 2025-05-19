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

func (r *mongoTrainingPlanRepository) Update(ctx context.Context, plan *domain.TrainingPlan) error {
    if plan.ID == primitive.NilObjectID {
        return errors.New("training plan ID is required for update")
    }

    filter := bson.M{"_id": plan.ID}
    // Construct update document carefully, only setting fields that should be updatable.
    // TrainerID and ClientID should generally not be changed by a simple plan update.
    // CreatedAt should not be changed.
    updateDoc := bson.M{
        "$set": bson.M{
            "name":        plan.Name,
            "description": plan.Description,
            "startDate":   plan.StartDate, // Pass pointer directly
            "endDate":     plan.EndDate,   // Pass pointer directly
            "isActive":    plan.IsActive,
            "updatedAt":   time.Now().UTC(), // Always update this
        },
    }

    result, err := r.collection.UpdateOne(ctx, filter, updateDoc)
    if err != nil {
        return err
    }
    if result.MatchedCount == 0 {
        return repository.ErrNotFound // Plan with that ID didn't exist
    }
    // result.ModifiedCount could be 0 if data was the same, which is not an error.
    return nil
}

// Implement DeactivateOtherPlansForClient if strict single active plan is needed
func (r *mongoTrainingPlanRepository) DeactivateOtherPlansForClient(ctx context.Context, clientID, trainerID, excludePlanID primitive.ObjectID) error {
    filter := bson.M{
        "clientId":  clientID,
        "trainerId": trainerID,
        "isActive":  true,
        "_id":       bson.M{"$ne": excludePlanID}, // Don't deactivate the plan we're trying to activate
    }
    update := bson.M{"$set": bson.M{"isActive": false, "updatedAt": time.Now().UTC()}}
    _, err := r.collection.UpdateMany(ctx, filter, update)
    return err
}

func (r *mongoTrainingPlanRepository) Delete(ctx context.Context, planID primitive.ObjectID, trainerID primitive.ObjectID) error {
    if planID == primitive.NilObjectID || trainerID == primitive.NilObjectID {
        return errors.New("plan ID and trainer ID are required for deletion")
    }

    // Filter ensures that the plan exists AND belongs to the specified trainer.
    filter := bson.M{
        "_id":       planID,
        "trainerId": trainerID,
    }

    result, err := r.collection.DeleteOne(ctx, filter)
    if err != nil {
        return err // Database error
    }

    if result.DeletedCount == 0 {
        // This means either the plan didn't exist, or it didn't belong to this trainer.
        // The service layer might have already checked existence via GetByID,
        // so this often implies an ownership mismatch if GetByID succeeded.
        return repository.ErrNotFound // Or a more specific "delete failed / not authorized"
    }
    return nil
}