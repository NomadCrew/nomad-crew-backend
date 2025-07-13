package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Handler represents a handler function that needs documentation
type Handler struct {
	Path       string // File path
	Package    string // Package name
	Handler    string // Handler function name
	Line       int    // Line number
	HasSwagger bool   // Whether it already has Swagger comments
}

func main() {
	// Directory to search through (handlers directory)
	handlersDir := "../../handlers"

	// Find all Go files in the handlers directory
	files, err := findGoFiles(handlersDir)
	if err != nil {
		fmt.Printf("Error finding Go files: %v\n", err)
		os.Exit(1)
	}

	// Find all handler functions that need documentation
	handlers, err := findHandlerFunctions(files)
	if err != nil {
		fmt.Printf("Error finding handler functions: %v\n", err)
		os.Exit(1)
	}

	// Generate documentation for handlers without Swagger comments
	generateDocumentation(handlers)
}

// findGoFiles finds all Go files in the specified directory
func findGoFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// findHandlerFunctions finds all handler functions in the given files
func findHandlerFunctions(files []string) ([]Handler, error) {
	var handlers []Handler
	handlerRegex := regexp.MustCompile(`func \(\w+ \*(\w+Handler)\) (\w+)\(`)
	swaggerRegex := regexp.MustCompile(`// @Router`)

	for _, file := range files {
		// Clean the path to prevent potential traversal issues
		file = filepath.Clean(file)

		// Read the file
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", file, err)
		}

		lines := strings.Split(string(content), "\n")
		packageName := extractPackageName(lines)

		// Find all handler functions
		for i, line := range lines {
			match := handlerRegex.FindStringSubmatch(line)
			if len(match) > 2 {
				// Check if there's already a Swagger comment
				hasSwagger := false
				// Look at preceding lines for Swagger comments
				for j := max(0, i-15); j < i; j++ {
					if swaggerRegex.MatchString(lines[j]) {
						hasSwagger = true
						break
					}
				}

				handlers = append(handlers, Handler{
					Path:       file,
					Package:    packageName,
					Handler:    match[2],
					Line:       i + 1,
					HasSwagger: hasSwagger,
				})
			}
		}
	}

	return handlers, nil
}

