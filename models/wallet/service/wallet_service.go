package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gabriel-vasile/mimetype"
)

// Allowed MIME types for document uploads
var allowedMimeTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
	"image/heic":      true,
	"image/heif":      true,
}

// MaxFileSize is the maximum allowed file size (10MB)
const MaxFileSize = 10 * 1024 * 1024

// FileStorage provides an abstraction over file storage backends
type FileStorage interface {
	Save(ctx context.Context, path string, reader io.Reader, size int64) error
	Delete(ctx context.Context, path string) error
	GetPath(ctx context.Context, path string) string
}

// LocalFileStorage stores files on the local filesystem
type LocalFileStorage struct {
	basePath string
}

// NewLocalFileStorage creates a new local file storage instance
func NewLocalFileStorage(basePath string) *LocalFileStorage {
	_ = os.MkdirAll(basePath, 0755)
	return &LocalFileStorage{basePath: basePath}
}

// containedPath resolves the full path and verifies it stays within basePath.
// Returns an error if the path escapes the storage directory.
func (s *LocalFileStorage) containedPath(path string) (string, error) {
	fullPath := filepath.Join(s.basePath, path)
	absBase, err := filepath.Abs(s.basePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve full path: %w", err)
	}
	if !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) && absFull != absBase {
		return "", fmt.Errorf("path traversal detected")
	}
	return absFull, nil
}

