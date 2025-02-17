package interfaces

import (
	"sync"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/supabase-community/supabase-go"
)

type CommandContext struct {
	Store          store.TripStore
	EventBus       types.EventPublisher
	WeatherSvc     types.WeatherServiceInterface
	SupabaseClient *supabase.Client
	Config         *config.ServerConfig
	RequestData    *sync.Map
}

// Add these methods directly to the interfaces package
func (c *CommandContext) SetRequestData(key string, value interface{}) {
	c.RequestData.Store(key, value)
}

func (c *CommandContext) GetRequestData(key string) (interface{}, bool) {
	return c.RequestData.Load(key)
}
