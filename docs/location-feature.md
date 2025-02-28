# Location Sharing Feature

## Overview

The location sharing feature allows trip members to share their real-time location with other members of the same trip. This is a critical feature for the trip management functionality, as it enables users to see each other's locations on a map.

## API Endpoints

### Update User Location

- **Path**: `/v1/location/update`
- **Method**: POST
- **Authentication**: Required
- **Description**: Updates the current user's location
- **Request Body**:

  ```json
  {
    "latitude": 37.7749,
    "longitude": -122.4194,
    "accuracy": 10.5,
    "timestamp": 1625097600000
  }
  ```

- **Response**: The updated location object

### Save Offline Location Updates

- **Path**: `/v1/location/offline`
- **Method**: POST
- **Authentication**: Required
- **Description**: Saves location updates that were captured while the device was offline
- **Headers**:
  - `X-Device-ID`: Optional device identifier to track which device sent the updates
- **Request Body**:

  ```json
  {
    "updates": [
      {
        "latitude": 37.7749,
        "longitude": -122.4194,
        "accuracy": 10.5,
        "timestamp": 1625097600000
      },
      {
        "latitude": 37.7750,
        "longitude": -122.4195,
        "accuracy": 8.3,
        "timestamp": 1625097660000
      }
    ]
  }
  ```

- **Response**:

  ```json
  {
    "message": "Offline locations saved successfully",
    "count": 2
  }
  ```

### Process Offline Location Updates

- **Path**: `/v1/location/process-offline`
- **Method**: POST
- **Authentication**: Required
- **Description**: Manually triggers processing of offline location updates for the current user
- **Response**:

  ```json
  {
    "message": "Offline locations processed successfully"
  }
  ```

### Get Trip Member Locations

- **Path**: `/v1/trips/{tripId}/locations`
- **Method**: GET
- **Authentication**: Required
- **Description**: Retrieves the latest locations of all members in a trip
- **Response**: Array of member locations

  ```json
  {
    "locations": [
      {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "tripId": "123e4567-e89b-12d3-a456-426614174001",
        "userId": "123e4567-e89b-12d3-a456-426614174002",
        "latitude": 37.7749,
        "longitude": -122.4194,
        "accuracy": 10.5,
        "timestamp": "2023-06-30T12:00:00Z",
        "createdAt": "2023-06-30T12:00:00Z",
        "updatedAt": "2023-06-30T12:00:00Z",
        "userName": "John Doe",
        "userRole": "MEMBER"
      }
    ]
  }
  ```

## Implementation Details

The location feature is implemented with the following components:

1. **Database**: Locations are stored in the `locations` table, which includes fields for latitude, longitude, accuracy, and timestamps.

2. **Types**:
   - `Location`: Represents a user's geographic location
   - `LocationUpdate`: Represents the payload for updating a location
   - `MemberLocation`: Extends Location with user information
   - `OfflineLocationUpdate`: Represents a batch of location updates captured while offline

3. **Services**:
   - `LocationService`: Handles business logic for location operations
   - `OfflineLocationService`: Handles queue-based offline location updates
   - Validates location data
   - Publishes location update events

4. **Handlers**:
   - `LocationHandler`: Handles HTTP requests for location operations
   - Includes endpoints for updating location, saving offline updates, and retrieving trip member locations

5. **Events**:
   - `EventTypeLocationUpdated`: Event type for location updates
   - Location updates are published as events to enable real-time updates

## Offline Support

The location feature includes support for offline location updates:

1. **Client-side Implementation**:
   - Mobile clients should store location updates locally when offline
   - When connectivity is restored, send batched updates to the `/v1/location/offline` endpoint
   - Include device identifier in the `X-Device-ID` header for tracking

2. **Server-side Implementation**:
   - Offline updates are stored in Redis with a 24-hour TTL
   - Updates are processed asynchronously to avoid blocking the client
   - Processing happens automatically when updates are submitted
   - Manual processing can be triggered via the `/v1/location/process-offline` endpoint

3. **Data Validation**:
   - Location updates older than 24 hours are discarded
   - Updates with invalid coordinates are rejected
   - Duplicate updates are handled gracefully

4. **Performance Considerations**:
   - Batched updates minimize API calls
   - Redis-based queue ensures scalability
   - Processing uses locks to prevent concurrent processing of the same user's updates

## Usage Guidelines

1. **Client-side Implementation**:
   - Mobile clients should update location periodically (e.g., every 1-5 minutes) when a trip is active
   - Web clients can update location at longer intervals
   - Set appropriate accuracy based on device capabilities
   - Store updates locally when offline and send when connectivity is restored

2. **Privacy Considerations**:
   - Location data is only shared with members of the same trip
   - Location data older than 24 hours is not returned by the API
   - Users should be informed about location sharing when joining a trip

3. **Performance Considerations**:
   - Location updates are stored efficiently to handle high volume
   - Only the most recent location for each user is returned by the API
   - Events enable real-time updates without polling

## Future Enhancements

1. **Geofencing**: Add support for trip-specific geofences to trigger notifications when members enter or leave specific areas.

2. **Location History**: Implement endpoints to retrieve historical location data for trip members.

3. **Privacy Controls**: Add user-level controls for location sharing preferences.
