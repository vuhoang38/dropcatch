//go:build manual

// Run with: go test -tags manual -run TestLoadsDoNotStealFocus ./internal/browser/
//
// Needs Google Chrome and a Mac desktop session. It checks that page loads
// after Chrome's first launch never bring Chrome to the front.
package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func frontPID(t *testing.T) int {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get unix id of first application process whose frontmost is true`).Output()
	if err != nil {
		t.Fatalf("osascript: %v", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return pid
}

func activate(pid int) {
	_ = exec.Command("osascript", "-e",
		`tell application "System Events" to set frontmost of (first process whose unix id is `+strconv.Itoa(pid)+`) to true`).Run()
}

// chromePIDs lists the main Chrome processes using profile, leaving out the
// helper processes Chrome starts and stops for each page.
func chromePIDs(profile string) map[int]bool {
	out, _ := exec.Command("ps", "-axo", "pid=,command=").Output()
	pids := map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "user-data-dir="+profile) || strings.Contains(line, "--type=") {
			continue
		}
		if n, err := strconv.Atoi(strings.Fields(line)[0]); err == nil {
			pids[n] = true
		}
	}
	return pids
}

func TestLoadsDoNotStealFocus(t *testing.T) {
	profile := t.TempDir()
	c := New(Config{ProfileDir: profile, Settle: 2 * time.Second})
	defer c.Close()
	ctx := context.Background()
	before := frontPID(t)

	// The first load launches Chrome, which may take focus once.
	if _, err := c.Load(ctx, "https://example.com/", nil); err != nil {
		t.Fatal(err)
	}
	activate(before)
	time.Sleep(time.Second)

	var (
		mu    sync.Mutex
		stole []int
		stop  = make(chan struct{})
		done  = make(chan struct{})
	)
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-time.After(300 * time.Millisecond):
			}
			if pid := frontPID(t); chromePIDs(profile)[pid] {
				mu.Lock()
				stole = append(stole, pid)
				mu.Unlock()
			}
		}
	}()
	for _, u := range []string{"https://example.org/", "https://example.net/"} {
		if _, err := c.Load(ctx, u, nil); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	<-done
	if len(stole) > 0 {
		t.Fatalf("Chrome came to the front %d times during later loads", len(stole))
	}
}

func TestOpenVisibleReusesChrome(t *testing.T) {
	profile := t.TempDir()
	c := New(Config{ProfileDir: profile, Settle: time.Second})
	defer c.Close()
	ctx := context.Background()
	if _, err := c.Load(ctx, "https://example.com/", nil); err != nil {
		t.Fatal(err)
	}
	before := chromePIDs(profile)
	if len(before) != 1 {
		t.Fatalf("want one main Chrome process, got %v", before)
	}

	err := c.OpenVisible(ctx, "https://example.org/", func(map[string]string) bool { return true }, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Load(ctx, "https://example.net/", nil); err != nil {
		t.Fatal(err)
	}
	after := chromePIDs(profile)
	for pid := range after {
		if !before[pid] {
			t.Fatalf("Chrome relaunched: before %v after %v", before, after)
		}
	}
}

// windowsVisible reports how many of Chrome's windows are on screen and not
// minimized.
func windowsVisible(t *testing.T, profile string) int {
	visible := 0
	for pid := range chromePIDs(profile) {
		out, err := exec.Command("osascript", "-e",
			`tell application "System Events" to tell (first process whose unix id is `+strconv.Itoa(pid)+`) to get value of attribute "AXMinimized" of every window`).Output()
		if err != nil {
			t.Fatalf("osascript: %v", err)
		}
		for _, v := range strings.Split(strings.TrimSpace(string(out)), ",") {
			if strings.TrimSpace(v) == "false" {
				visible++
			}
		}
	}
	return visible
}

func TestWindowStaysHidden(t *testing.T) {
	profile := t.TempDir()
	c := New(Config{ProfileDir: profile, Settle: time.Second})
	defer c.Close()
	ctx := context.Background()
	for _, u := range []string{"https://example.com/", "https://example.org/"} {
		if _, err := c.Load(ctx, u, nil); err != nil {
			t.Fatal(err)
		}
		if n := windowsVisible(t, profile); n > 0 {
			t.Fatalf("after loading %s, %d Chrome window(s) are on screen", u, n)
		}
	}

	// The login window shows, then hides again.
	err := c.OpenVisible(ctx, "https://example.net/", func(map[string]string) bool {
		if n := windowsVisible(t, profile); n != 1 {
			t.Errorf("login window: %d visible, want 1", n)
		}
		return true
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if n := windowsVisible(t, profile); n > 0 {
		t.Fatalf("after login, %d Chrome window(s) are on screen", n)
	}
}

// closeAllTabs closes every page the way you would by closing the window.
func closeAllTabs(t *testing.T, profile string) {
	b, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
	if err != nil {
		t.Fatal(err)
	}
	port := strings.SplitN(string(b), "\n", 2)[0]
	resp, err := http.Get("http://127.0.0.1:" + port + "/json/list")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var targets []struct{ ID, Type string }
	_ = json.NewDecoder(resp.Body).Decode(&targets)
	for _, tg := range targets {
		if tg.Type == "page" {
			r, err := http.Get("http://127.0.0.1:" + port + "/json/close/" + tg.ID)
			if err == nil {
				r.Body.Close()
			}
		}
	}
}

func TestOpenVisibleReturnsWhenWindowClosed(t *testing.T) {
	profile := t.TempDir()
	c := New(Config{ProfileDir: profile, Settle: time.Second})
	defer c.Close()
	ctx := context.Background()
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(4 * time.Second)
		closeAllTabs(t, profile)
	}()
	start := time.Now()
	if err := c.OpenVisible(ctx, "https://example.org/", nil, time.Minute); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Fatalf("OpenVisible took %s after the window closed", took)
	}
	// Chrome comes back hidden and still works.
	if _, err := c.Load(ctx, "https://example.net/", nil); err != nil {
		t.Fatal(err)
	}
	if n := windowsVisible(t, profile); n > 0 {
		t.Fatalf("%d Chrome window(s) on screen after relaunch", n)
	}
}

// A login page that is busy (here, showing a dialog that blocks its scripts)
// must not be mistaken for a closed window.
func TestOpenVisibleSurvivesBusyPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>login<script>setTimeout(() => alert("verify"), 500)</script></body></html>`))
	}))
	defer srv.Close()

	profile := t.TempDir()
	c := New(Config{ProfileDir: profile, Settle: time.Second})
	defer c.Close()
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	before := chromePIDs(profile)

	start := time.Now()
	err := c.OpenVisible(context.Background(), srv.URL, nil, 25*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 24*time.Second {
		t.Fatalf("gave up on an open, busy login window after %s", took)
	}
	for pid := range chromePIDs(profile) {
		if !before[pid] {
			t.Fatal("Chrome was relaunched, losing the login page")
		}
	}
}

// Signing in with Google opens a popup and takes a while. The window must
// wait for the login cookie, then close the popup and hide.
func TestOpenVisibleWaitsForLoginCookieAndClosesPopups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/popup" {
			w.Write([]byte(`<html><body>sign in with google</body></html>`))
			return
		}
		w.Write([]byte(`<html><body>login<script>
			window.open("/popup", "google", "width=400,height=400");
			setTimeout(() => { document.cookie = "testuid=1234567890; path=/" }, 8000);
		</script></body></html>`))
	}))
	defer srv.Close()

	profile := t.TempDir()
	c := New(Config{ProfileDir: profile, Settle: time.Second})
	defer c.Close()
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := c.OpenVisible(context.Background(), srv.URL, func(cookies map[string]string) bool {
		return cookies["testuid"] == "1234567890"
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	if took < 8*time.Second {
		t.Fatalf("hid the window after %s, before the login finished", took)
	}
	if took > 30*time.Second {
		t.Fatalf("took %s to notice the login cookie", took)
	}
	if n := windowsVisible(t, profile); n > 0 {
		t.Fatalf("%d Chrome window(s) still on screen after login", n)
	}
}
