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

3. **Services**:
   - `LocationService`: Handles business logic for location operations
   - Validates location data
   - Publishes location update events

4. **Handlers**:
   - `LocationHandler`: Handles HTTP requests for location operations
   - Includes endpoints for updating location and retrieving trip member locations

5. **Events**:
   - `EventTypeLocationUpdated`: Event type for location updates
   - Location updates are published as events to enable real-time updates

## Usage Guidelines

1. **Client-side Implementation**:
   - Mobile clients should update location periodically (e.g., every 1-5 minutes) when a trip is active
   - Web clients can update location at longer intervals
   - Set appropriate accuracy based on device capabilities

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

4. **Offline Support**: Implement queue-based location updates for when users are offline.
