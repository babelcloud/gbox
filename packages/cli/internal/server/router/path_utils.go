package router

import (
	"net/http"
	"strings"
)

// PathTransformer handles automatic path prefix transformation
type PathTransformer struct {
	prefix string
}

// NewPathTransformer creates a new path transformer with the given prefix
func NewPathTransformer(prefix string) *PathTransformer {
	return &PathTransformer{
		prefix: strings.TrimSuffix(prefix, "/"),
	}
}

// TransformHandler wraps a handler to automatically transform request paths
// It removes the prefix from the request path before passing to the handler
func (pt *PathTransformer) TransformHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a copy of the request with the transformed path
		newReq := *r
		newURL := *r.URL
		newReq.URL = &newURL

		// Remove the prefix from the path
		if strings.HasPrefix(r.URL.Path, pt.prefix) {
			newReq.URL.Path = strings.TrimPrefix(r.URL.Path, pt.prefix)
			// Ensure we don't have double slashes
			if !strings.HasPrefix(newReq.URL.Path, "/") {
				newReq.URL.Path = "/" + newReq.URL.Path
			}
		}

		// Call the original handler with the transformed request
		handler(w, &newReq)
	}
}

// AddPrefix adds the configured prefix to a path
func (pt *PathTransformer) AddPrefix(path string) string {
	if pt.prefix == "" {
		return path
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return pt.prefix + path
}