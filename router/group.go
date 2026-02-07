// Package router provides a way to group routes and apply middleware.
// Works with Go's standard http.ServeMux (Go 1.22+).
package router

import (
	"net/http"
)

// Group represents a collection of routes with optional middleware.
type Group struct {
	mux         *http.ServeMux
	basePath    string
	middlewares []func(http.Handler) http.Handler

	// optional custom 404 handler
	notFound http.HandlerFunc

	// root points to the root group for global middleware application.
	root *Group

	// routesLocked indicates that routes have been registered on the root group
	// and no further root-level middlewares may be added.
	routesLocked bool

	// rootCount captures how many root middlewares were present when this group
	// was created. Used to avoid double-applying root middlewares.
	rootCount int
}

// New creates a new root Group bound to the given mux.
func New(mux *http.ServeMux) *Group {
	return &Group{mux: mux}
}

// RootGroup creates a new root Group with a base path bound to the given mux.
func RootGroup(mux *http.ServeMux, basePath string) *Group {
	return &Group{mux: mux, basePath: basePath}
}

// ServeHTTP implements http.Handler for the group.
func (g *Group) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	root := g
	if g.root != nil {
		root = g.root
	}

	// resolve the handler and pattern from mux
	_, pattern := g.mux.Handler(r)

	if pattern != "" {
		r2 := *r
		r2.Pattern = pattern
		r = &r2
	}

	muxHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pattern == "" && root.notFound != nil {
			probe := &statusRecorder{status: http.StatusOK}
			g.mux.ServeHTTP(probe, r)

			if probe.status == http.StatusMethodNotAllowed {
				g.mux.ServeHTTP(w, r)
				return
			}
			root.notFound.ServeHTTP(w, r)
			return
		}
		g.mux.ServeHTTP(w, r)
	})

	root.wrapGlobal(muxHandler).ServeHTTP(w, r)
}

// Group creates a new subgroup with the same middleware stack.
func (g *Group) Group() *Group {
	return g.clone()
}

// Mount creates a new subgroup with a base path.
func (g *Group) Mount(basePath string) *Group {
	ng := g.clone()
	ng.basePath += basePath
	return ng
}

// Route configures the group inside the provided function.
func (g *Group) Route(fn func(*Group)) { fn(g) }

// NotFoundHandler sets a custom 404 handler on the root group.
func (g *Group) NotFoundHandler(handler http.HandlerFunc) {
	if g.root != nil {
		g.root.notFound = handler
		return
	}
	g.notFound = handler
}
