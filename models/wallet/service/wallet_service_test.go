package service_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	wallet "github.com/NomadCrew/nomad-crew-backend/models/wallet/service"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// MockWalletStore implements store.WalletStore
type MockWalletStore struct{ mock.Mock }

func (m *MockWalletStore) CreateDocument(ctx context.Context, doc *types.WalletDocument) (string, error) {
	args := m.Called(ctx, doc)
	return args.String(0), args.Error(1)
}
func (m *MockWalletStore) GetDocument(ctx context.Context, id string) (*types.WalletDocument, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WalletDocument), args.Error(1)
}
func (m *MockWalletStore) ListPersonalDocuments(ctx context.Context, userID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.WalletDocument), args.Int(1), args.Error(2)
}
func (m *MockWalletStore) ListGroupDocuments(ctx context.Context, tripID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	args := m.Called(ctx, tripID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*types.WalletDocument), args.Int(1), args.Error(2)
}
func (m *MockWalletStore) UpdateDocument(ctx context.Context, id string, update *types.WalletDocumentUpdate) (*types.WalletDocument, error) {
	args := m.Called(ctx, id, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WalletDocument), args.Error(1)
}
func (m *MockWalletStore) SoftDeleteDocument(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockWalletStore) GetDocumentByFilePath(ctx context.Context, filePath string) (*types.WalletDocument, error) {
	args := m.Called(ctx, filePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.WalletDocument), args.Error(1)
}
func (m *MockWalletStore) GetUserStorageUsage(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockWalletStore) GetTripStorageUsage(ctx context.Context, tripID string) (int64, error) {
	args := m.Called(ctx, tripID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockWalletStore) PurgeDeletedDocuments(ctx context.Context, olderThan time.Time) ([]string, error) {
	args := m.Called(ctx, olderThan)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockWalletStore) HardDeleteAllByUser(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// MockTripStore implements store.TripStore (only methods used by WalletService are meaningful)
type MockTripStore struct{ mock.Mock }

func (m *MockTripStore) GetPool() *pgxpool.Pool                               { return nil }
func (m *MockTripStore) CreateTrip(_ context.Context, _ types.Trip) (string, error) {
	return "", nil
}
func (m *MockTripStore) GetTrip(_ context.Context, _ string) (*types.Trip, error) { return nil, nil }
func (m *MockTripStore) UpdateTrip(_ context.Context, _ string, _ types.TripUpdate) (*types.Trip, error) {
	return nil, nil
}
func (m *MockTripStore) SoftDeleteTrip(_ context.Context, _ string) error { return nil }
func (m *MockTripStore) ListUserTrips(_ context.Context, _ string) ([]*types.Trip, error) {
	return nil, nil
}
func (m *MockTripStore) SearchTrips(_ context.Context, _ types.TripSearchCriteria) ([]*types.Trip, error) {
	return nil, nil
}
func (m *MockTripStore) AddMember(_ context.Context, _ *types.TripMembership) error { return nil }
func (m *MockTripStore) UpdateMemberRole(_ context.Context, _, _ string, _ types.MemberRole) error {
	return nil
}
func (m *MockTripStore) RemoveMember(_ context.Context, _, _ string) error { return nil }
func (m *MockTripStore) GetTripMembers(_ context.Context, _ string) ([]types.TripMembership, error) {
	return nil, nil
}
func (m *MockTripStore) GetUserRole(ctx context.Context, tripID string, userID string) (types.MemberRole, error) {
	args := m.Called(ctx, tripID, userID)
	return args.Get(0).(types.MemberRole), args.Error(1)
}
func (m *MockTripStore) LookupUserByEmail(_ context.Context, _ string) (*types.SupabaseUser, error) {
	return nil, nil
}
func (m *MockTripStore) CreateInvitation(_ context.Context, _ *types.TripInvitation) error {
	return nil
}
func (m *MockTripStore) GetInvitation(_ context.Context, _ string) (*types.TripInvitation, error) {
	return nil, nil
}
func (m *MockTripStore) GetInvitationsByTripID(_ context.Context, _ string) ([]*types.TripInvitation, error) {
	return nil, nil
}
func (m *MockTripStore) UpdateInvitationStatus(_ context.Context, _ string, _ types.InvitationStatus) error {
	return nil
}
func (m *MockTripStore) AcceptInvitationAtomically(_ context.Context, _ string, _ *types.TripMembership) error {
	return nil
}
func (m *MockTripStore) RemoveMemberWithOwnerLock(_ context.Context, _, _ string) error { return nil }
func (m *MockTripStore) UpdateMemberRoleWithOwnerLock(_ context.Context, _, _ string, _ types.MemberRole) error {
	return nil
}
func (m *MockTripStore) BeginTx(_ context.Context) (types.DatabaseTransaction, error) {
	return nil, nil
}
func (m *MockTripStore) Commit() error   { return nil }
func (m *MockTripStore) Rollback() error { return nil }

// MockFileStorage implements service.FileStorage
type MockFileStorage struct{ mock.Mock }

func (m *MockFileStorage) Save(ctx context.Context, path string, reader io.Reader, size int64) error {
	// Drain the reader so countingReader actually counts bytes
	buf, _ := io.ReadAll(reader)
	args := m.Called(ctx, path, buf, size)
	return args.Error(0)
}
func (m *MockFileStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}
func (m *MockFileStorage) GetPath(ctx context.Context, path string) string {
	args := m.Called(ctx, path)
	return args.String(0)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testSigningKey = "test-secret-key-32-bytes-long!!!"

func newTestService(ws *MockWalletStore, ts *MockTripStore, fs *MockFileStorage) *wallet.WalletService {
	return wallet.NewWalletService(ws, ts, fs, testSigningKey)
}

// minimalPNG is a valid 1x1 transparent PNG (67 bytes).
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, // IDAT chunk
	0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02,
	0x00, 0x01, 0xe5, 0x27, 0xde, 0xfc, 0x00, 0x00,
	0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, // IEND chunk
	0x60, 0x82,
}

// minimalJPEG is a minimal JPEG (just the SOI + EOI markers + JFIF header)
var minimalJPEG = func() []byte {
	// A minimal valid JPEG with JFIF APP0 marker
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}
}()

// minimalPDF is the PDF magic bytes followed by minimal content
var minimalPDF = []byte("%PDF-1.4 minimal")

func strPtr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// sanitizeFilename tests (exported via UploadDocument behavior)
// ---------------------------------------------------------------------------

func TestSanitizeFilename_ViaUpload(t *testing.T) {
	// We test sanitizeFilename indirectly through UploadDocument, checking the
	// storage path passed to FileStorage.Save.

	tests := []struct {
		name             string
		inputFilename    string
		expectContains   string // substring the sanitized name should contain
		expectNotContain string // substring that must NOT appear
	}{
		{
			name:             "pipe chars stripped",
			inputFilename:    "my|evil|file.png",
			expectContains:   "my_evil_file.png",
			expectNotContain: "|",
		},
		{
			name:           "null bytes stripped",
			inputFilename:  "file\x00name.png",
			expectContains: "file_name.png",
		},
		{
			name:           "path traversal stripped",
			inputFilename:  "../../../etc/passwd",
			expectContains: "_", // filepath.Base + regex replaces slashes
		},
		{
			name:           "unicode replaced with underscore",
			inputFilename:  "döcument-ñ.png",
			expectContains: "d_cument-_.png",
		},
		{
			name:           "spaces replaced",
			inputFilename:  "my document.png",
			expectContains: "my_document.png",
		},
		{
			name:           "normal filename preserved",
			inputFilename:  "passport-scan.png",
			expectContains: "passport-scan.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := new(MockWalletStore)
			ts := new(MockTripStore)
			fs := new(MockFileStorage)
			svc := newTestService(ws, ts, fs)

			// Expect Save to be called — capture the path
			var capturedPath string
			fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					capturedPath = args.String(1)
				}).Return(nil)
			ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
			ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

			ctx := context.Background()
			create := &types.WalletDocumentCreate{
				WalletType:   types.WalletTypePersonal,
				DocumentType: types.DocumentTypePassport,
				Name:         "Test",
			}

			_, _ = svc.UploadDocument(ctx, "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, tt.inputFilename, "")

			if tt.expectContains != "" {
				assert.Contains(t, capturedPath, tt.expectContains, "sanitized path should contain expected substring")
			}
			if tt.expectNotContain != "" {
				assert.NotContains(t, capturedPath, tt.expectNotContain, "sanitized path must not contain forbidden chars")
			}
		})
	}
}

