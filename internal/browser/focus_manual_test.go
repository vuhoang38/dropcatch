//go:build manual

// Run with: go test -tags manual -run TestLoadsDoNotStealFocus ./internal/browser/
//
// Needs Google Chrome and a Mac desktop session. It checks that page loads
// after Chrome's first launch never bring Chrome to the front.
package browser

import (
	"context"
	"os/exec"
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

	err := c.OpenVisible(ctx, "https://example.org/", func(u string) bool {
		return strings.Contains(u, "example.org")
	}, time.Minute)
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
	err := c.OpenVisible(ctx, "https://example.net/", func(u string) bool {
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
