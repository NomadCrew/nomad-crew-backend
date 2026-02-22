package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadCrew/nomad-crew-backend/middleware"
	walletSvc "github.com/NomadCrew/nomad-crew-backend/models/wallet/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// MockFileStorage implements walletSvc.FileStorage for poll image handler tests.
// ---------------------------------------------------------------------------

type MockFileStorage struct {
	mock.Mock
}

func (m *MockFileStorage) Save(ctx context.Context, path string, reader io.Reader, size int64) error {
	args := m.Called(ctx, path, reader, size)
	return args.Error(0)
}

func (m *MockFileStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockFileStorage) GetURL(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	return args.String(0), args.Error(1)
}

// compile-time check
var _ walletSvc.FileStorage = (*MockFileStorage)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testSigningKey = "test-signing-key-for-poll-images"

func setupPollImageHandler() (*PollImageHandler, *MockFileStorage) {
	fs := new(MockFileStorage)
	h := NewPollImageHandler(fs, testSigningKey)
	return h, fs
}

func buildPollImageRouter(path, method string, handler gin.HandlerFunc, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.UserIDKey), userID)
		}
		c.Next()
	})
	switch method {
	case http.MethodPost:
		r.POST(path, handler)
	case http.MethodGet:
		r.GET(path, handler)
	}
	return r
}

// buildPollImageBody creates a multipart/form-data body with just a "file" field.
func buildPollImageBody(t *testing.T, filename string, fileContent []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		assert.NoError(t, err)
		if fileContent != nil {
			_, _ = fw.Write(fileContent)
		}
	}

	w.Close()
	return &buf, w.FormDataContentType()
}

// jpegMagicBytes returns minimal JPEG magic bytes (512 bytes for sniff buffer).
func jpegMagicBytes() []byte {
	return append([]byte("\xff\xd8\xff\xe0"), make([]byte, 508)...)
}

// pngMagicBytes returns minimal PNG magic bytes.
func pngMagicBytes() []byte {
	header := []byte("\x89PNG\r\n\x1a\n")
	return append(header, make([]byte, 504)...)
}

// ---------------------------------------------------------------------------
// Upload Success Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_JPEG_Success(t *testing.T) {
	handler, fs := setupPollImageHandler()
	body, ct := buildPollImageBody(t, "beach.jpg", jpegMagicBytes())

	fs.On("Save", mock.Anything, mock.MatchedBy(func(path string) bool {
		return len(path) > 0
	}), mock.Anything, mock.AnythingOfType("int64")).Return(nil)

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["imageUrl"])
	assert.NotEmpty(t, resp["storagePath"])
	assert.Equal(t, "image/jpeg", resp["mimeType"])
	fs.AssertExpectations(t)
}

func TestPollImageUpload_PNG_Success(t *testing.T) {
	handler, fs := setupPollImageHandler()
	body, ct := buildPollImageBody(t, "food.png", pngMagicBytes())

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("int64")).Return(nil)

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "image/png", resp["mimeType"])
	fs.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// MIME Type Rejection Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_WrongMIME_Rejected(t *testing.T) {
	handler, _ := setupPollImageHandler()
	// PDF magic bytes — not an allowed image type
	pdfContent := append([]byte("%PDF-1.4"), make([]byte, 504)...)
	body, ct := buildPollImageBody(t, "document.pdf", pdfContent)

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPollImageUpload_TextFile_Rejected(t *testing.T) {
	handler, _ := setupPollImageHandler()
	textContent := []byte("this is just plain text content that should not be accepted as an image")
	body, ct := buildPollImageBody(t, "notes.txt", textContent)

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Oversize File Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_Oversized_Rejected(t *testing.T) {
	handler, _ := setupPollImageHandler()

	// Build a body exceeding MaxPollImageSize + 1MB overhead
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "huge.jpg")
	const oversizeBytes = (10*1024*1024 + 1024*1024 + 1)
	fw.Write(make([]byte, oversizeBytes))
	mw.Close()

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// MaxBytesReader causes ParseMultipartForm to fail -> 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Authentication Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_Unauthenticated(t *testing.T) {
	handler, _ := setupPollImageHandler()
	body, ct := buildPollImageBody(t, "photo.jpg", jpegMagicBytes())

	// No userID injected
	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, "")
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// Invalid Trip ID Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_InvalidTripUUID(t *testing.T) {
	handler, _ := setupPollImageHandler()
	body, ct := buildPollImageBody(t, "photo.jpg", jpegMagicBytes())

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, "/v1/trips/not-a-uuid/poll-images", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPollImageUpload_EmptyTripID(t *testing.T) {
	handler, _ := setupPollImageHandler()
	body, ct := buildPollImageBody(t, "photo.jpg", jpegMagicBytes())

	// Register on a path without :id param to simulate empty trip ID
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.UserIDKey), testUserID)
		c.Next()
	})
	r.POST("/v1/trips/:id/poll-images", handler.UploadPollImageHandler)

	// Gin will not match if we omit the segment, so just use a short invalid uuid
	req, _ := http.NewRequest(http.MethodPost, "/v1/trips/abc/poll-images", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Missing File Field Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_MissingFile(t *testing.T) {
	handler, _ := setupPollImageHandler()

	// Build a multipart body with no "file" field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormField("metadata")
	io.WriteString(fw, `{"some":"data"}`)
	mw.Close()

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Storage Path Tests
// ---------------------------------------------------------------------------

func TestPollImageUpload_StoragePathContainsTripAndUser(t *testing.T) {
	handler, fs := setupPollImageHandler()
	body, ct := buildPollImageBody(t, "sunset.jpg", jpegMagicBytes())

	var savedPath string
	fs.On("Save", mock.Anything, mock.MatchedBy(func(path string) bool {
		savedPath = path
		return true
	}), mock.Anything, mock.AnythingOfType("int64")).Return(nil)

	r := buildPollImageRouter("/v1/trips/:id/poll-images", http.MethodPost, handler.UploadPollImageHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/poll-images", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, savedPath, "poll-images/")
	assert.Contains(t, savedPath, testTripID)
	assert.Contains(t, savedPath, testUserID)
	assert.Contains(t, savedPath, "sunset.jpg")
	fs.AssertExpectations(t)
}
