package handlers_test 

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/NomadCrew/nomad-crew-backend/middleware"
    "github.com/NomadCrew/nomad-crew-backend/handlers"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/NomadCrew/nomad-crew-backend/errors"
)

type CreateUserRequest struct {
    Username       string `json:"username"`
    Email          string `json:"email"`
    Password       string `json:"password"`
    FirstName      string `json:"first_name,omitempty"`
    LastName       string `json:"last_name,omitempty"`
    ProfilePicture string `json:"profile_picture,omitempty"`
    PhoneNumber    string `json:"phone_number,omitempty"`
    Address        string `json:"address,omitempty"`
}

type MockUserModel struct {
    mock.Mock
}

func (m *MockUserModel) CreateUser(ctx context.Context, user *types.User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func (m *MockUserModel) GetUserByID(ctx context.Context, id int64) (*types.User, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*types.User), args.Error(1)
}

func (m *MockUserModel) UpdateUser(ctx context.Context, user *types.User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func (m *MockUserModel) DeleteUser(ctx context.Context, id int64) error {
    args := m.Called(ctx, id)
    return args.Error(0)
}

func (m *MockUserModel) AuthenticateUser(ctx context.Context, email, password string) (*types.User, error) {
    args := m.Called(ctx, email, password)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*types.User), args.Error(1)
}

func setupTestRouter() (*gin.Engine, *MockUserModel) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(middleware.ErrorHandler())

    mockModel := new(MockUserModel)
    handler := handlers.NewUserHandler(mockModel)

    r.POST("/users", handler.CreateUserHandler)
    r.GET("/users/:id", handler.GetUserHandler)
    r.PUT("/users/:id", handler.UpdateUserHandler)
    r.DELETE("/users/:id", handler.DeleteUserHandler)
    r.POST("/login", handler.LoginHandler)

    return r, mockModel
}

func TestCreateUserHandler(t *testing.T) {
	router, mockModel := setupTestRouter()

	// Create a new handler to access the setter
	handler := handlers.NewUserHandler(mockModel)

	// Replace the default generateJWT function with a mock
	mockGenerateJWT := func(user *types.User) (string, error) {
		return "mocked-token", nil
	}
	handler.SetGenerateJWTFunc(mockGenerateJWT)

	// Re-assign handler routes to use the updated handler
	router.POST("/users", handler.CreateUserHandler)

	tests := []struct {
		name           string
		payload        CreateUserRequest
		setupMock      func(*MockUserModel)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Success",
			payload: CreateUserRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(m *MockUserModel) {
				m.On("CreateUser", mock.Anything, mock.MatchedBy(func(user *types.User) bool {
					return user.Username == "testuser" && user.Email == "test@example.com"
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectedBody: `{
				"user": {
					"id": 0,
					"username": "testuser",
					"email": "test@example.com"
				},
				"token": "mocked-token"
			}`,
		},
		{
			name: "Invalid Email",
			payload: CreateUserRequest{
				Username: "testuser",
				Email:    "invalid-email",
				Password: "password123",
			},
			setupMock:      func(m *MockUserModel) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockModel)

			payloadBytes, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, w.Body.String())
			}
			mockModel.AssertExpectations(t)
		})
	}
}

func TestGetUserHandler(t *testing.T) {
    router, mockModel := setupTestRouter()

    tests := []struct {
        name           string
        userID         string
        setupMock      func(*MockUserModel)
        expectedStatus int
        expectedBody   string
    }{
        {
            name:   "Success",
            userID: "1",
            setupMock: func(m *MockUserModel) {
                m.On("GetUserByID", mock.Anything, int64(1)).Return(&types.User{
                    ID:       1,
                    Username: "testuser",
                    Email:    "test@example.com",
                }, nil)
            },
            expectedStatus: http.StatusOK,
            expectedBody:   `{"id":1,"username":"testuser","email":"test@example.com"}`,
        },
        {
            name:   "User Not Found",
            userID: "999",
            setupMock: func(m *MockUserModel) {
                m.On("GetUserByID", mock.Anything, int64(999)).Return(nil, 
                    errors.NotFound("User", 999))
            },
            expectedStatus: http.StatusNotFound,
            expectedBody:   `{"type":"NOT_FOUND","message":"User not found","detail":"ID: 999"}`,
        },
        {
            name:   "Invalid ID Format",
            userID: "invalid",
            setupMock: func(m *MockUserModel) {},
            expectedStatus: http.StatusBadRequest,
            expectedBody:   `{"type":"VALIDATION_ERROR","message":"Invalid user ID","detail":"Invalid input provided"}`,
        },        
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setupMock(mockModel)

            req, _ := http.NewRequest(http.MethodGet, "/users/"+tt.userID, nil)
            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)
            if tt.expectedBody != "" {
                assert.JSONEq(t, tt.expectedBody, w.Body.String())
            }
            mockModel.AssertExpectations(t)
        })
    }
}