func TestSanitizeFilename_DegenerateInputs(t *testing.T) {
	// Degenerate filenames should produce a safe fallback
	degenerateNames := []string{"", ".", "..", "...", "///"}

	for _, name := range degenerateNames {
		t.Run(fmt.Sprintf("input=%q", name), func(t *testing.T) {
			ws := new(MockWalletStore)
			ts := new(MockTripStore)
			fs := new(MockFileStorage)
			svc := newTestService(ws, ts, fs)

			var capturedPath string
			fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					capturedPath = args.String(1)
				}).Return(nil)
			ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
			ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

			ctx := context.Background()
			create := &types.WalletDocumentCreate{
				WalletType:   types.WalletTypePersonal,
				DocumentType: types.DocumentTypeOther,
				Name:         "Test",
			}

			_, err := svc.UploadDocument(ctx, "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, name, "")
			// The call may fail on MIME or succeed — either way the path must not be empty/dot
			if err == nil {
				assert.NotEmpty(t, capturedPath)
				// Extract the filename part (after the last /)
				parts := strings.Split(capturedPath, "/")
				filename := parts[len(parts)-1]
				// Strip the timestamp prefix (digits_)
				if idx := strings.Index(filename, "_"); idx >= 0 {
					filename = filename[idx+1:]
				}
				assert.NotEqual(t, "", filename)
				assert.NotEqual(t, ".", filename)
				assert.NotEqual(t, "..", filename)
			}
		})
	}
}

