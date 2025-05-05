package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AssignmentStatus type for assignment lifecycle
type AssignmentStatus string

const (
	StatusAssigned  AssignmentStatus = "assigned"
	StatusSubmitted AssignmentStatus = "submitted" // Client uploaded video
	StatusReviewed  AssignmentStatus = "reviewed"  // Trainer provided feedback
	StatusCompleted AssignmentStatus = "completed" // Alternative to reviewed, or final state
)

// Assignment connects an Exercise to a Client, as assigned by a Trainer.
type Assignment struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ExerciseID  primitive.ObjectID  `bson:"exerciseId" json:"exerciseId"` // Link to the specific Exercise
	ClientID    primitive.ObjectID  `bson:"clientId" json:"clientId"`     // Link to the Client
	TrainerID   primitive.ObjectID  `bson:"trainerId" json:"trainerId"`   // Link to the Trainer (denormalized for easier queries/auth)
	AssignedAt  time.Time           `bson:"assignedAt" json:"assignedAt"`
	DueDate     *time.Time          `bson:"dueDate,omitempty" json:"dueDate,omitempty"`         // Optional due date (pointer for nullability)
	Status      AssignmentStatus    `bson:"status" json:"status"`                               // Tracks the state of the assignment
	ClientNotes string              `bson:"clientNotes,omitempty" json:"clientNotes,omitempty"` // Notes from client when submitting
	UploadID    *primitive.ObjectID `bson:"uploadId,omitempty" json:"uploadId,omitempty"`       // Link to the client's video Upload (pointer for nullability)
	Feedback    string              `bson:"feedback,omitempty" json:"feedback,omitempty"`       // Feedback from the trainer
	UpdatedAt   time.Time           `bson:"updatedAt" json:"updatedAt"`
}
