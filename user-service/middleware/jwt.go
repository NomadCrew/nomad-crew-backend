package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

// EnsureValidToken verifies the Firebase ID token
func EnsureValidToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the token from the Authorization header
		idToken := extractBearerToken(r.Header.Get("Authorization"))
		if idToken == "" {
			http.Error(w, "Missing or malformed JWT", http.StatusUnauthorized)
			return
		}

		// Initialize Firebase SDK with service account key
		opt := option.WithCredentialsFile("/secrets/serviceAccountKey.json")
		app, err := firebase.NewApp(context.Background(), nil, opt)
		if err != nil {
			http.Error(w, fmt.Sprintf("error initializing app: %v", err), http.StatusInternalServerError)
			return
		}

		// Authenticate the token with Firebase
		client, err := app.Auth(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting Auth client: %v", err), http.StatusInternalServerError)
			return
		}

		token, err := client.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("error verifying ID token: %v", err), http.StatusUnauthorized)
			return
		}

		// Pass the user's Firebase UID to the next middleware or handler
		type contextKey string
		const uidKey contextKey = "uid"
		ctx := context.WithValue(r.Context(), uidKey, token.UID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, "Bearer ")
	if len(parts) != 2 {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