func TestSanitizeFilename_LongNamePreservesExtension(t *testing.T) {
	ws := new(MockWalletStore)
	ts := new(MockTripStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, ts, fs)

	var capturedPath string
	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedPath = args.String(1)
		}).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	// Create a filename longer than 255 chars with a .png extension
	longName := strings.Repeat("a", 260) + ".png"

	ctx := context.Background()
	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "Test",
	}

	_, err := svc.UploadDocument(ctx, "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, longName, "")
	require.NoError(t, err)

	// Extract the sanitized filename
	parts := strings.Split(capturedPath, "/")
	filename := parts[len(parts)-1]
	if idx := strings.Index(filename, "_"); idx >= 0 {
		filename = filename[idx+1:]
	}

	assert.LessOrEqual(t, len(filename), 255, "sanitized filename must be <= 255 chars")
	assert.True(t, strings.HasSuffix(filename, ".png"), "extension must be preserved after truncation")
}

// ---------------------------------------------------------------------------
// HMAC signed URL tests
// ---------------------------------------------------------------------------

func TestGenerateAndValidateSignedURL_RoundTrip(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	token := svc.GenerateSignedURL("personal/user-1/123_doc.png", 1*time.Hour)
	path, err := svc.ValidateSignedURL(token)

	require.NoError(t, err)
	assert.Equal(t, "personal/user-1/123_doc.png", path)
}

