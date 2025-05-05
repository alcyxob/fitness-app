package storage

import (
	"context"
	"time"
)

// Default expiry duration for presigned URLs
const DefaultPresignedURLExpiry = 15 * time.Minute

// FileStorage defines the interface for object storage operations.
type FileStorage interface {
	// GeneratePresignedUploadURL creates a temporary URL that allows PUT requests
	// for uploading an object directly to the storage provider.
	GeneratePresignedUploadURL(ctx context.Context, objectKey string, contentType string, expires time.Duration) (string, error)

	// GeneratePresignedDownloadURL creates a temporary URL that allows GET requests
	// for downloading/viewing an object directly from the storage provider.
	GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error)

	// DeleteObject removes an object from the storage provider.
	DeleteObject(ctx context.Context, objectKey string) error

	// GetObjectMetadata (Optional) - could be useful to check existence or get details
	// GetObjectMetadata(ctx context.Context, objectKey string) (*ObjectMetadata, error)
}

// ObjectMetadata (Optional) - structure for metadata if needed
// type ObjectMetadata struct {
// 	Size         int64
// 	ContentType  string
// 	LastModified time.Time
// 	ETag         string
// }

// Error constants for storage layer (optional)
var (
// ErrObjectNotFound = errors.New("object not found in storage")
)
