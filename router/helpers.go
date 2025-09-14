package router

import "net/http"

func (g *Group) clone() *Group {
	mws := make([]func(http.Handler) http.Handler, len(g.middlewares))
	copy(mws, g.middlewares)

	ng := &Group{
		mux:         g.mux,
		basePath:    g.basePath,
		middlewares: mws,
		root:        g.root,
		rootCount:   g.rootCount,
	}
	if ng.root == nil {
		ng.root = g
		ng.rootCount = len(g.middlewares)
	}
	return ng
}

func (g *Group) lockRoot() { g.routesLocked = true }

// statusRecorder is used to probe mux responses.
type statusRecorder struct {
	status int
}

func (r *statusRecorder) Header() http.Header       { return make(http.Header) }
func (r *statusRecorder) Write([]byte) (int, error) { return 0, nil }
func (r *statusRecorder) WriteHeader(status int)    { r.status = status }
