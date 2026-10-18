package server

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"dropcatch/internal/notify"
	"dropcatch/internal/store"
)

const maskPrefix = "••••"

func mask(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return maskPrefix
	}
	return maskPrefix + key[len(key)-4:]
}

func (s *server) getSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.GetSettings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	st.ResendAPIKey = mask(st.ResendAPIKey)
	writeJSON(w, http.StatusOK, st)
}

func (s *server) putSettings(w http.ResponseWriter, r *http.Request) {
	var in store.Settings
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "Bad request body")
		return
	}
	switch {
	case in.DropPct <= 0 || in.DropPct >= 100:
		writeError(w, http.StatusBadRequest, "Drop percent must be between 0 and 100")
		return
	case in.WindowDays < 1:
		writeError(w, http.StatusBadRequest, "Window must be at least 1 day")
		return
	case in.MinHistoryDays < 0:
		writeError(w, http.StatusBadRequest, "Minimum history can't be negative")
		return
	case in.CheckIntervalHours < 1:
		writeError(w, http.StatusBadRequest, "Check interval must be at least 1 hour")
		return
	}
	old, err := s.Store.GetSettings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	// The dashboard only ever sees the masked key, so sending it back means
	// "keep what's stored".
	if strings.HasPrefix(in.ResendAPIKey, maskPrefix) {
		in.ResendAPIKey = old.ResendAPIKey
	}
	in.ResendAPIKey = strings.TrimSpace(in.ResendAPIKey)
	in.Email = strings.TrimSpace(in.Email)
	if err := s.Store.PutSettings(r.Context(), in); err != nil {
		fail(w, err)
		return
	}
	in.ResendAPIKey = mask(in.ResendAPIKey)
	writeJSON(w, http.StatusOK, in)
}

func (s *server) testEmail(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.GetSettings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	subject, text := notify.TestEmail()
	if err := s.Mailer.Send(r.Context(), st.Email, subject, text); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type Session struct {
	Site   string `json:"site"`
	Status string `json:"status"`
}

// Sites that need your login to show voucher prices.
var loginURLs = map[string]string{
	"shopee": "https://shopee.sg/buyer/login",
	"lazada": "https://member.lazada.sg/user/login",
}

// loginDone tells, from the page address, that you finished logging in to
// site: you're on that site's pages and no longer on its login, sign up or
// verification pages.
func loginDone(site string) func(currentURL string) bool {
	return func(raw string) bool {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return false
		}
		switch site {
		case "shopee":
			return strings.HasSuffix(u.Host, "shopee.sg") &&
				!strings.HasPrefix(u.Path, "/buyer/") && !strings.HasPrefix(u.Path, "/verify/")
		case "lazada":
			return strings.HasSuffix(u.Host, "lazada.sg") &&
				u.Path != "/user/login" && u.Path != "/user/register" && !strings.Contains(u.Path, "punish")
		}
		return false
	}
}

func (s *server) sessions(w http.ResponseWriter, r *http.Request) {
	out := []Session{}
	for _, site := range []string{"shopee", "lazada"} {
		st, err := s.Store.GetSession(r.Context(), site)
		if err != nil {
			fail(w, err)
			return
		}
		out = append(out, Session{Site: site, Status: string(st)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	loginURL, ok := loginURLs[site]
	if !ok || s.Login == nil {
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	if !s.loginOpen.CompareAndSwap(false, true) {
		writeError(w, http.StatusConflict, "A login window is already open")
		return
	}
	go func() {
		defer s.loginOpen.Store(false)
		var loggedIn atomic.Bool
		isDone := loginDone(site)
		done := func(u string) bool {
			if isDone(u) {
				loggedIn.Store(true)
				return true
			}
			return false
		}
		// Hides again once you're logged in or close the window, up to 10 minutes.
		if err := s.Login.OpenVisible(context.Background(), loginURL, done, 10*time.Minute); err != nil {
			log.Printf("login window: %v", err)
		}
		st := store.SessionUnknown // you closed the window; the next check will tell
		if loggedIn.Load() {
			st = store.SessionOK
		}
		_ = s.Store.SetSession(context.Background(), site, st)
	}()
	w.WriteHeader(http.StatusAccepted)
}
