package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Exercise represents a single exercise definition in the library.
type Exercise struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TrainerID    primitive.ObjectID `bson:"trainerId" json:"trainerId"` // Link to the Trainer who created/owns this exercise
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description,omitempty" json:"description,omitempty"`
	Instructions string             `bson:"instructions,omitempty" json:"instructions,omitempty"` // More detailed instructions
	VideoURL     string             `bson:"videoUrl,omitempty" json:"videoUrl,omitempty"`         // Optional URL to an example video (e.g., YouTube link)
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
}