func TestValidateSignedURL_ExpiredToken(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	// Generate with negative duration to create an already-expired token
	token := svc.GenerateSignedURL("some/path.pdf", -1*time.Hour)
	_, err := svc.ValidateSignedURL(token)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateSignedURL_TamperedSignature(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	token := svc.GenerateSignedURL("some/path.pdf", 1*time.Hour)

	// Decode, tamper with first char of signature, re-encode
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)

	rawStr := string(raw)
	if rawStr[0] == 'a' {
		rawStr = "b" + rawStr[1:]
	} else {
		rawStr = "a" + rawStr[1:]
	}
	tampered := base64.RawURLEncoding.EncodeToString([]byte(rawStr))

	_, err = svc.ValidateSignedURL(tampered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestValidateSignedURL_TamperedPath(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	token := svc.GenerateSignedURL("some/path.pdf", 1*time.Hour)

	// Decode, change the path segment, re-encode
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)

	parts := strings.SplitN(string(raw), "|", 3)
	require.Len(t, parts, 3)
	tampered := base64.RawURLEncoding.EncodeToString([]byte(parts[0] + "|evil/path.pdf|" + parts[2]))

	_, err = svc.ValidateSignedURL(tampered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestValidateSignedURL_MalformedBase64(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	_, err := svc.ValidateSignedURL("!!!not-base64!!!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed")
}

func TestValidateSignedURL_MalformedPayload(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	// Valid base64 but wrong structure (no pipe delimiters)
	token := base64.RawURLEncoding.EncodeToString([]byte("no-pipes-here"))
	_, err := svc.ValidateSignedURL(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed")
}

func TestValidateSignedURL_DifferentSigningKey(t *testing.T) {
	svc1 := wallet.NewWalletService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage), "key-one-32-bytes-long-pad!!!!!!!")
	svc2 := wallet.NewWalletService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage), "key-two-32-bytes-long-pad!!!!!!!")

	token := svc1.GenerateSignedURL("path.pdf", 1*time.Hour)
	_, err := svc2.ValidateSignedURL(token)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

// ---------------------------------------------------------------------------
// MIME detection tests
// ---------------------------------------------------------------------------

func TestUploadDocument_AcceptsPNG(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         "test.png",
	}

	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "test.png", "")
	require.NoError(t, err)
	assert.Equal(t, "image/png", resp.MimeType)
	assert.Equal(t, int64(len(minimalPNG)), resp.FileSize)
}

func TestUploadDocument_AcceptsJPEG(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         "photo.jpg",
	}

	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalJPEG), int64(len(minimalJPEG)), create, "photo.jpg", "")
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", resp.MimeType)
}

func TestUploadDocument_AcceptsPDF(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "doc.pdf",
	}

	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPDF), int64(len(minimalPDF)), create, "doc.pdf", "")
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", resp.MimeType)
}

func TestUploadDocument_RejectsExecutable(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	// ELF binary header
	elfHeader := []byte{0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01, 0x01, 0x00}

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "evil",
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(elfHeader), int64(len(elfHeader)), create, "evil.bin", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MIME type")
}

func TestUploadDocument_RejectsHTML(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	htmlContent := []byte("<html><body><script>alert('xss')</script></body></html>")

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "page.html",
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(htmlContent), int64(len(htmlContent)), create, "page.html", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MIME type")
}

func TestUploadDocument_RejectsZeroBytesFile(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "empty",
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(nil), 0, create, "empty.txt", "")
	require.Error(t, err)
	// Zero bytes will be detected as application/octet-stream or similar — rejected by allowedMimeTypes
	assert.Contains(t, err.Error(), "MIME type")
}

// ---------------------------------------------------------------------------
// Upload: wallet type constraint tests
// ---------------------------------------------------------------------------

func TestUploadDocument_GroupWithoutTripIDFails(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypeGroup,
		DocumentType: types.DocumentTypeOther,
		Name:         "group-doc",
		TripID:       nil,
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "doc.png", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trip ID is required")
}

func TestUploadDocument_GroupWithEmptyTripIDFails(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	empty := ""
	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypeGroup,
		DocumentType: types.DocumentTypeOther,
		Name:         "group-doc",
		TripID:       &empty,
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "doc.png", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trip ID is required")
}

func TestUploadDocument_PersonalWithTripIDClearsTripID(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var capturedDoc *types.WalletDocument
	ws.On("CreateDocument", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedDoc = args.Get(1).(*types.WalletDocument)
		}).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         "passport",
		TripID:       strPtr("trip-123"),
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "pass.png", "")
	require.NoError(t, err)
	assert.Nil(t, capturedDoc.TripID, "personal document should have nil TripID")
}

func TestUploadDocument_NilMetadataBecomesEmptyMap(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var capturedDoc *types.WalletDocument
	ws.On("CreateDocument", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedDoc = args.Get(1).(*types.WalletDocument)
		}).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "test",
		Metadata:     nil,
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "test.png", "")
	require.NoError(t, err)
	assert.NotNil(t, capturedDoc.Metadata, "nil metadata should become empty map")
	assert.Empty(t, capturedDoc.Metadata)
}

