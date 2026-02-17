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
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWalletService implements WalletServiceInterface for handler tests.
type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) UploadDocument(ctx context.Context, userID string, file io.Reader, fileSize int64, create *types.WalletDocumentCreate, fileName, mimeType string) (*types.WalletDocumentResponse, error) {
	args := m.Called(ctx, userID, file, fileSize, create, fileName, mimeType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WalletDocumentResponse), args.Error(1)
}

func (m *MockWalletService) ListPersonalDocuments(ctx context.Context, userID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.WalletDocument), args.Int(1), args.Error(2)
}

func (m *MockWalletService) ListGroupDocuments(ctx context.Context, tripID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	args := m.Called(ctx, tripID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.WalletDocument), args.Int(1), args.Error(2)
}

func (m *MockWalletService) GetDocument(ctx context.Context, id, userID string) (*types.WalletDocumentResponse, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WalletDocumentResponse), args.Error(1)
}

func (m *MockWalletService) UpdateDocument(ctx context.Context, id, userID string, update *types.WalletDocumentUpdate) (*types.WalletDocument, error) {
	args := m.Called(ctx, id, userID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WalletDocument), args.Error(1)
}

func (m *MockWalletService) DeleteDocument(ctx context.Context, id, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockWalletService) ServeFile(ctx context.Context, token string) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}

// compile-time check
var _ WalletServiceInterface = (*MockWalletService)(nil)

// helpers

const (
	testUserID = "11111111-1111-1111-1111-111111111111"
	testDocID  = "22222222-2222-2222-2222-222222222222"
	testTripID = "33333333-3333-3333-3333-333333333333"
)

func setupWalletHandler() (*WalletHandler, *MockWalletService) {
	svc := new(MockWalletService)
	h := NewWalletHandler(svc)
	return h, svc
}

// buildRouter wraps a handler in a Gin router with the error handler middleware,
// matching the production setup so c.Error() calls produce the correct HTTP status.
func buildWalletRouter(path, method string, handler gin.HandlerFunc, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	// inject userID from test into the request context
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.UserIDKey), userID)
		}
		c.Next()
	})
	switch method {
	case http.MethodGet:
		r.GET(path, handler)
	case http.MethodPost:
		r.POST(path, handler)
	case http.MethodPut:
		r.PUT(path, handler)
	case http.MethodDelete:
		r.DELETE(path, handler)
	}
	return r
}

// buildMultipartBody creates a multipart/form-data body with a "file" field and a "metadata" JSON field.
func buildMultipartBody(t *testing.T, filename, metadataJSON string, fileContent []byte) (*bytes.Buffer, string) {
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

	if metadataJSON != "" {
		fw, err := w.CreateFormField("metadata")
		assert.NoError(t, err)
		_, _ = io.WriteString(fw, metadataJSON)
	}

	w.Close()
	return &buf, w.FormDataContentType()
}

