package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper that returns a middleware which writes prefix before calling next
func writeBeforeMiddleware(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(prefix))
			next.ServeHTTP(w, r)
		})
	}
}

func TestMiddlewareOrderAndHandlerExecution(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	root.Use(writeBeforeMiddleware("root;"))

	child := root.With(writeBeforeMiddleware("child;"))
	child.HandleFunc("/a", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("handler;"))
	}))

	// request must go through root.ServeHTTP to have global middlewares applied
	req := httptest.NewRequest(http.MethodGet, "/a", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	body := rec.Body.String()
	want := "root;child;handler;"
	if body != want {
		t.Fatalf("unexpected body: got %q, want %q", body, want)
	}
}

func TestNotFoundHandlerUsedForTrue404(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	root.NotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("custom-404"))
	}))

	// no routes registered -> should invoke custom 404
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	if rec.Body.String() != "custom-404" {
		t.Fatalf("expected custom-404, got %q", rec.Body.String())
	}
}

func TestHandleFilesServesFilesUnderPrefix(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	dir := t.TempDir()
	filename := "foo.txt"
	content := "file-content"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// mount file server at /static/
	root.HandleFiles("/static/", http.Dir(dir))

	// request the file
	req := httptest.NewRequest(http.MethodGet, "/static/foo.txt", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, content) {
		t.Fatalf("expected file content %q in response, got %q", content, body)
	}
}

func TestHandleRootAndHandleRootFunc(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	// test HandleRoot (handler, not Func)
	root.HandleRoot("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("root-handler"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Body.String() != "root-handler" {
		t.Fatalf("expected root-handler, got %q", rec.Body.String())
	}

	// test HandleRootFunc
	root2 := New(http.NewServeMux())
	root2.HandleRootFunc("", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("root-func"))
	})
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	root2.ServeHTTP(rec2, req2)
	if rec2.Body.String() != "root-func" {
		t.Fatalf("expected root-func, got %q", rec2.Body.String())
	}
}

func TestWithDoesNotMutateOriginalStack(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	root.Use(writeBeforeMiddleware("a;"))
	if len(root.middlewares) != 1 {
		t.Fatalf("expected root to have 1 middleware, got %d", len(root.middlewares))
	}

	newGroup := root.With(writeBeforeMiddleware("b;"))
	if len(root.middlewares) != 1 {
		t.Fatalf("root middlewares mutated; want 1 got %d", len(root.middlewares))
	}
	if len(newGroup.middlewares) != 2 {
		t.Fatalf("expected newGroup to have 2 middlewares, got %d", len(newGroup.middlewares))
	}
}

func TestUsePanicsIfRoutesLocked(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux)

	// Register a route to lock the group
	g.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })

	// subsequent Use should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic from Use after routes registered")
		}
	}()
	g.Use(writeBeforeMiddleware("should-panic;"))
}

func TestWrapHelperOrder(t *testing.T) {
	// base handler writes "H"
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("H"))
	})

	// mw1 writes "A" before next
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("A"))
			next.ServeHTTP(w, r)
		})
	}
	// mw2 writes "B" before next
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("B"))
			next.ServeHTTP(w, r)
		})
	}

	// Wrap semantics: mws ... are applied inside-out, then mw1 is outermost.
	// In our implementation: Wrap(base, mw1, mw2) => mw1(mw2(base))
	wrapped := Wrap(base, mw1, mw2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Body.String() != "ABH" {
		t.Fatalf("unexpected wrap order: got %q, want %q", rec.Body.String(), "ABH")
	}
}

func TestMountAndGroupBasePathPrefixing(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	// mount sub-group at /api
	api := root.Mount("/api")
	api.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("pong")) })

	// request "/api/ping"
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Body.String() != "pong" {
		t.Fatalf("expected pong, got %q", rec.Body.String())
	}
}

func TestHandlerReturnsMuxHandlerAndPattern(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux)
	g.HandleFunc("/h", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	req := httptest.NewRequest(http.MethodGet, "/h", nil)
	h, pat := g.Handler(req)
	if h == nil {
		t.Fatalf("expected handler, got nil")
	}
	if pat == "" {
		t.Fatalf("expected non-empty pattern, got empty")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("handler did not produce expected output; got %q", rec.Body.String())
	}
}

func TestRouteConfigAndRouteMethod(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux)

	called := false
	g.Route(func(gr *Group) {
		called = true
	})
	if !called {
		t.Fatalf("Route did not call configure function")
	}
}

func TestMethodPatternRegistration(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	// register a GET /hello pattern
	root.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("world"))
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Body.String() != "world" {
		t.Fatalf("expected world, got %q", rec.Body.String())
	}
}

func TestRootPatternRewriteForSlash(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	// Register with explicit "/" pattern (should rewrite internally)
	root.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("slash"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	if rec.Body.String() != "slash" {
		t.Fatalf("expected slash, got %q", rec.Body.String())
	}
}

func TestChildGroupMiddlewareOnlyAppliesNewOnes(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)
	root.Use(writeBeforeMiddleware("root;"))

	child := root.Group()
	child.Use(writeBeforeMiddleware("child;"))

	child.HandleFunc("/g", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/g", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	if rec.Body.String() != "root;child;ok" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestWrapMiddlewareOnRootDoesNothing(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)

	h := root.wrapMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("x"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Body.String() != "x" {
		t.Fatalf("expected x, got %q", rec.Body.String())
	}
}

func TestWrapGlobalAppliesRootMiddlewares(t *testing.T) {
	mux := http.NewServeMux()
	root := New(mux)
	root.Use(writeBeforeMiddleware("mw;"))

	h := root.wrapGlobal(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("end"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Body.String() != "mw;end" {
		t.Fatalf("expected mw;end, got %q", rec.Body.String())
	}
}

func TestStatusRecorder(t *testing.T) {
	rec := &statusRecorder{}
	if rec.Header() == nil {
		t.Fatalf("expected non-nil header")
	}
	rec.WriteHeader(404)
	if rec.status != 404 {
		t.Fatalf("expected 404 recorded, got %d", rec.status)
	}
	if n, err := rec.Write([]byte("ignored")); n != 0 || err != nil {
		t.Fatalf("expected Write to return (0,nil), got (%d,%v)", n, err)
	}
}
