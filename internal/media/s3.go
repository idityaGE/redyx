package media

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client wraps the AWS S3 client for MinIO-compatible object storage.
type S3Client struct {
	client   *s3.Client
	presign  *s3.PresignClient
	bucket   string
	endpoint string
}

// NewS3Client creates an S3 client configured for MinIO with path-style addressing.
func NewS3Client(endpoint, accessKey, secretKey, region, bucket string) (*S3Client, error) {
	if region == "" {
		region = "us-east-1"
	}

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true, // CRITICAL for MinIO — virtual-hosted style won't work
	})

	presign := s3.NewPresignClient(client)

	return &S3Client{
		client:   client,
		presign:  presign,
		bucket:   bucket,
		endpoint: endpoint,
	}, nil
}

// GeneratePresignedPUT creates a presigned PUT URL for uploading an object.
// Expiry is 1 hour (long enough for large video uploads).
func (c *S3Client) GeneratePresignedPUT(ctx context.Context, key, contentType string, sizeBytes int64) (string, time.Time, error) {
	expiry := 1 * time.Hour

	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(sizeBytes),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("presign put object: %w", err)
	}

	expiresAt := time.Now().Add(expiry)
	return req.URL, expiresAt, nil
}

// ObjectExists checks if an object exists in the bucket using HeadObject.
func (c *S3Client) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// If the object doesn't exist, HeadObject returns a NotFound-style error
		return false, nil
	}
	return true, nil
}

// GetObjectURL constructs the public URL for an object (MinIO endpoint + bucket + key).
func (c *S3Client) GetObjectURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)
}

// GetObject downloads an object from the bucket and returns the body as an io.ReadCloser.
func (c *S3Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return out.Body, nil
}
