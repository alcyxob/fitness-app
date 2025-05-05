package storage

import (
	"context"
	"alcyxob/fitness-app/internal/config" // Import your config package
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsCfg "github.com/aws/aws-sdk-go-v2/config" // Alias config to avoid clash
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3Storage implements the FileStorage interface using an S3-compatible backend.
type s3Storage struct {
	client        *s3.Client        // Regular client for operations like DeleteObject
	presignClient *s3.PresignClient // Special client for generating presigned URLs
	bucketName    string
}

// NewS3Storage creates a new S3 storage service instance.
func NewS3Storage(cfg config.S3Config) (FileStorage, error) {
	// Custom resolver for S3-compatible endpoints (like MinIO, DigitalOcean Spaces)
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		// Return the custom endpoint URL from config
		if cfg.Endpoint != "" {
			return aws.Endpoint{
				PartitionID:   "aws", // Usually "aws" even for compatible storage
				URL:           cfg.Endpoint,
				SigningRegion: cfg.Region,
				// Source: aws.EndpointSourceCustom, // Optional: indicate source
			}, nil
		}
		// Fallback to default AWS endpoint resolution if no custom endpoint is set
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	// Load AWS configuration
	awsSDKConfig, err := awsCfg.LoadDefaultConfig(context.TODO(), // Use context.TODO() for init is generally acceptable
		awsCfg.WithRegion(cfg.Region),
		awsCfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awsCfg.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		log.Printf("ERROR: Failed to load AWS SDK config for S3: %v", err)
		return nil, err
	}

	// Create the S3 client
	// Force path-style addressing required by most S3-compatible services (like MinIO)
	s3Client := s3.NewFromConfig(awsSDKConfig, func(o *s3.Options) {
		o.UsePathStyle = true // IMPORTANT for S3-compatible like MinIO!
		// o.ClientLogMode = aws.LogRetries | aws.LogRequest // Enable logging for debugging if needed
	})

	// Create the S3 Presign Client from the regular client
	presignClient := s3.NewPresignClient(s3Client)

	log.Printf("S3 Storage Service initialized for endpoint: %s, bucket: %s", cfg.Endpoint, cfg.BucketName)

	return &s3Storage{
		client:        s3Client,
		presignClient: presignClient,
		bucketName:    cfg.BucketName,
	}, nil
}

// GeneratePresignedUploadURL creates a temporary URL for uploading (PUT).
func (s *s3Storage) GeneratePresignedUploadURL(ctx context.Context, objectKey string, contentType string, expires time.Duration) (string, error) {
	if expires <= 0 {
		expires = DefaultPresignedURLExpiry
	}

	presignParams := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType), // Important: Client MUST set this header on upload
		// ACL: types.ObjectCannedACLPublicRead, // Optional: Set ACL if needed (e.g., for public read access) - Careful!
	}

	req, err := s.presignClient.PresignPutObject(ctx, presignParams, s3.WithPresignExpires(expires))
	if err != nil {
		log.Printf("ERROR: Failed to generate presigned PUT URL for key '%s': %v", objectKey, err)
		return "", err
	}

	return req.URL, nil
}

// GeneratePresignedDownloadURL creates a temporary URL for downloading (GET).
func (s *s3Storage) GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	if expires <= 0 {
		expires = DefaultPresignedURLExpiry
	}

	presignParams := &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	}

	req, err := s.presignClient.PresignGetObject(ctx, presignParams, s3.WithPresignExpires(expires))
	if err != nil {
		log.Printf("ERROR: Failed to generate presigned GET URL for key '%s': %v", objectKey, err)
		return "", err
	}

	return req.URL, nil
}

// DeleteObject removes an object from the S3 bucket.
func (s *s3Storage) DeleteObject(ctx context.Context, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		// You might want to check the specific error type (e.g., NoSuchKey)
		// but often just returning the error is sufficient.
		log.Printf("ERROR: Failed to delete object '%s' from bucket '%s': %v", objectKey, s.bucketName, err)
		return err
	}

	log.Printf("INFO: Deleted object '%s' from bucket '%s'", objectKey, s.bucketName)
	return nil
}