// Save stores a file at the given path relative to basePath
func (s *LocalFileStorage) Save(ctx context.Context, path string, reader io.Reader, size int64) error {
	fullPath, err := s.containedPath(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// Delete removes a file at the given path relative to basePath
func (s *LocalFileStorage) Delete(ctx context.Context, path string) error {
	fullPath, err := s.containedPath(path)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// GetPath returns the absolute filesystem path for serving
func (s *LocalFileStorage) GetPath(ctx context.Context, path string) string {
	fullPath, err := s.containedPath(path)
	if err != nil {
		return "" // caller should check for empty string
	}
	return fullPath
}

type countingReader struct {
	r io.Reader
	n int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	cr.n += int64(n)
	return n, err
}

// WalletService handles wallet document business logic
type WalletService struct {
	store       store.WalletStore
	tripStore   store.TripStore
	fileStorage FileStorage
	signingKey  []byte
}

// NewWalletService creates a new wallet service
func NewWalletService(walletStore store.WalletStore, tripStore store.TripStore, fileStorage FileStorage, signingKey string) *WalletService {
	return &WalletService{
		store:       walletStore,
		tripStore:   tripStore,
		fileStorage: fileStorage,
		signingKey:  []byte(signingKey),
	}
}

// UploadDocument handles uploading a document file and creating the database record
func (s *WalletService) UploadDocument(ctx context.Context, userID string, file io.Reader, fileSize int64, create *types.WalletDocumentCreate, fileName, mimeType string) (*types.WalletDocumentResponse, error) {
	// Server-side MIME detection: sniff the first 512 bytes to verify content type
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(file, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	detectedMIME := mimetype.Detect(sniffBuf[:n]).String()
	// Reconstruct reader with sniffed bytes prepended
	file = io.MultiReader(bytes.NewReader(sniffBuf[:n]), file)
	cr := &countingReader{r: file}

	// Use detected MIME type (ignore client-provided Content-Type)
	mimeType = detectedMIME

	// Validate MIME type
	if !allowedMimeTypes[mimeType] {
		return nil, apperrors.ValidationFailed("invalid_mime_type", fmt.Sprintf("MIME type %s is not allowed. Allowed: pdf, jpeg, png, heic", mimeType))
	}

	// Validate wallet type constraints
	if create.WalletType == types.WalletTypeGroup && (create.TripID == nil || *create.TripID == "") {
		return nil, apperrors.ValidationFailed("missing_trip_id", "trip ID is required for group documents")
	}
	if create.WalletType == types.WalletTypePersonal && create.TripID != nil {
		create.TripID = nil // personal documents cannot have a trip ID
	}

	// Generate storage path: <walletType>/<userID>/<timestamp>_<filename>
	storagePath := fmt.Sprintf("%s/%s/%d_%s", create.WalletType, userID, time.Now().UnixNano(), sanitizeFilename(fileName))

	// Save file to storage
	if err := s.fileStorage.Save(ctx, storagePath, cr, fileSize); err != nil {
		// Clean up partial file on save failure
		_ = s.fileStorage.Delete(ctx, storagePath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Validate actual bytes written (cr.n is authoritative; ignores client-reported fileSize)
	if cr.n > MaxFileSize {
		_ = s.fileStorage.Delete(ctx, storagePath)
		return nil, apperrors.ValidationFailed("file_too_large", fmt.Sprintf("file size %d exceeds maximum of %d bytes", cr.n, MaxFileSize))
	}

	// Create database record
	doc := &types.WalletDocument{
		UserID:       userID,
		TripID:       create.TripID,
		WalletType:   create.WalletType,
		DocumentType: create.DocumentType,
		Name:         create.Name,
		Description:  create.Description,
		FilePath:     storagePath,
		FileSize:     cr.n,
		MimeType:     mimeType,
		Metadata:     create.Metadata,
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]interface{}{}
	}

	id, err := s.store.CreateDocument(ctx, doc)
	if err != nil {
		// Attempt to clean up the uploaded file on DB failure
		_ = s.fileStorage.Delete(ctx, storagePath)
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	doc.ID = id
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()

	return &types.WalletDocumentResponse{
		WalletDocument: *doc,
		DownloadURL:    s.GenerateSignedURL(storagePath, 1*time.Hour),
	}, nil
}

// ListPersonalDocuments returns paginated personal documents for a user
func (s *WalletService) ListPersonalDocuments(ctx context.Context, userID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	return s.store.ListPersonalDocuments(ctx, userID, limit, offset)
}

// ListGroupDocuments returns paginated group documents for a trip
func (s *WalletService) ListGroupDocuments(ctx context.Context, tripID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	return s.store.ListGroupDocuments(ctx, tripID, limit, offset)
}

// GetDocument retrieves a document with a signed download URL
func (s *WalletService) GetDocument(ctx context.Context, id, userID string) (*types.WalletDocumentResponse, error) {
	doc, err := s.store.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify access: personal = owner only, group = trip member
	if doc.WalletType == types.WalletTypePersonal && doc.UserID != userID {
		return nil, apperrors.AuthorizationFailed("forbidden", "you do not have access to this document")
	}
	if doc.WalletType == types.WalletTypeGroup && doc.TripID != nil {
		_, roleErr := s.tripStore.GetUserRole(ctx, *doc.TripID, userID)
		if roleErr != nil {
			return nil, apperrors.AuthorizationFailed("forbidden", "you must be a trip member to access this document")
		}
	}

	return &types.WalletDocumentResponse{
		WalletDocument: *doc,
		DownloadURL:    s.GenerateSignedURL(doc.FilePath, 1*time.Hour),
	}, nil
}

// UpdateDocument updates a document's metadata
func (s *WalletService) UpdateDocument(ctx context.Context, id, userID string, update *types.WalletDocumentUpdate) (*types.WalletDocument, error) {
	// Check ownership
	doc, err := s.store.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}

	if doc.UserID != userID {
		return nil, apperrors.AuthorizationFailed("forbidden", "you do not have permission to update this document")
	}

	return s.store.UpdateDocument(ctx, id, update)
}

// DeleteDocument soft-deletes a document and removes the file
func (s *WalletService) DeleteDocument(ctx context.Context, id, userID string) error {
	doc, err := s.store.GetDocument(ctx, id)
	if err != nil {
		return err
	}

	if doc.UserID != userID {
		return apperrors.AuthorizationFailed("forbidden", "you do not have permission to delete this document")
	}

	if err := s.store.SoftDeleteDocument(ctx, id); err != nil {
		return err
	}

	// Delete file from storage (best effort)
	_ = s.fileStorage.Delete(ctx, doc.FilePath)

	return nil
}

// GenerateSignedURL creates an HMAC-signed download URL token.
// The raw format is hex(hmac(path|expiry))|path|expiry, then base64url-encoded
// to avoid issues with / and | characters in URL path parameters.
func (s *WalletService) GenerateSignedURL(docPath string, expiresIn time.Duration) string {
	expiry := time.Now().Add(expiresIn).Unix()
	message := fmt.Sprintf("%s|%d", docPath, expiry)

	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))

	raw := fmt.Sprintf("%s|%s|%d", sig, docPath, expiry)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ValidateSignedURL validates an HMAC-signed token and returns the file path
func (s *WalletService) ValidateSignedURL(token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", apperrors.ValidationFailed("invalid_token", "malformed download token")
	}

	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return "", apperrors.ValidationFailed("invalid_token", "malformed download token")
	}

	sig, docPath, expiryStr := parts[0], parts[1], parts[2]

	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", apperrors.ValidationFailed("invalid_token", "invalid expiry in token")
	}

	if time.Now().Unix() > expiry {
		return "", apperrors.ValidationFailed("token_expired", "download link has expired")
	}

	// Recompute HMAC
	message := fmt.Sprintf("%s|%d", docPath, expiry)
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", apperrors.ValidationFailed("invalid_token", "invalid signature")
	}

	return docPath, nil
}

// ServeFile validates a signed token and returns the local filesystem path
func (s *WalletService) ServeFile(ctx context.Context, token string) (string, error) {
	docPath, err := s.ValidateSignedURL(token)
	if err != nil {
		return "", err
	}
	return s.fileStorage.GetPath(ctx, docPath), nil
}

var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9._\-]`)

// sanitizeFilename removes path separators and dangerous characters from a filename.
// Preserves the file extension when truncating long names.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = safeFilenameRe.ReplaceAllString(name, "_")
	if name == "" || name == "." || name == ".." {
		name = "upload"
	}
	if len(name) > 255 {
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		maxStem := 255 - len(ext)
		if maxStem < 1 {
			maxStem = 1
		}
		if len(stem) > maxStem {
			stem = stem[:maxStem]
		}
		name = stem + ext
	}
	return name
}
