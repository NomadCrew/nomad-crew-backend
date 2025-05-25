package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
	"github.com/NomadCrew/nomad-crew-backend/types"
)

func main() {
	// Load configuration to get Pexels API key
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.ExternalServices.PexelsAPIKey == "" {
		log.Fatalf("PEXELS_API_KEY environment variable is required for testing")
	}

	// Initialize Pexels client
	pexelsClient := pexels.NewClient(cfg.ExternalServices.PexelsAPIKey)
	fmt.Printf("✅ Pexels client initialized successfully\n")

	// Test scenarios
	testScenarios := []struct {
		name string
		trip *types.Trip
	}{
		{
			name: "Trip with DestinationName",
			trip: &types.Trip{
				Name:                 "Summer Vacation",
				Description:          "A wonderful trip",
				DestinationName:      stringPtr("Paris, France"),
				DestinationLatitude:  48.8566,
				DestinationLongitude: 2.3522,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(7 * 24 * time.Hour),
			},
		},
		{
			name: "Trip with DestinationAddress only",
			trip: &types.Trip{
				Name:                 "Business Trip",
				Description:          "Work conference",
				DestinationAddress:   stringPtr("Tokyo, Japan"),
				DestinationLatitude:  35.6762,
				DestinationLongitude: 139.6503,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(3 * 24 * time.Hour),
			},
		},
		{
			name: "Trip with only Name containing location",
			trip: &types.Trip{
				Name:                 "London Adventure",
				Description:          "Exploring the city",
				DestinationLatitude:  51.5074,
				DestinationLongitude: -0.1278,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(5 * 24 * time.Hour),
			},
		},
		{
			name: "Trip with no location info",
			trip: &types.Trip{
				Name:                 "Mystery Trip",
				Description:          "Secret destination",
				DestinationLatitude:  0,
				DestinationLongitude: 0,
				StartDate:            time.Now().Add(24 * time.Hour),
				EndDate:              time.Now().Add(2 * 24 * time.Hour),
			},
		},
	}

	fmt.Println("\n🧪 Testing Pexels Background Image Integration")
	fmt.Println(strings.Repeat("=", 50))

	for i, scenario := range testScenarios {
		fmt.Printf("\n%d. %s\n", i+1, scenario.name)
		fmt.Printf("   Trip: %s\n", scenario.trip.Name)

		// Test buildPexelsSearchQuery
		searchQuery := pexels.BuildSearchQuery(scenario.trip)
		fmt.Printf("   Search Query: '%s'\n", searchQuery)

		if searchQuery == "" {
			fmt.Printf("   ⚠️  No search query generated\n")
			continue
		}

		// Test actual Pexels API call with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		imageURL, err := pexelsClient.SearchDestinationImage(ctx, searchQuery)
		cancel()

		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		if imageURL == "" {
			fmt.Printf("   ⚠️  No image found\n")
		} else {
			fmt.Printf("   ✅ Image URL: %s\n", imageURL)
		}
	}

	fmt.Println("\n🎯 Testing Complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Create a trip via the API without backgroundImageUrl")
	fmt.Println("2. Check that the trip gets created with a Pexels image URL")
	fmt.Println("3. Verify the image URL is valid and displays correctly")
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
