package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Upload stores metadata about a file uploaded by a client,
// typically linked to an Assignment. The actual file resides in S3.
type Upload struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AssignmentID primitive.ObjectID `bson:"assignmentId" json:"assignmentId"` // Link back to the assignment
	ClientID     primitive.ObjectID `bson:"clientId" json:"clientId"`         // Link to the client who uploaded
	TrainerID    primitive.ObjectID `bson:"trainerId" json:"trainerId"`       // Link to trainer (denormalized)
	S3ObjectKey  string             `bson:"s3ObjectKey" json:"-"`             // The unique key (path/filename) in the S3 bucket - internal use
	FileName     string             `bson:"fileName" json:"fileName"`         // Original filename provided by client
	ContentType  string             `bson:"contentType" json:"contentType"`   // MIME type (e.g., "video/mp4")
	Size         int64              `bson:"size" json:"size"`                 // File size in bytes
	UploadedAt   time.Time          `bson:"uploadedAt" json:"uploadedAt"`
	// PresignedURL string          `bson:"-" json:"presignedUrl,omitempty"` // Optionally generate and add this for downloads (not stored in DB)
}