// extractPackageName extracts the package name from the file content
func extractPackageName(lines []string) string {
	packageRegex := regexp.MustCompile(`^package (\w+)`)
	for _, line := range lines {
		match := packageRegex.FindStringSubmatch(line)
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// generateDocumentation generates Swagger documentation for handlers
func generateDocumentation(handlers []Handler) {
	totalHandlers := 0
	documentedHandlers := 0

	fmt.Println("=== API Endpoints Documentation Status ===")
	fmt.Println()

	for _, handler := range handlers {
		totalHandlers++
		if handler.HasSwagger {
			documentedHandlers++
			continue
		}

		relativePath := strings.ReplaceAll(handler.Path, "\\", "/")
		relativePath = strings.TrimPrefix(relativePath, "../../")

		fmt.Printf("Handler: %s.%s\n", handler.Package, handler.Handler)
		fmt.Printf("  File: %s\n", relativePath)
		fmt.Printf("  Line: %d\n", handler.Line)
		fmt.Printf("  Needs Swagger documentation\n\n")

		// Suggest documentation template based on handler name
		suggestDocumentation(handler)
	}

	fmt.Printf("\nSummary: %d of %d handlers documented (%.1f%%)\n",
		documentedHandlers, totalHandlers,
		float64(documentedHandlers)/float64(totalHandlers)*100)
}

// suggestDocumentation suggests Swagger documentation for a handler
func suggestDocumentation(handler Handler) {
	handlerName := handler.Handler

	// Example Swagger template
	fmt.Printf("  Suggested documentation template:\n\n")
	fmt.Printf("// %s godoc\n", handlerName)
	fmt.Printf("// @Summary %s\n", humanizeHandlerName(handlerName))
	fmt.Printf("// @Description %s\n", humanizeHandlerName(handlerName))
	fmt.Printf("// @Tags %s\n", strings.ToLower(strings.TrimSuffix(handler.Package, "Handler")))
	fmt.Printf("// @Accept json\n")
	fmt.Printf("// @Produce json\n")

	// Guess parameters based on handler name
	params := generateParams(handlerName)
	for _, param := range params {
		fmt.Printf("// @Param %s\n", param)
	}

	// Guess responses based on handler name
	responses := generateResponses(handlerName)
	for _, response := range responses {
		fmt.Printf("// @Success %s\n", response)
	}

	fmt.Printf("// @Router /%s [%s]\n", guessRouterPath(handlerName), guessHTTPMethod(handlerName))
	fmt.Printf("// @Security BearerAuth\n")
	fmt.Printf("\n")
}

// humanizeHandlerName converts a camel case handler name to a human-readable form
func humanizeHandlerName(name string) string {
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	name = re.ReplaceAllString(name, "$1 $2")

	// Handle common prefixes
	name = strings.TrimSuffix(name, "Handler")

	if strings.HasPrefix(name, "Get") {
		return "Get " + name[3:]
	} else if strings.HasPrefix(name, "Create") {
		return "Create " + name[6:]
	} else if strings.HasPrefix(name, "Update") {
		return "Update " + name[6:]
	} else if strings.HasPrefix(name, "Delete") {
		return "Delete " + name[6:]
	} else if strings.HasPrefix(name, "List") {
		return "List " + name[4:]
	}

	return name
}

// generateParams generates parameter documentation based on handler name
func generateParams(name string) []string {
	var params []string

	// Check for common patterns in handler names
	if strings.Contains(name, "ById") || strings.Contains(name, "ByID") {
		params = append(params, "id path string true \"ID\"")
	}

	if strings.HasPrefix(name, "Create") || strings.HasPrefix(name, "Update") {
		params = append(params, "request body object true \"Request body\"")
	}

	if strings.HasPrefix(name, "List") {
		params = append(params, "limit query int false \"Number of items to return (default 20, max 100)\"")
		params = append(params, "offset query int false \"Offset for pagination (default 0)\"")
	}

	return params
}

// generateResponses generates response documentation based on handler name
func generateResponses(name string) []string {
	var responses []string

	if strings.HasPrefix(name, "Get") || strings.HasPrefix(name, "List") {
		responses = append(responses, "200 {object} models.Response \"Successful response\"")
	} else if strings.HasPrefix(name, "Create") {
		responses = append(responses, "201 {object} models.Response \"Created successfully\"")
	} else if strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Delete") {
		responses = append(responses, "200 {object} models.Response \"Successful operation\"")
	}

	responses = append(responses, "400 {object} models.ErrorResponse \"Bad request\"")
	responses = append(responses, "401 {object} models.ErrorResponse \"Unauthorized\"")
	responses = append(responses, "500 {object} models.ErrorResponse \"Internal server error\"")

	return responses
}

// guessRouterPath guesses the router path based on handler name
func guessRouterPath(name string) string {
	// Strip common prefixes
	path := name
	for _, prefix := range []string{"Get", "Create", "Update", "Delete", "List"} {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}

	// Convert camel case to path segments
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	path = re.ReplaceAllString(path, "$1-$2")
	path = strings.ToLower(path)

	// Replace "by-id" with a parameter
	path = strings.Replace(path, "by-id", "/{id}", -1)

	return path
}

// guessHTTPMethod guesses the HTTP method based on handler name
func guessHTTPMethod(name string) string {
	if strings.HasPrefix(name, "Get") || strings.HasPrefix(name, "List") {
		return "get"
	} else if strings.HasPrefix(name, "Create") {
		return "post"
	} else if strings.HasPrefix(name, "Update") {
		return "put"
	} else if strings.HasPrefix(name, "Delete") {
		return "delete"
	}

	return "get" // Default
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