func sampleDocResponse() *types.WalletDocumentResponse {
	return &types.WalletDocumentResponse{
		WalletDocument: types.WalletDocument{
			ID:           testDocID,
			UserID:       testUserID,
			WalletType:   types.WalletTypePersonal,
			DocumentType: types.DocumentTypePassport,
			Name:         "My Passport",
			FileSize:     1024,
			MimeType:     "image/jpeg",
			Metadata:     map[string]interface{}{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		DownloadURL: "https://example.com/download/token",
	}
}

// ---------------------------------------------------------------------------
// parseMultipartUpload tests
// ---------------------------------------------------------------------------

func TestParseMultipartUpload_MissingFile(t *testing.T) {
	// metadata present but no file field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormField("metadata")
	_, _ = io.WriteString(fw, `{"walletType":"personal","documentType":"passport","name":"doc"}`)
	mw.Close()

	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseMultipartUpload_MissingMetadata(t *testing.T) {
	// file present but no metadata field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "passport.jpg")
	// write minimal valid JPEG magic bytes so MIME sniffing doesn't choke
	fw.Write([]byte("\xff\xd8\xff"))
	mw.Close()

	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseMultipartUpload_InvalidJSONMetadata(t *testing.T) {
	body, ct := buildMultipartBody(t, "passport.jpg", `{not valid json}`, []byte("%PDF-1.4"))
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseMultipartUpload_BodyExceedsMaxBytes(t *testing.T) {
	// Build a body that exceeds MaxFileSize+1MB limit by writing a large "file".
	// We use a pipe so we don't allocate 11MB in memory — the reader will be limited
	// by MaxBytesReader before it's fully consumed.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "big.pdf")
	// Write 11MB of zeros — exceeds the 10MB file + 1MB form overhead limit
	const oversizeBytes = (10*1024*1024 + 1024*1024 + 1) // 11MB + 1 byte
	fw.Write(make([]byte, oversizeBytes))
	mf, _ := mw.CreateFormField("metadata")
	io.WriteString(mf, `{"walletType":"personal","documentType":"passport","name":"big"}`)
	mw.Close()

	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// MaxBytesReader causes ParseMultipartForm to fail → 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// UploadPersonalDocumentHandler tests
// ---------------------------------------------------------------------------

func TestUploadPersonalDocumentHandler_Unauthenticated(t *testing.T) {
	body, ct := buildMultipartBody(t, "doc.pdf", `{"walletType":"personal","documentType":"passport","name":"doc"}`, []byte("%PDF-1.4"))
	handler, _ := setupWalletHandler()
	// no userID injected → empty string
	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, "")

	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUploadPersonalDocumentHandler_Success(t *testing.T) {
	// JPEG magic bytes so MIME sniffing returns image/jpeg (allowed type)
	jpegBytes := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 508)...)
	metaJSON := `{"walletType":"personal","documentType":"passport","name":"My Passport"}`
	body, ct := buildMultipartBody(t, "passport.jpg", metaJSON, jpegBytes)

	handler, svc := setupWalletHandler()
	resp := sampleDocResponse()
	svc.On("UploadDocument", mock.Anything, testUserID, mock.Anything, mock.AnythingOfType("int64"), mock.Anything, "passport.jpg", mock.AnythingOfType("string")).
		Return(resp, nil)

	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, testDocID, got["id"])
	svc.AssertExpectations(t)
}

func TestUploadPersonalDocumentHandler_ServiceError(t *testing.T) {
	jpegBytes := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 508)...)
	body, ct := buildMultipartBody(t, "passport.jpg", `{"walletType":"personal","documentType":"passport","name":"doc"}`, jpegBytes)

	handler, svc := setupWalletHandler()
	svc.On("UploadDocument", mock.Anything, testUserID, mock.Anything, mock.AnythingOfType("int64"), mock.Anything, "passport.jpg", mock.AnythingOfType("string")).
		Return(nil, apperrors.InternalServerError("storage error"))

	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// UploadGroupDocumentHandler tests
// ---------------------------------------------------------------------------

func TestUploadGroupDocumentHandler_InvalidTripUUID(t *testing.T) {
	body, ct := buildMultipartBody(t, "doc.pdf", `{"walletType":"group","documentType":"passport","name":"doc"}`, []byte("%PDF-1.4"))
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodPost, handler.UploadGroupDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodPost, "/v1/trips/not-a-uuid/wallet/documents", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadGroupDocumentHandler_Unauthenticated(t *testing.T) {
	body, ct := buildMultipartBody(t, "doc.pdf", `{"walletType":"group","documentType":"passport","name":"doc"}`, []byte("%PDF-1.4"))
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodPost, handler.UploadGroupDocumentHandler, "")

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/wallet/documents", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUploadGroupDocumentHandler_Success(t *testing.T) {
	jpegBytes := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 508)...)
	metaJSON := `{"walletType":"group","documentType":"passport","name":"Group Doc"}`
	body, ct := buildMultipartBody(t, "group.jpg", metaJSON, jpegBytes)

	handler, svc := setupWalletHandler()
	tripIDCopy := testTripID
	resp := &types.WalletDocumentResponse{
		WalletDocument: types.WalletDocument{
			ID:           testDocID,
			UserID:       testUserID,
			TripID:       &tripIDCopy,
			WalletType:   types.WalletTypeGroup,
			DocumentType: types.DocumentTypePassport,
			Name:         "Group Doc",
			FileSize:     512,
			MimeType:     "image/jpeg",
			Metadata:     map[string]interface{}{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		DownloadURL: "https://example.com/download/token",
	}
	svc.On("UploadDocument", mock.Anything, testUserID, mock.Anything, mock.AnythingOfType("int64"), mock.Anything, "group.jpg", mock.AnythingOfType("string")).
		Return(resp, nil)

	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodPost, handler.UploadGroupDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/wallet/documents", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListPersonalDocumentsHandler tests
// ---------------------------------------------------------------------------

func TestListPersonalDocumentsHandler_Unauthenticated(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents", http.MethodGet, handler.ListPersonalDocumentsHandler, "")

	req, _ := http.NewRequest(http.MethodGet, "/v1/wallet/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListPersonalDocumentsHandler_Success(t *testing.T) {
	handler, svc := setupWalletHandler()
	docs := []*types.WalletDocument{
		{ID: testDocID, UserID: testUserID, WalletType: types.WalletTypePersonal, DocumentType: types.DocumentTypePassport, Name: "Passport"},
	}
	svc.On("ListPersonalDocuments", mock.Anything, testUserID, 20, 0).Return(docs, 1, nil)

	r := buildWalletRouter("/v1/wallet/documents", http.MethodGet, handler.ListPersonalDocumentsHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, "/v1/wallet/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["pagination"].(map[string]interface{})["total"])
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListGroupDocumentsHandler tests
// ---------------------------------------------------------------------------

func TestListGroupDocumentsHandler_InvalidTripUUID(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodGet, handler.ListGroupDocumentsHandler, testUserID)

	req, _ := http.NewRequest(http.MethodGet, "/v1/trips/not-a-uuid/wallet/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListGroupDocumentsHandler_Unauthenticated(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodGet, handler.ListGroupDocumentsHandler, "")

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/wallet/documents", testTripID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListGroupDocumentsHandler_Success(t *testing.T) {
	handler, svc := setupWalletHandler()
	tripIDCopy := testTripID
	docs := []*types.WalletDocument{
		{ID: testDocID, UserID: testUserID, TripID: &tripIDCopy, WalletType: types.WalletTypeGroup, DocumentType: types.DocumentTypePassport, Name: "Group Doc"},
	}
	svc.On("ListGroupDocuments", mock.Anything, testTripID, 20, 0).Return(docs, 1, nil)

	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodGet, handler.ListGroupDocumentsHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/trips/%s/wallet/documents", testTripID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// GetDocumentHandler tests
// ---------------------------------------------------------------------------

func TestGetDocumentHandler_InvalidDocUUID(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodGet, handler.GetDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodGet, "/v1/wallet/documents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetDocumentHandler_Unauthenticated(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodGet, handler.GetDocumentHandler, "")

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetDocumentHandler_Success(t *testing.T) {
	handler, svc := setupWalletHandler()
	resp := sampleDocResponse()
	svc.On("GetDocument", mock.Anything, testDocID, testUserID).Return(resp, nil)

	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodGet, handler.GetDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, testDocID, body["id"])
	svc.AssertExpectations(t)
}

func TestGetDocumentHandler_NotFound(t *testing.T) {
	handler, svc := setupWalletHandler()
	svc.On("GetDocument", mock.Anything, testDocID, testUserID).
		Return(nil, apperrors.NotFound("Document", testDocID))

	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodGet, handler.GetDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// UpdateDocumentHandler tests
// ---------------------------------------------------------------------------

func TestUpdateDocumentHandler_InvalidDocUUID(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodPut, handler.UpdateDocumentHandler, testUserID)

	body, _ := json.Marshal(types.WalletDocumentUpdate{})
	req, _ := http.NewRequest(http.MethodPut, "/v1/wallet/documents/not-a-uuid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateDocumentHandler_Unauthenticated(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodPut, handler.UpdateDocumentHandler, "")

	body, _ := json.Marshal(types.WalletDocumentUpdate{})
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateDocumentHandler_Success(t *testing.T) {
	handler, svc := setupWalletHandler()
	newName := "Updated Name"
	update := types.WalletDocumentUpdate{Name: &newName}
	updated := &types.WalletDocument{
		ID:           testDocID,
		UserID:       testUserID,
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         newName,
	}
	svc.On("UpdateDocument", mock.Anything, testDocID, testUserID, mock.MatchedBy(func(u *types.WalletDocumentUpdate) bool {
		return u.Name != nil && *u.Name == newName
	})).Return(updated, nil)

	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodPut, handler.UpdateDocumentHandler, testUserID)
	body, _ := json.Marshal(update)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// DeleteDocumentHandler tests
// ---------------------------------------------------------------------------

func TestDeleteDocumentHandler_InvalidDocUUID(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodDelete, handler.DeleteDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodDelete, "/v1/wallet/documents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteDocumentHandler_Unauthenticated(t *testing.T) {
	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodDelete, handler.DeleteDocumentHandler, "")

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteDocumentHandler_Success(t *testing.T) {
	handler, svc := setupWalletHandler()
	svc.On("DeleteDocument", mock.Anything, testDocID, testUserID).Return(nil)

	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodDelete, handler.DeleteDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Document deleted successfully", body["message"])
	svc.AssertExpectations(t)
}

func TestDeleteDocumentHandler_Forbidden(t *testing.T) {
	handler, svc := setupWalletHandler()
	svc.On("DeleteDocument", mock.Anything, testDocID, testUserID).
		Return(apperrors.AuthorizationFailed("forbidden", "you do not have permission to delete this document"))

	r := buildWalletRouter("/v1/wallet/documents/:docID", http.MethodDelete, handler.DeleteDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/wallet/documents/%s", testDocID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ServeFileHandler tests
// ---------------------------------------------------------------------------

func TestServeFileHandler_EmptyToken(t *testing.T) {
	handler, _ := setupWalletHandler()
	// Register handler on a path that allows an empty-looking token via a static route.
	// The handler checks token == "" and returns 400 (missing_token).
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	// Use a wildcard so a request to /v1/wallet/files/ reaches the handler with empty token param.
	r.GET("/v1/wallet/files/:token", handler.ServeFileHandler)

	// Gin will not match /:token with an empty segment, so "/v1/wallet/files/" returns 301 or 404.
	// The real guard is tested via TestServeFileHandler_InvalidToken (non-empty bad token → 400).
	// Here we just verify that an omitted path segment yields a non-200.
	req, _ := http.NewRequest(http.MethodGet, "/v1/wallet/files/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Gin redirects trailing-slash requests → 301, or 404 depending on RedirectTrailingSlash.
	// Either way it should not be 200 or 201.
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.NotEqual(t, http.StatusCreated, w.Code)
}

func TestServeFileHandler_InvalidToken(t *testing.T) {
	handler, svc := setupWalletHandler()
	svc.On("ServeFile", mock.Anything, "badtoken").
		Return("", apperrors.ValidationFailed("invalid_token", "invalid signature"))

	r := buildWalletRouter("/v1/wallet/files/:token", http.MethodGet, handler.ServeFileHandler, testUserID)
	req, _ := http.NewRequest(http.MethodGet, "/v1/wallet/files/badtoken", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestServeFileHandler_ExpiredToken(t *testing.T) {
	handler, svc := setupWalletHandler()
	svc.On("ServeFile", mock.Anything, "expiredtoken").
		Return("", apperrors.ValidationFailed("token_expired", "download link has expired"))

	r := buildWalletRouter("/v1/wallet/files/:token", http.MethodGet, handler.ServeFileHandler, "")
	req, _ := http.NewRequest(http.MethodGet, "/v1/wallet/files/expiredtoken", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// metadataStr=="" fix: verify no panic when file is closed before returning error
// ---------------------------------------------------------------------------

func TestParseMultipartUpload_MetadataEmptyBranchNoPanic(t *testing.T) {
	// Regression test for the fix: when metadataStr == "" we must return a proper error,
	// not the stale err from fileHeader.Open() (which could be nil, causing a nil return
	// that would lead to a nil-deref panic on defer doc.file.Close()).
	//
	// We simulate this by sending a form with a file but no metadata field.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "test.jpg")
	fw.Write([]byte("\xff\xd8\xff"))
	mw.Close()

	handler, _ := setupWalletHandler()
	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)

	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	// Must not panic
	assert.NotPanics(t, func() {
		r.ServeHTTP(w, req)
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Multipart: verify WalletType is forced to personal/group regardless of metadata
// ---------------------------------------------------------------------------

func TestUploadPersonalDocumentHandler_ForcePersonalWalletType(t *testing.T) {
	// Even if metadata says "group", personal handler forces walletType = personal
	jpegBytes := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 508)...)
	// metadata has walletType=group but handler should override it
	body, ct := buildMultipartBody(t, "doc.jpg", `{"walletType":"group","documentType":"passport","name":"doc","tripId":"some-trip"}`, jpegBytes)

	handler, svc := setupWalletHandler()
	resp := sampleDocResponse()
	svc.On("UploadDocument", mock.Anything, testUserID, mock.Anything, mock.AnythingOfType("int64"),
		mock.MatchedBy(func(c *types.WalletDocumentCreate) bool {
			return c.WalletType == types.WalletTypePersonal && c.TripID == nil
		}),
		"doc.jpg", mock.AnythingOfType("string"),
	).Return(resp, nil)

	r := buildWalletRouter("/v1/wallet/documents", http.MethodPost, handler.UploadPersonalDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, "/v1/wallet/documents", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestUploadGroupDocumentHandler_ForceGroupWalletTypeWithTripID(t *testing.T) {
	jpegBytes := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 508)...)
	// metadata says personal, but the group handler must override walletType=group + tripID from route
	body, ct := buildMultipartBody(t, "doc.jpg", `{"walletType":"personal","documentType":"passport","name":"doc"}`, jpegBytes)

	handler, svc := setupWalletHandler()
	resp := sampleDocResponse()
	svc.On("UploadDocument", mock.Anything, testUserID, mock.Anything, mock.AnythingOfType("int64"),
		mock.MatchedBy(func(c *types.WalletDocumentCreate) bool {
			return c.WalletType == types.WalletTypeGroup && c.TripID != nil && *c.TripID == testTripID
		}),
		"doc.jpg", mock.AnythingOfType("string"),
	).Return(resp, nil)

	r := buildWalletRouter("/v1/trips/:id/wallet/documents", http.MethodPost, handler.UploadGroupDocumentHandler, testUserID)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/trips/%s/wallet/documents", testTripID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

