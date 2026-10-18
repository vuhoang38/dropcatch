// Package server is the JSON API plus the built dashboard.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dropcatch/internal/checker"
	"dropcatch/internal/notify"
	"dropcatch/internal/reader"
	"dropcatch/internal/store"
)

// LoginOpener shows a visible Chrome window so you can log in.
type LoginOpener interface {
	OpenVisible(ctx context.Context, url string, done func(currentURL string) bool, timeout time.Duration) error
}

type Deps struct {
	Store   *store.Store
	Checker *checker.Checker
	Mailer  notify.Mailer
	Login   LoginOpener
	Now     func() time.Time
}

type server struct {
	Deps
	loginOpen atomic.Bool

	previewMu sync.Mutex
	previews  map[string]cachedPreview // by canonical URL
}

type cachedPreview struct {
	listing reader.Listing
	at      time.Time
}

func New(d Deps, static fs.FS) http.Handler {
	s := &server{Deps: d, previews: map[string]cachedPreview{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/products", s.listProducts)
	mux.HandleFunc("POST /api/products", s.createProduct)
	mux.HandleFunc("GET /api/products/{id}", s.getProduct)
	mux.HandleFunc("PATCH /api/products/{id}", s.patchProduct)
	mux.HandleFunc("DELETE /api/products/{id}", s.deleteProduct)
	mux.HandleFunc("GET /api/products/{id}/history", s.history)
	mux.HandleFunc("POST /api/products/{id}/check", s.checkNow)
	mux.HandleFunc("POST /api/products/{id}/links", s.addLink)
	mux.HandleFunc("DELETE /api/links/{id}", s.deleteLink)
	mux.HandleFunc("PATCH /api/variants/{id}", s.patchVariant)
	mux.HandleFunc("POST /api/preview", s.preview)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("POST /api/settings/test-email", s.testEmail)
	mux.HandleFunc("GET /api/sessions", s.sessions)
	mux.HandleFunc("POST /api/sessions/{site}/login", s.login)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "Not found")
	})
	mux.Handle("/", spa(static))
	return mux
}

// spa serves built files, and index.html for any other path so client side
// routes like /products/1 load the app.
func spa(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if st, err := fs.Stat(static, name); err == nil && !st.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		b, err := fs.ReadFile(static, "index.html")
		if err != nil {
			http.Error(w, "dashboard not built, run make web", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"message": msg})
}

// fail maps store and reader errors to a status code and a message you can act on.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Not found")
	case errors.Is(err, store.ErrDuplicate):
		writeError(w, http.StatusConflict, "This link is already tracked")
	case errors.Is(err, reader.ErrLoggedOut):
		writeError(w, http.StatusBadRequest, "You're logged out of this site. Log in again from Settings, then retry.")
	case errors.Is(err, reader.ErrBlocked):
		writeError(w, http.StatusBadRequest, "The site showed a verification check. Solve it from Settings, then retry.")
	case errors.Is(err, reader.ErrUnreadable), strings.HasPrefix(err.Error(), "unsupported site"):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		log.Printf("api: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, store.ErrNotFound
	}
	return id, nil
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}
