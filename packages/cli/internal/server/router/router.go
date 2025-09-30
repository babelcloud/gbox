package router

import (
	"net/http"
)

// Router defines the interface for route registration
type Router interface {
	RegisterRoutes(mux *http.ServeMux, server interface{})
	GetPathPrefix() string
}

// RouteGroup helps organize related routes
type RouteGroup struct {
	prefix string
	mux    *http.ServeMux
	server interface{}
}

// NewRouteGroup creates a new route group with a common prefix
func NewRouteGroup(prefix string, mux *http.ServeMux, server interface{}) *RouteGroup {
	return &RouteGroup{
		prefix: prefix,
		mux:    mux,
		server: server,
	}
}

// HandleFunc registers a handler function with the group's prefix
func (g *RouteGroup) HandleFunc(pattern string, handler http.HandlerFunc) {
	fullPattern := g.prefix + pattern
	g.mux.HandleFunc(fullPattern, handler)
}

// Handle registers a handler with the group's prefix
func (g *RouteGroup) Handle(pattern string, handler http.Handler) {
	fullPattern := g.prefix + pattern
	g.mux.Handle(fullPattern, handler)
}