// ---------------------------------------------------------------------------
// Upload: failure cleanup tests
// ---------------------------------------------------------------------------

func TestUploadDocument_DBFailureCleansUpFile(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("", errors.New("db connection lost"))
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)
	fs.On("Delete", mock.Anything, mock.Anything).Return(nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "test",
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "test.png", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create document record")

	// Verify Delete was called for cleanup
	fs.AssertCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestUploadDocument_SaveFailureCleansUp(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("disk full"))
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)
	fs.On("Delete", mock.Anything, mock.Anything).Return(nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "test",
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "test.png", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save file")

	// Verify Delete was called for partial file cleanup
	fs.AssertCalled(t, "Delete", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// CountingReader: actual bytes recorded
// ---------------------------------------------------------------------------

func TestUploadDocument_RecordsActualBytesWritten(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var capturedDoc *types.WalletDocument
	ws.On("CreateDocument", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedDoc = args.Get(1).(*types.WalletDocument)
		}).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	pngData := minimalPNG
	// Pass a deliberately wrong client-reported size
	clientReportedSize := int64(999999)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         "test",
	}

	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(pngData), clientReportedSize, create, "test.png", "")
	require.NoError(t, err)

	// The DB record should have actual bytes, not client-reported
	assert.Equal(t, int64(len(pngData)), capturedDoc.FileSize, "DB record should have actual byte count")
	assert.Equal(t, int64(len(pngData)), resp.FileSize, "response should have actual byte count")
	assert.NotEqual(t, clientReportedSize, capturedDoc.FileSize, "should not trust client-reported size")
}

// ---------------------------------------------------------------------------
// Authorization: GetDocument
// ---------------------------------------------------------------------------

func TestGetDocument_PersonalOwnerAccess(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	doc := &types.WalletDocument{
		ID:         "doc-1",
		UserID:     "user-1",
		WalletType: types.WalletTypePersonal,
		FilePath:   "personal/user-1/123_doc.png",
	}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)

	resp, err := svc.GetDocument(context.Background(), "doc-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "doc-1", resp.ID)
	assert.NotEmpty(t, resp.DownloadURL)
}

func TestGetDocument_PersonalNonOwnerForbidden(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	doc := &types.WalletDocument{
		ID:         "doc-1",
		UserID:     "user-1",
		WalletType: types.WalletTypePersonal,
	}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)

	_, err := svc.GetDocument(context.Background(), "doc-1", "other-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestGetDocument_GroupTripMemberAccess(t *testing.T) {
	ws := new(MockWalletStore)
	ts := new(MockTripStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, ts, fs)

	tripID := "trip-1"
	doc := &types.WalletDocument{
		ID:         "doc-1",
		UserID:     "user-1",
		TripID:     &tripID,
		WalletType: types.WalletTypeGroup,
		FilePath:   "group/user-1/123_doc.png",
	}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)
	ts.On("GetUserRole", mock.Anything, "trip-1", "user-2").Return(types.MemberRoleMember, nil)

	resp, err := svc.GetDocument(context.Background(), "doc-1", "user-2")
	require.NoError(t, err)
	assert.Equal(t, "doc-1", resp.ID)
}

func TestGetDocument_GroupNonMemberForbidden(t *testing.T) {
	ws := new(MockWalletStore)
	ts := new(MockTripStore)
	svc := newTestService(ws, ts, new(MockFileStorage))

	tripID := "trip-1"
	doc := &types.WalletDocument{
		ID:         "doc-1",
		UserID:     "user-1",
		TripID:     &tripID,
		WalletType: types.WalletTypeGroup,
	}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)
	ts.On("GetUserRole", mock.Anything, "trip-1", "outsider").Return(types.MemberRole(""), errors.New("not a member"))

	_, err := svc.GetDocument(context.Background(), "doc-1", "outsider")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trip member")
}

