package router

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

// PatternRouter provides pattern-based routing with placeholder support
// Supports patterns like "/api/devices/{serial}/files"
type PatternRouter struct {
	routes []routeEntry
}

type routeEntry struct {
	pattern *regexp.Regexp
	handler http.HandlerFunc
	keys    []string
}

// NewPatternRouter creates a new pattern router
func NewPatternRouter() *PatternRouter {
	return &PatternRouter{
		routes: make([]routeEntry, 0),
	}
}

// HandleFunc registers a handler for a URL pattern with placeholders
// Pattern examples:
//   - "/api/devices/{serial}" - matches /api/devices/abc123
//   - "/api/devices/{serial}/files" - matches /api/devices/abc123/files
//   - "/api/devices/{serial}/files/{path:.*}" - matches /api/devices/abc123/files/any/path
func (pr *PatternRouter) HandleFunc(pattern string, handler http.HandlerFunc) {
	regexPattern, keys := compilePattern(pattern)
	pr.routes = append(pr.routes, routeEntry{
		pattern: regexPattern,
		handler: handler,
		keys:    keys,
	})
}

// ServeHTTP implements http.Handler interface
func (pr *PatternRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range pr.routes {
		if matches := route.pattern.FindStringSubmatch(r.URL.Path); matches != nil {
			// Extract path parameters and store them in request context
			if len(route.keys) > 0 {
				// Create a new request with path parameters
				ctx := r.Context()
				for i, key := range route.keys {
					if i+1 < len(matches) {
						ctx = withPathParam(ctx, key, matches[i+1])
					}
				}
				r = r.WithContext(ctx)
			}
			route.handler(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

// compilePattern converts a pattern with placeholders to a regular expression
// Returns the compiled regex and a list of placeholder keys
func compilePattern(pattern string) (*regexp.Regexp, []string) {
	keys := make([]string, 0)

	// Escape special regex characters except {}
	regexPattern := regexp.QuoteMeta(pattern)

	// Find all placeholders like {key} or {key:regex}
	placeholderRegex := regexp.MustCompile(`\\\{([^}:]+)(?::([^}]+))?\\\}`)
	regexPattern = placeholderRegex.ReplaceAllStringFunc(regexPattern, func(match string) string {
		// Extract the placeholder content (remove escaped braces)
		content := strings.TrimPrefix(strings.TrimSuffix(match, `\}`), `\{`)
		parts := strings.SplitN(content, ":", 2)

		key := parts[0]
		keys = append(keys, key)

		// Use custom regex if provided, otherwise match any non-slash characters
		if len(parts) == 2 {
			return "(" + parts[1] + ")"
		}
		return `([^/]+)`
	})

	// Anchor the pattern to match the full path
	regexPattern = "^" + regexPattern + "$"

	return regexp.MustCompile(regexPattern), keys
}

// PathParam retrieves a path parameter from the request context
//
// Use this function in your handlers to access URL path parameters defined
// with placeholders like {serial}, {id}, etc.
//
// Example:
//
//	router.HandleFunc("/api/devices/{serial}/files/{filename}", handler)
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    serial := router.PathParam(r, "serial")
//	    filename := router.PathParam(r, "filename")
//	    // Use the parameters...
//	}
//
// Returns an empty string if the parameter doesn't exist.
func PathParam(r *http.Request, key string) string {
	// Use a string with prefix as context key to avoid type conflicts
	contextKey := "gbox-pattern-router:" + key
	if val := r.Context().Value(contextKey); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func withPathParam(ctx context.Context, key, value string) context.Context {
	// Use a string with prefix as context key to avoid type conflicts
	contextKey := "gbox-pattern-router:" + key
	return context.WithValue(ctx, contextKey, value)
}
