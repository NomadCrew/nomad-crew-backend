package command

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/interfaces"
	"github.com/NomadCrew/nomad-crew-backend/models/trip/validation"
	"github.com/NomadCrew/nomad-crew-backend/pkg/pexels"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

type CreateTripCommand struct {
	BaseCommand
	Trip *types.Trip
}

func (c *CreateTripCommand) Validate(ctx context.Context) error {
	if c.Trip == nil {
		return errors.ValidationFailed("trip_required", "Trip data is required")
	}
	if c.Trip.CreatedBy == "" {
		return errors.ValidationFailed("creator_required", "trip creator ID is required")
	}
	return validation.ValidateNewTrip(c.Trip)
}

func (c *CreateTripCommand) ValidatePermissions(ctx context.Context) error {
	// No existing trip to validate against
	// Creation requires valid user session only
	return nil
}

func (c *CreateTripCommand) Execute(ctx context.Context) (*interfaces.CommandResult, error) {
	// Log the start of the trip creation process.
	logger.GetLogger().Debugw("Executing CreateTripCommand", "trip", c.Trip)

	if err := c.Validate(ctx); err != nil {
		logger.GetLogger().Errorw("Trip validation failed", "error", err)
		return nil, err
	}

	// Set system-managed fields
	c.Trip.CreatedAt = time.Now()
	c.Trip.UpdatedAt = time.Now()
	logger.GetLogger().Debugw("Set system-managed fields", "createdAt", c.Trip.CreatedAt, "updatedAt", c.Trip.UpdatedAt)

	// Populate background image using Pexels API based on the destination address.
	// Log that we are about to search for a background image.
	logger.GetLogger().Debugw("Searching for background image", "destinationAddress", c.Trip.Destination.Address)
	pexelsClient := pexels.NewClient(os.Getenv("PEXELS_API_KEY"))
	imageURL, err := pexelsClient.SearchDestinationImage(c.Trip.Destination.Address)
	if err != nil {
		logger.GetLogger().Warnw("Failed to fetch background image", "error", err)
		// Continue without image - don't fail the trip creation
	} else if imageURL == "" {
		logger.GetLogger().Debug("No background image returned from Pexels", "destinationAddress", c.Trip.Destination.Address)
	} else {
		logger.GetLogger().Debugw("Background image found", "imageURL", imageURL)
		c.Trip.BackgroundImageURL = imageURL
	}

	createdID, err := c.Ctx.Store.CreateTrip(ctx, *c.Trip)
	if err != nil {
		logger.GetLogger().Errorw("Failed to create trip", "error", err)
		return nil, errors.NewDatabaseError(err)
	}
	logger.GetLogger().Debugw("Trip created in DB", "tripID", createdID)

	// Fetch the full trip record with database-generated fields
	createdTrip, err := c.Ctx.Store.GetTrip(ctx, createdID)
	if err != nil {
		logger.GetLogger().Errorw("Failed to fetch created trip", "tripID", createdID, "error", err)
		return nil, errors.NewDatabaseError(err)
	}
	c.Trip = createdTrip
	logger.GetLogger().Debugw("Fetched full trip data", "trip", c.Trip)

	// Create a default chat group for the trip
	if c.Ctx.ChatStore != nil {
		chatGroup := types.ChatGroup{
			TripID:      c.Trip.ID,
			Name:        c.Trip.Name + " Chat",
			Description: "Default chat group for " + c.Trip.Name,
			CreatedBy:   c.Trip.CreatedBy,
		}

		chatGroupID, err := c.Ctx.ChatStore.CreateChatGroup(ctx, chatGroup)
		if err != nil {
			logger.GetLogger().Errorw("Failed to create default chat group for trip", "error", err, "tripID", c.Trip.ID)
			// Don't fail the trip creation if chat group creation fails
		} else {
			logger.GetLogger().Infow("Created default chat group for trip", "chatGroupID", chatGroupID, "tripID", c.Trip.ID)

			// Add trip members to the chat group
			// First, add the trip creator
			err = c.Ctx.ChatStore.AddChatGroupMember(ctx, chatGroupID, c.Trip.CreatedBy)
			if err != nil {
				logger.GetLogger().Warnw("Failed to add trip creator to chat group", "error", err, "chatGroupID", chatGroupID, "userID", c.Trip.CreatedBy)
			}

			// Get all trip members and add them to the chat group
			members, err := c.Ctx.Store.GetTripMembers(ctx, c.Trip.ID)
			if err != nil {
				logger.GetLogger().Warnw("Failed to get trip members", "error", err, "tripID", c.Trip.ID)
			} else {
				for _, member := range members {
					// Skip the creator as they've already been added
					if member.UserID == c.Trip.CreatedBy {
						continue
					}

					err = c.Ctx.ChatStore.AddChatGroupMember(ctx, chatGroupID, member.UserID)
					if err != nil {
						logger.GetLogger().Warnw("Failed to add trip member to chat group", "error", err, "chatGroupID", chatGroupID, "userID", member.UserID)
					}
				}
			}
		}
	} else {
		logger.GetLogger().Warnw("ChatStore not available, skipping default chat group creation")
	}

	payload, _ := json.Marshal(c.Trip)

	logger.GetLogger().Infow("Trip creation succeeded", "tripID", c.Trip.ID)
	return &interfaces.CommandResult{
		Success: true,
		Data:    c.Trip,
		Events: []types.Event{{
			BaseEvent: types.BaseEvent{
				ID:        uuid.NewString(),
				Type:      types.EventTypeTripCreated,
				TripID:    c.Trip.ID,
				UserID:    c.UserID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Payload: payload,
		}},
	}, nil
}
