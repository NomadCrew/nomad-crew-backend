package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	walletSvc "github.com/NomadCrew/nomad-crew-backend/models/wallet/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
)

// WalletServiceInterface defines the methods used by WalletHandler,
// allowing the handler to be tested with mocks.
type WalletServiceInterface interface {
	UploadDocument(ctx context.Context, userID string, file io.Reader, fileSize int64, create *types.WalletDocumentCreate, fileName, mimeType string) (*types.WalletDocumentResponse, error)
	ListPersonalDocuments(ctx context.Context, userID string, limit, offset int) ([]*types.WalletDocument, int, error)
	ListGroupDocuments(ctx context.Context, tripID string, limit, offset int) ([]*types.WalletDocument, int, error)
	GetDocument(ctx context.Context, id, userID string) (*types.WalletDocumentResponse, error)
	UpdateDocument(ctx context.Context, id, userID string, update *types.WalletDocumentUpdate) (*types.WalletDocument, error)
	DeleteDocument(ctx context.Context, id, userID string) error
	ServeFile(ctx context.Context, token string) (string, string, error)
}

// Ensure the concrete service satisfies the interface at compile time.
var _ WalletServiceInterface = (*walletSvc.WalletService)(nil)

type WalletHandler struct {
	walletService WalletServiceInterface
}

func NewWalletHandler(walletService WalletServiceInterface) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
	}
}

// auditCtx enriches the request context with client IP and User-Agent for audit logging.
func auditCtx(c *gin.Context) context.Context {
	return walletSvc.WithAuditMeta(c.Request.Context(), c.ClientIP(), c.Request.UserAgent())
}

// UploadPersonalDocumentHandler handles personal document uploads
// POST /v1/wallet/documents (multipart)
func (h *WalletHandler) UploadPersonalDocumentHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	doc, err := h.parseMultipartUpload(c)
	if err != nil {
		return // error already set on context
	}
	defer doc.file.Close()

	// Force personal wallet type
	doc.create.WalletType = types.WalletTypePersonal
	doc.create.TripID = nil

	resp, err := h.walletService.UploadDocument(auditCtx(c), userID, doc.file, doc.fileSize, doc.create, doc.fileName, doc.mimeType)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// UploadGroupDocumentHandler handles group document uploads
// POST /v1/trips/:id/wallet/documents (multipart)
func (h *WalletHandler) UploadGroupDocumentHandler(c *gin.Context) {
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

	doc, err := h.parseMultipartUpload(c)
	if err != nil {
		return // error already set on context
	}
	defer doc.file.Close()

	// Force group wallet type with the trip ID from the route
	doc.create.WalletType = types.WalletTypeGroup
	doc.create.TripID = &tripID

	resp, err := h.walletService.UploadDocument(auditCtx(c), userID, doc.file, doc.fileSize, doc.create, doc.fileName, doc.mimeType)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListPersonalDocumentsHandler lists personal documents for the authenticated user
// GET /v1/wallet/documents
func (h *WalletHandler) ListPersonalDocumentsHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	params := getPaginationParams(c, 20, 0)

	docs, total, err := h.walletService.ListPersonalDocuments(c.Request.Context(), userID, params.Limit, params.Offset)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": docs,
		"pagination": gin.H{
			"limit":  params.Limit,
			"offset": params.Offset,
			"total":  total,
		},
	})
}

// ListGroupDocumentsHandler lists group documents for a trip
// GET /v1/trips/:id/wallet/documents
func (h *WalletHandler) ListGroupDocumentsHandler(c *gin.Context) {
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

	params := getPaginationParams(c, 20, 0)

	docs, total, err := h.walletService.ListGroupDocuments(c.Request.Context(), tripID, params.Limit, params.Offset)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": docs,
		"pagination": gin.H{
			"limit":  params.Limit,
			"offset": params.Offset,
			"total":  total,
		},
	})
}

