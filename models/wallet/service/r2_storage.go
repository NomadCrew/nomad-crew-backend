package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2FileStorage stores files in Cloudflare R2 (S3-compatible).
type R2FileStorage struct {
	client     *s3.Client
	presigner  *s3.PresignClient
	bucketName string
}

// NewR2FileStorage creates a new R2-backed file storage instance.
func NewR2FileStorage(accountID, bucketName, accessKeyID, secretAccessKey string) (*R2FileStorage, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: &endpoint,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
	})

	return &R2FileStorage{
		client:     client,
		presigner:  s3.NewPresignClient(client),
		bucketName: bucketName,
	}, nil
}

// validateKey rejects storage keys containing path traversal segments.
func validateKey(key string) error {
	for _, segment := range strings.Split(key, "/") {
		if segment == ".." {
			return fmt.Errorf("path traversal detected in storage key")
		}
	}
	return nil
}

// Save uploads a file to R2.
func (s *R2FileStorage) Save(ctx context.Context, path string, reader io.Reader, size int64) error {
	if err := validateKey(path); err != nil {
		return err
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucketName,
		Key:    &path,
		Body:   reader,
	})
	if err != nil {
		return fmt.Errorf("r2 put object failed: %w", err)
	}
	return nil
}

// Delete removes a file from R2.
func (s *R2FileStorage) Delete(ctx context.Context, path string) error {
	if err := validateKey(path); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucketName,
		Key:    &path,
	})
	if err != nil {
		return fmt.Errorf("r2 delete object failed: %w", err)
	}
	return nil
}

// GetURL returns a presigned download URL with a 5-minute TTL.
// The Content-Disposition header includes the original filename extracted from the key.
func (s *R2FileStorage) GetURL(ctx context.Context, path string) (string, error) {
	if err := validateKey(path); err != nil {
		return "", err
	}
	// Extract filename from key (format: <walletType>/<userID>/<timestamp>_<filename>)
	baseName := filepath.Base(path)
	// Strip the timestamp prefix (digits followed by underscore)
	if idx := strings.Index(baseName, "_"); idx >= 0 {
		baseName = baseName[idx+1:]
	}
	disposition := fmt.Sprintf("attachment; filename=\"%s\"", baseName)
	result, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     &s.bucketName,
		Key:                        &path,
		ResponseContentDisposition: &disposition,
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return "", fmt.Errorf("r2 presign failed: %w", err)
	}
	return result.URL, nil
}
