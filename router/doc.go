// Package router provides a simple way to group routes and apply middleware
// using Go's standard http.ServeMux (Go 1.22+). It supports:
//
//   - Grouping routes under a common base path
//   - Attaching middleware stacks at the root or per group
//   - Mounting static file handlers
//   - Registering handlers with or without HTTP method prefixes
//   - Defining custom NotFound (404) handlers
//
// Example usage:
//
//	mux := http.NewServeMux()
//	r := router.New(mux)
//
//	// global middleware
//	r.Use(loggingMiddleware)
//
//	// mount API group
//	api := r.Mount("/api")
//	api.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
//	    w.Write([]byte("pong"))
//	})
//
//	// serve
//	http.ListenAndServe(":8080", r)
//
// Middleware added to the root group executes for every request. Middleware
// added to a subgroup executes only for that group's routes. The order of
// middleware application is the same as the order they are added, i.e. first
// added runs outermost.
//
// Route patterns may be plain paths ("/foo") or include an HTTP method prefix
// ("GET /foo"). Root "/" patterns are normalized to "/{$}" to avoid acting as
// a catch-all.
package router
