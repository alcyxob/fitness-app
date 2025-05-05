package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Role type to distinguish between user roles
type Role string

// Define constants for roles
const (
	RoleTrainer Role = "trainer"
	RoleClient  Role = "client"
)

// User represents a user in the system (either a Trainer or a Client).
type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Email        string             `bson:"email" json:"email"`    // Should be unique
	PasswordHash string             `bson:"passwordHash" json:"-"` // Never expose this via JSON
	Role         Role               `bson:"role" json:"role"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`

	// --- Trainer-specific ---
	// Stores ObjectIDs of Clients managed by this Trainer.
	// Use pointers or omitempty if a trainer might initially have no clients.
	ClientIDs []primitive.ObjectID `bson:"clientIds,omitempty" json:"clientIds,omitempty"`

	// --- Client-specific ---
	// Stores the ObjectID of the Trainer managing this Client.
	// Use pointer or omitempty as a client might not be assigned immediately.
	TrainerID *primitive.ObjectID `bson:"trainerId,omitempty" json:"trainerId,omitempty"`
}

// Helper methods (Optional but can be useful)
func (u *User) IsTrainer() bool {
	return u.Role == RoleTrainer
}

func (u *User) IsClient() bool {
	return u.Role == RoleClient
}
