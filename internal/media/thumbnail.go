package media

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
)

// maxThumbnailWidth is the maximum width for generated thumbnails.
const maxThumbnailWidth = 320

// GenerateThumbnail downloads a source image from S3, resizes it to max 320px wide
// (maintaining aspect ratio), encodes as JPEG, and uploads the thumbnail back to S3.
// Only processes images (JPEG, PNG, GIF, WebP). Videos are skipped (return nil).
// If thumbnail generation fails, the error is returned but callers should treat
// it as non-fatal — uploads should still proceed to READY status.
func GenerateThumbnail(ctx context.Context, s3Client *S3Client, sourceKey, thumbKey string) error {
	// Download source image from S3
	body, err := s3Client.GetObject(ctx, sourceKey)
	if err != nil {
		return fmt.Errorf("download source image: %w", err)
	}
	defer body.Close()

	// Decode the image using imaging (supports JPEG, PNG, GIF, BMP, TIFF)
	img, err := imaging.Decode(body)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	// Resize to max 320px wide maintaining aspect ratio
	// Height 0 means auto-calculate to preserve aspect ratio
	thumb := imaging.Resize(img, maxThumbnailWidth, 0, imaging.Lanczos)

	// Encode thumbnail as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}

	// Upload thumbnail to S3
	thumbBytes := buf.Bytes()
	_, err = s3Client.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s3Client.bucket),
		Key:           aws.String(thumbKey),
		Body:          bytes.NewReader(thumbBytes),
		ContentType:   aws.String("image/jpeg"),
		ContentLength: aws.Int64(int64(len(thumbBytes))),
	})
	if err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}

	return nil
}
