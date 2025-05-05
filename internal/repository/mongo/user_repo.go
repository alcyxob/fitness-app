package mongo

import (
	"context"
	"errors" // Import the standard errors package
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/repository" // Import the repository interfaces package
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const userCollectionName = "users"

// mongoUserRepository implements the repository.UserRepository interface using MongoDB.
type mongoUserRepository struct {
	collection *mongo.Collection
}

// NewMongoUserRepository creates a new instance of mongoUserRepository.
// It expects a connected *mongo.Database instance.
func NewMongoUserRepository(db *mongo.Database) repository.UserRepository {
	// Ensure indexes are created (optional: can be done separately)
	// EnsureUserIndexes(context.Background(), db.Collection(userCollectionName)) // Example call

	return &mongoUserRepository{
		collection: db.Collection(userCollectionName),
	}
}

// Create inserts a new user into the database.
func (r *mongoUserRepository) Create(ctx context.Context, user *domain.User) (primitive.ObjectID, error) {
	// Ensure essential fields are set (basic validation, more robust validation belongs in service layer)
	if user.Email == "" || user.PasswordHash == "" || user.Role == "" {
		return primitive.NilObjectID, errors.New("user email, password hash, and role are required")
	}

	user.ID = primitive.NewObjectID() // Generate new ObjectID
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		// Check for duplicate key error (e.g., if email is unique index)
		if mongo.IsDuplicateKeyError(err) {
			// Consider returning a more specific custom error or repository.ErrConflict
			return primitive.NilObjectID, errors.New("user with this email already exists")
		}
		return primitive.NilObjectID, err // Return other insertion errors
	}

	// Assert the type of the inserted ID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("failed to convert inserted ID")
	}

	return insertedID, nil
}

// GetByEmail retrieves a user by their email address.
func (r *mongoUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	// Use bson.M for the filter document
	filter := bson.M{"email": email}

	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Return the custom repository error for not found
			return nil, repository.ErrNotFound
		}
		return nil, err // Return other errors
	}
	return &user, nil
}

// GetByID retrieves a user by their MongoDB ObjectID.
func (r *mongoUserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*domain.User, error) {
	var user domain.User
	filter := bson.M{"_id": id}

	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// AddClientIDToTrainer adds a client's ID to a trainer's ClientIDs array.
func (r *mongoUserRepository) AddClientIDToTrainer(ctx context.Context, trainerID, clientID primitive.ObjectID) error {
	filter := bson.M{"_id": trainerID, "role": domain.RoleTrainer}
	update := bson.M{
		"$addToSet": bson.M{"clientIds": clientID}, // $addToSet prevents duplicates
		"$set":      bson.M{"updatedAt": time.Now().UTC()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Check if a document was actually found and modified
	if result.MatchedCount == 0 {
		return repository.ErrNotFound // Or a more specific error like "trainer not found"
	}
	// result.ModifiedCount might be 0 if the clientID was already in the set, which is okay.

	return nil
}

// GetClientsByTrainerID retrieves all client users associated with a specific trainer.
func (r *mongoUserRepository) GetClientsByTrainerID(ctx context.Context, trainerID primitive.ObjectID) ([]domain.User, error) {
	// Find the trainer first to get the list of client IDs
	trainer, err := r.GetByID(ctx, trainerID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errors.New("trainer not found") // More specific error
		}
		return nil, err // Other errors from GetByID
	}

	if !trainer.IsTrainer() {
		return nil, errors.New("user is not a trainer") // Authorization check
	}

	if len(trainer.ClientIDs) == 0 {
		return []domain.User{}, nil // Return empty slice if trainer has no clients
	}

	// Now find all users whose IDs are in the trainer's ClientIDs list
	var clients []domain.User
	// Use $in operator to find documents where _id is in the provided array
	filter := bson.M{"_id": bson.M{"$in": trainer.ClientIDs}}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx) // Ensure cursor is closed

	// Decode all documents found by the cursor
	if err = cursor.All(ctx, &clients); err != nil {
		return nil, err
	}

	// Check for cursor errors after iteration
	if err = cursor.Err(); err != nil {
		return nil, err
	}

	return clients, nil
}

// SetTrainerForClient sets the TrainerID field for a specific client user.
func (r *mongoUserRepository) SetTrainerForClient(ctx context.Context, clientID, trainerID primitive.ObjectID) error {
	filter := bson.M{"_id": clientID, "role": domain.RoleClient}
	update := bson.M{
		"$set": bson.M{
			"trainerId": trainerID,
			"updatedAt": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return repository.ErrNotFound // Or "client not found"
	}
	// ModifiedCount would be 0 if the trainerID was already set to the same value.

	return nil
}

// EnsureUserIndexes creates necessary indexes for the users collection.
// Call this once during application startup.
func EnsureUserIndexes(ctx context.Context, collection *mongo.Collection) {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}}, // Create index on email
			Options: options.Index().SetUnique(true),  // Make email unique
		},
		{
			Keys:    bson.D{{Key: "role", Value: 1}}, // Index on role might be useful
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "trainerId", Value: 1}}, // Index for finding clients by trainer
			Options: options.Index().SetSparse(true),      // Sparse because not all users have trainerId
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// Log the error, but maybe don't crash the app?
		// Consider how critical index creation failure is.
		// log.Printf("WARN: Failed to create indexes for collection %s: %v", collection.Name(), err)
	}
}
