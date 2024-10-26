package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/NomadCrew/nomad-crew-backend/user-service/models"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Server struct {
	DB *pgxpool.Pool
}

// RegisterHandler godoc
// @Summary Register user
// @Description Register a new user with username and email
// @Tags users
// @Accept json
// @Produce json
// @Param user body User true "Create user"
// @Success 200 {object} User
// @Router /register [post]
func (s *Server) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var u models.User
	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := u.SaveUser(ctx, s.DB); err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

// LoginHandler godoc
// @Summary Login user
// @Description Login a user with email and password
// @Tags users
// @Accept  json
// @Produce  json
// @Param credentials body Credentials true "User credentials"
// @Success 200 {object} User
// @Router /login [post]
func (s *Server) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Implement the functinality to authenticate and assing a custom JWT token
	// if you decide to move away from firebase auth on client-side. Otherwise, this is just a placeholder.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("Logged in successfully")
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResp, _ := json.Marshal(map[string]string{"status": "ok"})
	_, _ = w.Write(jsonResp)
}

func (s *Server) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("userInfo")
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user := claims.(map[string]interface{})
	email := user["email"].(string)

	ctx := r.Context()
	u, err := models.GetUserByEmail(ctx, s.DB, email)
	if err != nil {
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(u)
}

func (s *Server) GetNearbyPlacesHandler(w http.ResponseWriter, r *http.Request) {
	lat := r.URL.Query().Get("lat")
	lon := r.URL.Query().Get("lon")

	if lat == "" || lon == "" {
		http.Error(w, "Missing latitude or longitude", http.StatusBadRequest)
		return
	}

	GEOAPIFY_KEY := os.Getenv("GEOAPIFY_KEY")
	if GEOAPIFY_KEY == "" {
		http.Error(w, "GEOAPIFY_KEY environment variable not set", http.StatusInternalServerError)
		return
	}
	baseURL := "https://api.geoapify.com/v2/places"
	categories := "activity,natural,leisure"
	filter := "circle:" + lon + "," + lat + ",15000"
	limit := 20

	url := fmt.Sprintf("%s?categories=%s&filter=%s&limit=%d&apiKey=%s", baseURL, categories, filter, limit, GEOAPIFY_KEY)

	// Create the HTTP request
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	// Send the request to Geoapify
	res, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	// Read the response
	body, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the JSON response
	var geoapifyResponse struct {
		Features []struct {
			Properties struct {
				PlaceID   string `json:"place_id"`
				Name      string `json:"name"`
				Formatted string `json:"formatted"`
				Website   string `json:"website"`
				Contact   struct {
					Phone string `json:"phone"`
				} `json:"contact"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &geoapifyResponse); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the default image from Pexels
	defaultImage, err := fetchDefaultImage()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare the response
	nearbyPlaces := []map[string]interface{}{}
	for _, feature := range geoapifyResponse.Features {
		properties := feature.Properties
		nearbyPlace := map[string]interface{}{
			"id":      properties.PlaceID,
			"name":    properties.Name,
			"address": properties.Formatted,
			"website": properties.Website,
			"phone":   properties.Contact.Phone,
			"image":   defaultImage,
		}
		nearbyPlaces = append(nearbyPlaces, nearbyPlace)
	}

	// Write the JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nearbyPlaces)
}

func fetchDefaultImage() (string, error) {
	pexelsAPIKey := os.Getenv("PEXELS_API_KEY")
	if pexelsAPIKey == "" {
		return "", fmt.Errorf("PEXELS_API_KEY is not set")
	}

	url := "https://api.pexels.com/v1/search?query=park&per_page=1"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", pexelsAPIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var pexelsResponse struct {
		Photos []struct {
			Src struct {
				Small string `json:"small"`
			} `json:"src"`
		} `json:"photos"`
	}
	err = json.Unmarshal(body, &pexelsResponse)
	if err != nil {
		return "", err
	}

	if len(pexelsResponse.Photos) > 0 {
		return pexelsResponse.Photos[0].Src.Small, nil
	}

	return "", fmt.Errorf("no photos found")
}