// ---------------------------------------------------------------------------
// Authorization: UpdateDocument
// ---------------------------------------------------------------------------

func TestUpdateDocument_OwnerCanUpdate(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	doc := &types.WalletDocument{ID: "doc-1", UserID: "user-1"}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)

	updatedDoc := &types.WalletDocument{ID: "doc-1", UserID: "user-1", Name: "Updated"}
	update := &types.WalletDocumentUpdate{Name: strPtr("Updated")}
	ws.On("UpdateDocument", mock.Anything, "doc-1", update).Return(updatedDoc, nil)

	result, err := svc.UpdateDocument(context.Background(), "doc-1", "user-1", update)
	require.NoError(t, err)
	assert.Equal(t, "Updated", result.Name)
}

func TestUpdateDocument_NonOwnerForbidden(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	doc := &types.WalletDocument{ID: "doc-1", UserID: "user-1"}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)

	_, err := svc.UpdateDocument(context.Background(), "doc-1", "other-user", &types.WalletDocumentUpdate{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

// ---------------------------------------------------------------------------
// DeleteDocument
// ---------------------------------------------------------------------------

func TestDeleteDocument_OwnerCanDelete(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	doc := &types.WalletDocument{ID: "doc-1", UserID: "user-1", FilePath: "personal/user-1/file.png"}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)
	ws.On("SoftDeleteDocument", mock.Anything, "doc-1").Return(nil)
	fs.On("Delete", mock.Anything, "personal/user-1/file.png").Return(nil)

	err := svc.DeleteDocument(context.Background(), "doc-1", "user-1")
	require.NoError(t, err)

	ws.AssertCalled(t, "SoftDeleteDocument", mock.Anything, "doc-1")
	fs.AssertCalled(t, "Delete", mock.Anything, "personal/user-1/file.png")
}

func TestDeleteDocument_NonOwnerForbidden(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	doc := &types.WalletDocument{ID: "doc-1", UserID: "user-1"}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)

	err := svc.DeleteDocument(context.Background(), "doc-1", "other-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestDeleteDocument_FileDeleteFailureIsBestEffort(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	doc := &types.WalletDocument{ID: "doc-1", UserID: "user-1", FilePath: "some/path.png"}
	ws.On("GetDocument", mock.Anything, "doc-1").Return(doc, nil)
	ws.On("SoftDeleteDocument", mock.Anything, "doc-1").Return(nil)
	fs.On("Delete", mock.Anything, mock.Anything).Return(errors.New("storage unreachable"))

	// Should succeed even though file deletion failed
	err := svc.DeleteDocument(context.Background(), "doc-1", "user-1")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// ServeFile
// ---------------------------------------------------------------------------

func TestServeFile_ValidToken(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	token := svc.GenerateSignedURL("personal/user-1/doc.png", 1*time.Hour)
	ws.On("GetDocumentByFilePath", mock.Anything, "personal/user-1/doc.png").
		Return(&types.WalletDocument{ID: "doc-1", MimeType: "image/png"}, nil)
	fs.On("GetPath", mock.Anything, "personal/user-1/doc.png").Return("/storage/personal/user-1/doc.png")

	path, mimeType, err := svc.ServeFile(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "/storage/personal/user-1/doc.png", path)
	assert.Equal(t, "image/png", mimeType)
}

func TestServeFile_InvalidToken(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	_, _, err := svc.ServeFile(context.Background(), "garbage-token")
	require.Error(t, err)
}

func TestServeFile_SoftDeletedDocument(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	token := svc.GenerateSignedURL("personal/user-1/doc.png", 1*time.Hour)
	ws.On("GetDocumentByFilePath", mock.Anything, "personal/user-1/doc.png").
		Return(nil, apperrors.NotFound("wallet_document", "personal/user-1/doc.png"))

	_, _, err := svc.ServeFile(context.Background(), token)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// ListDocuments passthrough
// ---------------------------------------------------------------------------

func TestListPersonalDocuments(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	docs := []*types.WalletDocument{{ID: "doc-1"}, {ID: "doc-2"}}
	ws.On("ListPersonalDocuments", mock.Anything, "user-1", 10, 0).Return(docs, 2, nil)

	result, total, err := svc.ListPersonalDocuments(context.Background(), "user-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
}

func TestListGroupDocuments(t *testing.T) {
	ws := new(MockWalletStore)
	svc := newTestService(ws, new(MockTripStore), new(MockFileStorage))

	docs := []*types.WalletDocument{{ID: "doc-1"}}
	ws.On("ListGroupDocuments", mock.Anything, "trip-1", 20, 5).Return(docs, 1, nil)

	result, total, err := svc.ListGroupDocuments(context.Background(), "trip-1", 20, 5)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, total)
}

// ---------------------------------------------------------------------------
// Upload: group with valid tripID succeeds
// ---------------------------------------------------------------------------

func TestUploadDocument_GroupWithTripIDSucceeds(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetTripStorageUsage", mock.Anything, "trip-123").Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypeGroup,
		DocumentType: types.DocumentTypeOther,
		Name:         "group-doc",
		TripID:       strPtr("trip-123"),
	}

	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "doc.png", "")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.DownloadURL)
}

