package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	walletSvc "github.com/NomadCrew/nomad-crew-backend/models/wallet/service"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
)

// Allowed MIME types for poll images
var pollImageAllowedMimes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/heic": true,
	"image/heif": true,
	"image/webp": true,
}

// MaxPollImageSize is 10MB (same as wallet)
const MaxPollImageSize = 10 * 1024 * 1024

// PollImageHandler handles poll image uploads.
// It reuses the FileStorage interface from the wallet feature.
type PollImageHandler struct {
	fileStorage walletSvc.FileStorage
	signingKey  []byte
}

// NewPollImageHandler creates a new poll image handler.
func NewPollImageHandler(fileStorage walletSvc.FileStorage, signingKey string) *PollImageHandler {
	return &PollImageHandler{
		fileStorage: fileStorage,
		signingKey:  []byte(signingKey),
	}
}

// UploadPollImageHandler handles image uploads for poll options.
// POST /v1/trips/:id/poll-images
// Accepts multipart with "file" field. Returns JSON with imageUrl.
func (h *PollImageHandler) UploadPollImageHandler(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" || !isValidUUID(tripID) {
		_ = c.Error(apperrors.ValidationFailed("validation_failed", "valid trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Enforce max body size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxPollImageSize+1024*1024)

	if err := c.Request.ParseMultipartForm(MaxPollImageSize); err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_form", "failed to parse multipart form"))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		_ = c.Error(apperrors.ValidationFailed("missing_file", "file field is required"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_file", "failed to open uploaded file"))
		return
	}
	defer file.Close()

	// Server-side MIME detection
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(file, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		_ = c.Error(fmt.Errorf("failed to read file header: %w", err))
		return
	}
	detectedMIME := mimetype.Detect(sniffBuf[:n]).String()

	if !pollImageAllowedMimes[detectedMIME] {
		_ = c.Error(apperrors.ValidationFailed("invalid_mime_type",
			fmt.Sprintf("MIME type %s is not allowed. Allowed: jpeg, png, heic, webp", detectedMIME)))
		return
	}

	// Reconstruct reader with sniffed bytes prepended
	reader := io.MultiReader(bytes.NewReader(sniffBuf[:n]), file)

	// Counting reader to validate actual bytes
	cr := &countingReader{r: reader}

	// Storage path: poll-images/<tripID>/<userID>/<timestamp>_<filename>
	sanitized := walletSvc.SanitizeFilename(fileHeader.Filename)
	storagePath := fmt.Sprintf("poll-images/%s/%s/%d_%s", tripID, userID, time.Now().UnixNano(), sanitized)

	if err := h.fileStorage.Save(c.Request.Context(), storagePath, cr, fileHeader.Size); err != nil {
		_ = h.fileStorage.Delete(c.Request.Context(), storagePath)
		_ = c.Error(fmt.Errorf("failed to save file: %w", err))
		return
	}

	if cr.n > MaxPollImageSize {
		_ = h.fileStorage.Delete(c.Request.Context(), storagePath)
		_ = c.Error(apperrors.ValidationFailed("file_too_large",
			fmt.Sprintf("file size %d exceeds maximum of %d bytes", cr.n, MaxPollImageSize)))
		return
	}

	// Generate a signed URL for the image
	imageURL := walletSvc.GenerateSignedURLStatic(h.signingKey, storagePath, 365*24*time.Hour)

	c.JSON(http.StatusCreated, gin.H{
		"imageUrl":    imageURL,
		"storagePath": storagePath,
		"mimeType":    detectedMIME,
		"fileSize":    cr.n,
	})
}

// countingReader wraps a reader and counts bytes read.
type countingReader struct {
	r io.Reader
	n int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	cr.n += int64(n)
	return n, err
}
