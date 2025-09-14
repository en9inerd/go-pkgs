package router

import (
	"net/http"
	"regexp"
	"strings"
)

// matches "METHOD /path"
var reGo122 = regexp.MustCompile(`^(\S+)\s+(.+)$`)

// Handle registers a route with middlewares applied.
func (g *Group) Handle(pattern string, handler http.Handler) {
	g.lockRoot()

	if strings.HasSuffix(pattern, "/") {
		full := g.basePath + pattern
		g.mux.Handle(full, g.wrapMiddleware(handler))
		return
	}
	g.register(pattern, handler.ServeHTTP)
}

// HandleFunc registers a route handler function.
func (g *Group) HandleFunc(pattern string, handler http.HandlerFunc) {
	g.register(pattern, handler)
}

// HandleFiles serves static files.
func (g *Group) HandleFiles(pattern string, root http.FileSystem) {
	g.lockRoot()

	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}
	full := g.basePath + pattern

	if pattern == "/" && g.basePath == "" {
		g.mux.Handle("/", g.wrapMiddleware(http.FileServer(root)))
		return
	}

	handler := http.StripPrefix(strings.TrimSuffix(full, "/"), http.FileServer(root))
	g.mux.Handle(full, g.wrapMiddleware(handler))
}

// HandleRoot registers a handler for the group's root without redirect.
func (g *Group) HandleRoot(method string, handler http.Handler) {
	g.lockRoot()
	pattern := g.basePath
	if pattern == "" {
		pattern = "/"
	}
	if method != "" {
		pattern = method + " " + pattern
	}
	g.mux.Handle(pattern, g.wrapMiddleware(handler))
}

// HandleRootFunc registers a root handler func.
func (g *Group) HandleRootFunc(method string, handler http.HandlerFunc) {
	g.lockRoot()
	pattern := g.basePath
	if pattern == "" {
		pattern = "/"
	}
	if method != "" {
		pattern = method + " " + pattern
	}
	g.mux.HandleFunc(pattern, g.wrapMiddleware(handler).ServeHTTP)
}

// Handler proxies to mux.Handler.
func (g *Group) Handler(r *http.Request) (h http.Handler, pattern string) {
	return g.mux.Handler(r)
}

func (g *Group) register(pattern string, handler http.HandlerFunc) {
	g.lockRoot()
	matches := reGo122.FindStringSubmatch(pattern)

	var path, method string
	if len(matches) > 2 {
		method, path = matches[1], matches[2]
		pattern = method + " " + g.basePath + path
	} else {
		path = pattern
		pattern = g.basePath + pattern
	}

	if pattern == "/" || path == "/" {
		if method != "" {
			pattern = method + " " + g.basePath + "/{$}"
		} else {
			pattern = g.basePath + "/{$}"
		}
	}
	g.mux.HandleFunc(pattern, g.wrapMiddleware(handler).ServeHTTP)
}
