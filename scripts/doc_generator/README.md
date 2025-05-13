# API Documentation Generator

This tool scans the codebase for API handlers and helps document them using Swagger annotations.

## Usage

Run from the project root:

```bash
cd scripts/doc_generator
go run main.go
```

This will:

1. Scan all handler files in the `handlers` directory
2. Identify handler functions that lack Swagger documentation
3. Generate template Swagger comments for each undocumented handler
4. Provide a summary of documentation coverage

## Benefits

- Ensures consistent API documentation
- Automatically generates appropriate tags, parameters, and response codes
- Helps maintain up-to-date and comprehensive documentation
- Supports Swagger UI integration for interactive API exploration

## Integration with Swagger

Once handlers are documented with Swagger annotations, you can generate the Swagger spec:

```bash
swag init -g main.go -o ./static/docs/api
```

This will create or update the OpenAPI specification in the `static/docs/api` directory, which can be used with Swagger UI to provide interactive API documentation.

## Example Output

```
=== API Endpoints Documentation Status ===

Handler: TripHandler.GetTripHandler
  File: handlers/trip_handler.go
  Line: 120
  Needs Swagger documentation

  Suggested documentation template:

// GetTripHandler godoc
// @Summary Get Trip
// @Description Get Trip
// @Tags trip
// @Accept json
// @Produce json
// @Param id path string true "ID"
// @Success 200 {object} models.Response "Successful response"
// @Success 400 {object} models.ErrorResponse "Bad request"
// @Success 401 {object} models.ErrorResponse "Unauthorized"
// @Success 500 {object} models.ErrorResponse "Internal server error"
// @Router /trip/{id} [get]
// @Security BearerAuth

Summary: 15 of 30 handlers documented (50.0%) 