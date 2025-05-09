package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Workout represents a single workout session within a TrainingPlan.
type Workout struct { // Renamed struct
    ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    TrainingPlanID primitive.ObjectID `bson:"trainingPlanId" json:"trainingPlanId"` // Link back to the plan
    TrainerID      primitive.ObjectID `bson:"trainerId" json:"trainerId"`         // Denormalized for easier query/auth
    ClientID       primitive.ObjectID `bson:"clientId" json:"clientId"`           // Denormalized
    Name           string             `bson:"name" json:"name"`                   // e.g., "Day 1: Upper Body", "Long Run"
    DayOfWeek      *int               `bson:"dayOfWeek,omitempty" json:"dayOfWeek,omitempty"` // Optional: e.g., 1 (Mon) - 7 (Sun)
    Notes          string             `bson:"notes,omitempty" json:"notes,omitempty"`     // Notes for the client for this specific workout
    Sequence       int                `bson:"sequence"`                               // Order within the plan (if not using DayOfWeek)
    CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
    UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
    // Exercises will be linked via Assignments pointing to THIS Workout's ID
}