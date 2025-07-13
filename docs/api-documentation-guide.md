# API Documentation Guide

## Overview

NomadCrew backend uses [Swagger/OpenAPI](https://swagger.io/specification/) for API documentation. This document explains how to document API endpoints and how to maintain the documentation.

## Documentation Structure

The API documentation consists of:

1. **Swagger Annotations**: Comments in handler functions that describe endpoints
2. **OpenAPI Specification**: Generated from annotations, stored in `static/docs/api/openapi.yaml`
3. **Swagger UI**: Provides an interactive web interface for exploring the API

## How to Document an Endpoint

Each handler function that exposes an API endpoint should have Swagger annotations. Here's a template:

```go
// HandlerName godoc
// @Summary Brief summary of what the endpoint does
// @Description Detailed description of the endpoint
// @Tags category-name
// @Accept json
// @Produce json
// @Param param_name path/query/body/header/formData type required "Description"
// @Success 200 {object} ResponseModel "Success response description"
// @Failure 400 {object} ErrorResponse "Bad request description"
// @Failure 401 {object} ErrorResponse "Unauthorized description"
// @Failure 500 {object} ErrorResponse "Server error description"
// @Router /path [method]
// @Security BearerAuth
func (h *Handler) HandlerName(c *gin.Context) {
    // Handler implementation
}
```

### Annotation Explanation

- `// HandlerName godoc`: Required marker for Swagger to identify the documentation
- `@Summary`: Brief description (single line)
- `@Description`: Detailed description (can be multiple lines)
- `@Tags`: Category for grouping endpoints in UI
- `@Accept`: Accepted MIME types (typically `json`)
- `@Produce`: Response MIME types (typically `json`)
- `@Param`: Parameter details (name, location, type, required, description)
- `@Success`: Success response with status code, type, and description
- `@Failure`: Error response with status code, type, and description
- `@Router`: Endpoint path and HTTP method
- `@Security`: Authentication requirements

## Documentation Workflow

### 1. Identify Undocumented Endpoints

Use the documentation generator tool:

```bash
cd scripts/doc_generator
go run main.go
```

This will identify handlers lacking documentation and provide templates.

### 2. Add Swagger Annotations

Add the suggested annotations (or custom ones) to the handler function.

### 3. Generate the OpenAPI Specification

```bash
swag init -g main.go -o ./static/docs/api
```

This will update the OpenAPI specification in `static/docs/api/`.

### 4. Verify Documentation

Start the server and access the Swagger UI (typically at `/swagger/index.html`) to verify your documentation.

## Models Documentation

You should document models used in your API responses:

```go
// User represents a user in the system
// @Description User account information
type User struct {
    // The unique identifier for the user
    ID        string    `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
    
    // The user's name
    Name      string    `json:"name" example:"John Doe"`
    
    // The user's email address
    Email     string    `json:"email" example:"john@example.com"`
    
    // When the user was created
    CreatedAt time.Time `json:"createdAt" example:"2023-01-01T00:00:00Z"`
}
```

## Best Practices

1. **Be Consistent**: Use consistent naming, descriptions, and parameter models
2. **Include Examples**: Add example values in model documentation
3. **Detailed Parameters**: Describe all parameters, including query parameters
4. **Security**: Document authentication requirements for each endpoint
5. **Versioning**: Include API version information
6. **Response Models**: Document all response models
7. **Error Responses**: Document all possible error responses

## Tips for Better Documentation

- Group related endpoints with the same tag
- Use meaningful tag names (typically resource names like "users", "trips")
- Provide clear descriptions that explain the purpose of the endpoint
- Specify all possible response status codes
- Include validation requirements in parameter descriptions

## Resource Path Conventions

- Collection endpoints: `/resources`
- Individual resource: `/resources/{id}`
- Nested resources: `/resources/{id}/sub-resources`
- Actions on resources: `/resources/{id}/actions`

## HTTP Method Conventions

- `GET`: Retrieve data
- `POST`: Create new data
- `PUT`: Update existing data (full replace)
- `PATCH`: Partial update
- `DELETE`: Remove data

## Maintaining Documentation

- Update documentation when you change endpoints
- Review documentation regularly for accuracy
- Regenerate OpenAPI spec after making changes
- Include documentation updates in code reviews 