// GetDocumentHandler retrieves a single document with download URL
// GET /v1/wallet/documents/:docID
func (h *WalletHandler) GetDocumentHandler(c *gin.Context) {
	docID := c.Param("docID")
	if docID == "" || !isValidUUID(docID) {
		_ = c.Error(apperrors.ValidationFailed("validation_failed", "valid document ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	resp, err := h.walletService.GetDocument(auditCtx(c), docID, userID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateDocumentHandler updates a document's metadata
// PUT /v1/wallet/documents/:docID
func (h *WalletHandler) UpdateDocumentHandler(c *gin.Context) {
	docID := c.Param("docID")
	if docID == "" || !isValidUUID(docID) {
		_ = c.Error(apperrors.ValidationFailed("validation_failed", "valid document ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	var req types.WalletDocumentUpdate
	if !bindJSONOrError(c, &req) {
		return
	}

	doc, err := h.walletService.UpdateDocument(auditCtx(c), docID, userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, doc)
}

// DeleteDocumentHandler soft-deletes a document
// DELETE /v1/wallet/documents/:docID
func (h *WalletHandler) DeleteDocumentHandler(c *gin.Context) {
	docID := c.Param("docID")
	if docID == "" || !isValidUUID(docID) {
		_ = c.Error(apperrors.ValidationFailed("validation_failed", "valid document ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(apperrors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	if err := h.walletService.DeleteDocument(auditCtx(c), docID, userID); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Document deleted successfully",
	})
}

// ServeFileHandler serves a document file using a signed token
// GET /v1/wallet/files/:token
func (h *WalletHandler) ServeFileHandler(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		_ = c.Error(apperrors.ValidationFailed("missing_token", "download token is required"))
		return
	}

	filePath, mimeType, err := h.walletService.ServeFile(c.Request.Context(), token)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// Security headers for file downloads
	c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(filePath)+"\"")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}

	c.File(filePath)
}

// multipartUpload holds parsed multipart upload data
type multipartUpload struct {
	file     *multipartFileReader
	fileSize int64
	fileName string
	mimeType string
	create   *types.WalletDocumentCreate
}

// multipartFileReader wraps multipart.File to implement io.Reader with Close
type multipartFileReader struct {
	file interface {
		Read(p []byte) (n int, err error)
		Close() error
	}
}

func (r *multipartFileReader) Read(p []byte) (n int, err error) {
	return r.file.Read(p)
}

func (r *multipartFileReader) Close() error {
	return r.file.Close()
}

// parseMultipartUpload parses a multipart form upload containing a file and JSON metadata.
// The caller MUST call file.Close() on the returned multipartUpload when done.
func (h *WalletHandler) parseMultipartUpload(c *gin.Context) (*multipartUpload, error) {
	// Enforce max body size at the HTTP level to reject oversized requests early
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, walletSvc.MaxFileSize+1024*1024) // file + 1MB for form fields

	// Parse multipart form with 10MB max
	if err := c.Request.ParseMultipartForm(walletSvc.MaxFileSize); err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_form", "failed to parse multipart form"))
		return nil, err
	}

	// Get file from form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		_ = c.Error(apperrors.ValidationFailed("missing_file", "file field is required"))
		return nil, err
	}

	file, err := fileHeader.Open()
	if err != nil {
		_ = c.Error(apperrors.ValidationFailed("invalid_file", "failed to open uploaded file"))
		return nil, err
	}

	// Get metadata JSON from form field
	metadataStr := c.PostForm("metadata")
	if metadataStr == "" {
		file.Close()
		validationErr := apperrors.ValidationFailed("missing_metadata", "metadata field is required (JSON)")
		_ = c.Error(validationErr)
		return nil, validationErr
	}

	var create types.WalletDocumentCreate
	if err := json.Unmarshal([]byte(metadataStr), &create); err != nil {
		file.Close()
		_ = c.Error(apperrors.ValidationFailed("invalid_metadata", "metadata must be valid JSON"))
		return nil, err
	}

	// Detect MIME type from header (server-side detection happens in service layer)
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return &multipartUpload{
		file:     &multipartFileReader{file: file},
		fileSize: fileHeader.Size,
		fileName: fileHeader.Filename,
		mimeType: mimeType,
		create:   &create,
	}, nil
}
