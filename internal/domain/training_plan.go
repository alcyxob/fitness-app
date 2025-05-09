// internal/domain/training_plan.go
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TrainingPlan represents a structured plan assigned to a client by a trainer.
type TrainingPlan struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TrainerID   primitive.ObjectID `bson:"trainerId" json:"trainerId"`       // Who created the plan
	ClientID    primitive.ObjectID `bson:"clientId" json:"clientId"`         // Who the plan is for
	Name        string             `bson:"name" json:"name"`                 // e.g., "Phase 1: Hypertrophy"
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	StartDate   *time.Time         `bson:"startDate,omitempty" json:"startDate,omitempty"` // Optional start date
	EndDate     *time.Time         `bson:"endDate,omitempty" json:"endDate,omitempty"`   // Optional end date
	IsActive    bool               `bson:"isActive" json:"isActive"`         // Is this the currently active plan for the client?
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}