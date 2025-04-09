# Trip Service Architecture

This directory contains the refactored Trip domain services that implement the Separation of Concerns (SoC) principle and Single Responsibility Pattern.

## Architecture Overview

The original `TripModel` has been decomposed into smaller, more focused services:

```
                      ┌────────────────────────┐
                      │  TripModelCoordinator  │
                      │  (Facade)              │
                      └────────────────────────┘
                                 │
           ┌──────────┬──────────┼──────────┬──────────┐
           │          │          │          │          │
┌──────────▼─┐  ┌─────▼──────┐  ┌▼──────────┐  ┌──────▼───────┐
│ TripMgmtSvc │  │ MemberSvc  │  │ InviteSvc │  │ TripChatSvc  │
└──────────┬─┘  └─────┬──────┘  └┬──────────┘  └──────┬───────┘
           │          │          │          │          │
           └──────────┴──────────┼──────────┴──────────┘
                                 │
                         ┌───────▼───────┐
                         │    Stores     │
                         └───────────────┘
```

## Services

1. **TripManagementService**
   - Core CRUD operations for trips
   - Trip status management
   - Trip search/listing
   - Weather updates

2. **TripMemberService**
   - Member management (add, update role, remove)
   - Member role verification
   - Membership queries

3. **InvitationService**
   - Create and manage invitations
   - Process invitation responses
   - User lookups

4. **TripChatService**
   - List chat messages for trips
   - Update last read message

5. **TripModelCoordinator**
   - Facade that implements the TripModelInterface
   - Orchestrates the individual services
   - Provides backward compatibility

## Benefits

- **Better Separation of Concerns**: Each service has a clear, focused responsibility
- **Improved Testability**: Services can be tested independently
- **Reduced Coupling**: Dependencies are better managed and more explicit
- **Easier Maintenance**: Changes to one area don't affect others
- **Better Code Organization**: Code is more modular and easier to navigate

## Usage

The original `TripModel` now delegates to the coordinator, so all existing code will continue to work without changes. New code should use the coordinator or individual services directly where appropriate.

```go
// Example of using the coordinator directly
coordinator := service.NewTripModelCoordinator(
    store,
    eventBus,
    weatherSvc,
    supabaseClient,
    config,
    emailSvc,
    chatStore,
)

// Create a trip
trip := &types.Trip{/* ... */}
err := coordinator.CreateTrip(ctx, trip)
```

## Migration Path

1. Current implementation: TripModel delegates to Coordinator
2. Future implementation: Use services directly in handlers
3. Long-term: Phase out the TripModel wrapper entirely 