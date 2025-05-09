// internal/domain/assignment.go (MODIFIED)
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AssignmentStatus string // Keep this

const ( // Keep these
	StatusAssigned  AssignmentStatus = "assigned"
	StatusSubmitted AssignmentStatus = "submitted"
	StatusReviewed  AssignmentStatus = "reviewed"
	StatusCompleted AssignmentStatus = "completed"
)

// Assignment now links an Exercise to a specific Workout session.
type Assignment struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WorkoutID  primitive.ObjectID `bson:"workoutId" json:"workoutId"`   // <<< CHANGED: Link to the Workout session
	ExerciseID primitive.ObjectID `bson:"exerciseId" json:"exerciseId"` // Link to the specific Exercise
    // ClientID/TrainerID are implicitly known via the WorkoutID, but could be denormalized if needed for perf

	// --- Exercise Execution Details ---
	Sets           *int    `bson:"sets,omitempty" json:"sets,omitempty"`
	Reps           *string `bson:"reps,omitempty" json:"reps,omitempty"`             // e.g., "8-12", "AMRAP"
	Rest           *string `bson:"rest,omitempty" json:"rest,omitempty"`             // e.g., "60s", "2m"
	Tempo          *string `bson:"tempo,omitempty" json:"tempo,omitempty"`           // e.g., "2010" (2 sec down, 0 pause, 1 sec up, 0 pause)
	Weight         *string `bson:"weight,omitempty" json:"weight,omitempty"`         // e.g., "10kg", "BW", "RPE 8"
	Duration       *string `bson:"duration,omitempty" json:"duration,omitempty"`     // e.g., "30min", "5km" (for cardio/timed)
	Sequence       int     `bson:"sequence"`                                         // Order of exercise within the workout
	TrainerNotes   string  `bson:"trainerNotes,omitempty" json:"trainerNotes,omitempty"` // Specific notes for this exercise assignment

    // --- Client Tracking Fields ---
	AssignedAt     time.Time          `bson:"assignedAt" json:"assignedAt"` // When this specific assignment was configured
	Status         AssignmentStatus   `bson:"status" json:"status"`
	ClientNotes    string             `bson:"clientNotes,omitempty" json:"clientNotes,omitempty"`
	UploadID       *primitive.ObjectID `bson:"uploadId,omitempty" json:"uploadId,omitempty"` // Link to video proof
	Feedback       string             `bson:"feedback,omitempty" json:"feedback,omitempty"` // Trainer feedback on submission
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
}