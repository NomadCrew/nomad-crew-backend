package types

// UserResponse represents the public user data returned in API responses
type UserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// CreateUserResponse converts a User to a UserResponse
func CreateUserResponse(user *User) UserResponse {
	return UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}
}
