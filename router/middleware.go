package router

import "net/http"

// Use appends middleware(s) to the group.
func (g *Group) Use(mw func(http.Handler) http.Handler, more ...func(http.Handler) http.Handler) {
	if g.routesLocked {
		panic("router: Use called after routes were registered; add middleware before routes or use Group/With")
	}
	g.middlewares = append(g.middlewares, mw)
	g.middlewares = append(g.middlewares, more...)
}

// With returns a new group with appended middleware(s).
func (g *Group) With(mw func(http.Handler) http.Handler, more ...func(http.Handler) http.Handler) *Group {
	newStack := make([]func(http.Handler) http.Handler, len(g.middlewares), len(g.middlewares)+len(more)+1)
	copy(newStack, g.middlewares)
	newStack = append(newStack, mw)
	newStack = append(newStack, more...)

	ng := &Group{
		mux:         g.mux,
		basePath:    g.basePath,
		middlewares: newStack,
		root:        g.root,
		rootCount:   g.rootCount,
	}
	if ng.root == nil {
		ng.root = g
		ng.rootCount = len(g.middlewares)
	}
	return ng
}

// Wrap applies middleware(s) around a handler.
func Wrap(handler http.Handler, mw1 func(http.Handler) http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return mw1(handler)
}

// wrapMiddleware applies the group's middlewares.
func (g *Group) wrapMiddleware(handler http.Handler) http.Handler {
	if g.root == nil {
		return handler
	}
	start := g.rootCount
	if start > len(g.middlewares) {
		start = len(g.middlewares)
	}
	for i := len(g.middlewares) - 1; i >= start; i-- {
		handler = g.middlewares[i](handler)
	}
	return handler
}

// wrapGlobal applies only the root middlewares.
func (g *Group) wrapGlobal(handler http.Handler) http.Handler {
	root := g
	if g.root != nil {
		root = g.root
	}
	for i := len(root.middlewares) - 1; i >= 0; i-- {
		handler = root.middlewares[i](handler)
	}
	return handler
}
