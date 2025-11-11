package router

import (
	"context"
	"net/http"
	"regexp"
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

	placeholderRegex := regexp.MustCompile(`\{([^}:]+)(?::([^}]+))?\}`)
	regexPattern := "^"
	lastIndex := 0

	for _, match := range placeholderRegex.FindAllStringSubmatchIndex(pattern, -1) {
		start, end := match[0], match[1]
		keyStart, keyEnd := match[2], match[3]
		regexPattern += regexp.QuoteMeta(pattern[lastIndex:start])

		key := pattern[keyStart:keyEnd]
		keys = append(keys, key)

		// If there's a custom regex (second capturing group indices), use it; otherwise default
		if len(match) >= 6 && match[4] != -1 && match[5] != -1 {
			customRegex := pattern[match[4]:match[5]]
			regexPattern += "(" + customRegex + ")"
		} else {
			regexPattern += `([^/]+)`
		}
		lastIndex = end
	}

	regexPattern += regexp.QuoteMeta(pattern[lastIndex:])
	regexPattern += "$"

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
