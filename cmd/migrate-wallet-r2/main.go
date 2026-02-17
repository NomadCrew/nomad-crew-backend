// Package main provides a one-time migration tool that copies wallet files
// from local disk storage to Cloudflare R2. It does NOT modify the database.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "List files that would be migrated without uploading")
	concurrency := flag.Int("concurrency", 4, "Number of parallel uploads")
	storagePath := flag.String("storage-path", "/var/data/wallet-files", "Local base path for wallet files")
	flag.Parse()

	ctx := context.Background()

	// --- Database connection ---
	dbURL := buildDatabaseURL()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("Connected to database")

	// --- Fetch file paths from DB ---
	rows, err := pool.Query(ctx,
		`SELECT file_path FROM wallet_documents WHERE deleted_at IS NULL`)
	if err != nil {
		log.Fatalf("Failed to query wallet_documents: %v", err)
	}
	defer rows.Close()

	var filePaths []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		filePaths = append(filePaths, fp)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("Row iteration error: %v", err)
	}

	total := len(filePaths)
	log.Printf("Found %d active wallet documents", total)

	if total == 0 {
		log.Println("Nothing to migrate")
		return
	}

	if *dryRun {
		log.Println("=== DRY RUN — files that would be migrated ===")
		for i, fp := range filePaths {
			localPath := filepath.Join(*storagePath, fp)
			exists := "EXISTS"
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				exists = "MISSING"
			}
			fmt.Printf("  [%d/%d] %s (%s)\n", i+1, total, fp, exists)
		}
		log.Println("=== DRY RUN complete ===")
		return
	}

	// --- R2 / S3 client ---
	s3Client, bucket := newR2Client(ctx)

	// --- Concurrent migration ---
	var (
		migrated int64
		skipped  int64
		errCount int64
		wg       sync.WaitGroup
		sem      = make(chan struct{}, *concurrency)
	)

	for i, fp := range filePaths {
		wg.Add(1)
		sem <- struct{}{} // acquire slot

		go func(idx int, key string) {
			defer wg.Done()
			defer func() { <-sem }() // release slot

			localPath := filepath.Join(*storagePath, key)

			// Check if already in R2
			if existsInR2(ctx, s3Client, bucket, key) {
				log.Printf("Skipping file %d/%d (already in R2): %s", idx+1, total, key)
				atomic.AddInt64(&skipped, 1)
				return
			}

			// Open local file
			f, err := os.Open(localPath)
			if err != nil {
				log.Printf("ERROR file %d/%d: cannot open %s: %v", idx+1, total, localPath, err)
				atomic.AddInt64(&errCount, 1)
				return
			}
			defer f.Close()

			log.Printf("Migrating file %d/%d: %s", idx+1, total, key)

			// Upload to R2
			if err := uploadToR2(ctx, s3Client, bucket, key, f); err != nil {
				log.Printf("ERROR file %d/%d: upload failed for %s: %v", idx+1, total, key, err)
				atomic.AddInt64(&errCount, 1)
				return
			}

			// Verify with HeadObject
			if !existsInR2(ctx, s3Client, bucket, key) {
				log.Printf("ERROR file %d/%d: verification failed for %s", idx+1, total, key)
				atomic.AddInt64(&errCount, 1)
				return
			}

			atomic.AddInt64(&migrated, 1)
		}(i, fp)
	}

	wg.Wait()

	log.Println("=== Migration Summary ===")
	log.Printf("  Total:    %d", total)
	log.Printf("  Migrated: %d", migrated)
	log.Printf("  Skipped:  %d (already in R2)", skipped)
	log.Printf("  Errors:   %d", errCount)

	if errCount > 0 {
		os.Exit(1)
	}
}

// buildDatabaseURL constructs a PostgreSQL connection string from env vars.
// Supports DATABASE_URL directly, or individual DB_* vars.
func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	pass := envOrDefault("DB_PASSWORD", "")
	name := envOrDefault("DB_NAME", "nomadcrew")
	ssl := envOrDefault("DB_SSL_MODE", "disable")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(user), url.QueryEscape(pass), host, port, name, ssl)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newR2Client creates an S3-compatible client pointed at Cloudflare R2.
func newR2Client(ctx context.Context) (*s3.Client, string) {
	accountID := requireEnv("R2_ACCOUNT_ID")
	bucket := requireEnv("R2_BUCKET_NAME")
	accessKey := requireEnv("R2_ACCESS_KEY_ID")
	secretKey := requireEnv("R2_SECRET_ACCESS_KEY")

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return client, bucket
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return v
}

// existsInR2 checks if an object already exists in R2 via HeadObject.
func existsInR2(ctx context.Context, client *s3.Client, bucket, key string) bool {
	_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

// uploadToR2 uploads a file to R2 using PutObject.
func uploadToR2(ctx context.Context, client *s3.Client, bucket, key string, body io.ReadSeeker) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		Body:              body,
		ChecksumAlgorithm: s3types.ChecksumAlgorithmCrc32,
	})
	return err
}