// ---------------------------------------------------------------------------
// Upload: client-provided MIME type is ignored
// ---------------------------------------------------------------------------

func TestUploadDocument_IgnoresClientMIMEType(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         "test",
	}

	// Client says "text/plain" but content is PNG
	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalPNG), int64(len(minimalPNG)), create, "test.png", "text/plain")
	require.NoError(t, err)
	assert.Equal(t, "image/png", resp.MimeType, "server-side detection should override client MIME type")
}

// ---------------------------------------------------------------------------
// Edge case: short file (<512 bytes) still works
// ---------------------------------------------------------------------------

func TestUploadDocument_ShortFileAccepted(t *testing.T) {
	ws := new(MockWalletStore)
	fs := new(MockFileStorage)
	svc := newTestService(ws, new(MockTripStore), fs)

	fs.On("Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ws.On("CreateDocument", mock.Anything, mock.Anything).Return("doc-1", nil)
	ws.On("GetUserStorageUsage", mock.Anything, mock.Anything).Return(int64(0), nil)

	// minimalJPEG is only 22 bytes (well under 512)
	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypePassport,
		Name:         "tiny",
	}

	resp, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(minimalJPEG), int64(len(minimalJPEG)), create, "tiny.jpg", "")
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", resp.MimeType)
	assert.Equal(t, int64(len(minimalJPEG)), resp.FileSize)
}

// ---------------------------------------------------------------------------
// Regression: pipe in HMAC token format
// ---------------------------------------------------------------------------

func TestSignedURL_PathWithSpecialCharsRoundTrips(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	// Paths with slashes (normal) should round-trip fine
	paths := []string{
		"personal/user-1/123_doc.png",
		"group/user-1/456_passport-scan.pdf",
		"personal/user-abc-def/789_file.jpg",
	}

	for _, p := range paths {
		token := svc.GenerateSignedURL(p, 1*time.Hour)
		got, err := svc.ValidateSignedURL(token)
		require.NoError(t, err, "path: %s", p)
		assert.Equal(t, p, got)
	}
}

// ---------------------------------------------------------------------------
// apperrors type assertions (verify we return structured errors)
// ---------------------------------------------------------------------------

func TestUploadDocument_MIMERejectionReturnsValidationError(t *testing.T) {
	svc := newTestService(new(MockWalletStore), new(MockTripStore), new(MockFileStorage))

	// Send a ZIP file
	zipHeader := []byte{0x50, 0x4B, 0x03, 0x04, 0x0A, 0x00, 0x00, 0x00}
	create := &types.WalletDocumentCreate{
		WalletType:   types.WalletTypePersonal,
		DocumentType: types.DocumentTypeOther,
		Name:         "archive",
	}

	_, err := svc.UploadDocument(context.Background(), "user-1", bytes.NewReader(zipHeader), int64(len(zipHeader)), create, "archive.zip", "")
	require.Error(t, err)

	// Verify it's an AppError
	var appErr *apperrors.AppError
	assert.True(t, errors.As(err, &appErr), "should return an AppError for MIME validation failure")
}
