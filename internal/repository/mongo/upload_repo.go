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

const uploadCollectionName = "uploads"

// mongoUploadRepository implements repository.UploadRepository
type mongoUploadRepository struct {
	collection *mongo.Collection
}

// NewMongoUploadRepository creates a new Upload repository backed by MongoDB.
func NewMongoUploadRepository(db *mongo.Database) repository.UploadRepository {
	// EnsureUploadIndexes(context.Background(), db.Collection(uploadCollectionName)) // Call during startup
	return &mongoUploadRepository{
		collection: db.Collection(uploadCollectionName),
	}
}

// Create inserts new upload metadata into the database.
func (r *mongoUploadRepository) Create(ctx context.Context, upload *domain.Upload) (primitive.ObjectID, error) {
	// Basic validation
	if upload.AssignmentID == primitive.NilObjectID ||
		upload.ClientID == primitive.NilObjectID ||
		upload.TrainerID == primitive.NilObjectID || // Added TrainerID here for consistency if needed for queries
		upload.S3ObjectKey == "" {
		return primitive.NilObjectID, errors.New("upload requires assignmentId, clientId, trainerId, and s3ObjectKey")
	}

	upload.ID = primitive.NewObjectID()
	upload.UploadedAt = time.Now().UTC() // Set the upload timestamp

	result, err := r.collection.InsertOne(ctx, upload)
	if err != nil {
		// Could potentially check for duplicate S3ObjectKey if it must be unique, though unlikely collision
		return primitive.NilObjectID, err
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted ID")
	}

	return insertedID, nil
}

// GetByID retrieves upload metadata by its ID.
func (r *mongoUploadRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.Upload, error) {
	var upload domain.Upload
	filter := bson.M{"_id": id}

	err := r.collection.FindOne(ctx, filter).Decode(&upload)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &upload, nil
}

// GetByAssignmentID retrieves upload metadata linked to a specific assignment.
// Assumes one upload per assignment based on the interface definition.
// If multiple uploads per assignment are allowed, this should return []domain.Upload.
func (r *mongoUploadRepository) GetByAssignmentID(ctx context.Context, assignmentID primitive.ObjectID) (*domain.Upload, error) {
	var upload domain.Upload
	filter := bson.M{"assignmentId": assignmentID}

	// Optional: Sort if multiple could exist and you want the latest/earliest
	// findOneOptions := options.FindOne().SetSort(bson.D{{"uploadedAt", -1}})

	err := r.collection.FindOne(ctx, filter /*, findOneOptions*/).Decode(&upload)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// It's valid for an assignment to not have an upload yet,
			// so ErrNotFound might be the expected outcome in some flows.
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &upload, nil
}

/*
// Delete - Optional: Implement if you need to delete metadata, e.g., after deleting from S3.
func (r *mongoUploadRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return repository.ErrNotFound // Or ErrDeleteFailed
	}
	return nil
}
*/

// EnsureUploadIndexes creates necessary indexes for the uploads collection.
func EnsureUploadIndexes(ctx context.Context, collection *mongo.Collection) {
	indexes := []mongo.IndexModel{
		{
			// Index for finding the upload linked to an assignment (likely unique link)
			Keys:    bson.D{{Key: "assignmentId", Value: 1}},
			Options: options.Index().SetUnique(true), // Assuming one upload per assignment
		},
		{
			// Index for finding uploads by client
			Keys:    bson.D{{Key: "clientId", Value: 1}},
			Options: options.Index(),
		},
		{
			// Index for finding uploads by trainer (if needed for trainer views)
			Keys:    bson.D{{Key: "trainerId", Value: 1}},
			Options: options.Index(),
		},
		// Optional: Index on S3ObjectKey if you ever need to look up metadata by the S3 key
		// {
		//	Keys:    bson.D{{Key: "s3ObjectKey", Value: 1}},
		//	Options: options.Index().SetUnique(true), // S3 keys should be unique within the bucket
		// },
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}
