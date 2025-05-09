// internal/domain/exercise.go
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Exercise represents a single exercise definition in the library.
type Exercise struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TrainerID   primitive.ObjectID `bson:"trainerId" json:"trainerId"` // Link to the Trainer who created/owns this exercise
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"` // General description
	
	MuscleGroup    string `bson:"muscleGroup,omitempty" json:"muscleGroup,omitempty"`         // e.g., "Chest", "Legs", "Back"
	ExecutionTechnic string `bson:"executionTechnic,omitempty" json:"executionTechnic,omitempty"` // Detailed instructions
	Applicability  string `bson:"applicability,omitempty" json:"applicability,omitempty"`     // e.g., "Home", "Gym", "Home/Gym"
	Difficulty     string `bson:"difficulty,omitempty" json:"difficulty,omitempty"`         // e.g., "Novice", "Medium", "Advanced"
	VideoURL       string `bson:"videoUrl,omitempty" json:"videoUrl,omitempty"` // Optional URL to an example video (trainer might upload later to S3 and link here)
	// --- END NEW FIELDS ---

	// Instructions field might be redundant now if ExecutionTechnic covers it.
	// Let's remove 'Instructions' if 'ExecutionTechnic' is more comprehensive.
	// Instructions string            `bson:"instructions,omitempty" json:"instructions,omitempty"` 

	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}