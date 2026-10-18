// Package browser drives your installed Chrome through a dedicated profile.
//
// On macOS, launching Chrome or opening a window or tab brings Chrome to the
// front, which takes your keyboard away mid sentence. Loading a new page in a
// tab that already exists doesn't. So Chrome starts once and every page load
// reuses its one tab.
//
// macOS also ignores --window-position offscreen and opens the window at the
// top left of your screen, and won't let a window move fully offscreen. So
// the window is minimized to the Dock right after launch instead, with
// Chrome's background throttling off so pages load the same.
//
// Checked by hand on 2026-10-18, see focus_manual_test.go.
package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"dropcatch/internal/reader"
)

const DefaultChromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

type Config struct {
	ChromePath string
	ProfileDir string
	Settle     time.Duration // wait after navigation before reading; default 6s
	Timeout    time.Duration // give up on one page load after this; default 90s
}

// Chrome keeps one Chrome process and one tab alive, and runs one page load
// or login window at a time, since a profile can't be opened twice.
type Chrome struct {
	cfg Config
	mu  sync.Mutex // one Load or OpenVisible at a time

	tab  context.Context // the one tab; nil until started
	stop func()          // shuts Chrome down

	capMu   sync.Mutex
	capture []string
	hits    map[network.RequestID]string
}

func New(cfg Config) *Chrome {
	if cfg.ChromePath == "" {
		cfg.ChromePath = DefaultChromePath
	}
	if cfg.Settle == 0 {
		cfg.Settle = 6 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 90 * time.Second
	}
	return &Chrome{cfg: cfg}
}

// Start launches Chrome now, so the one moment it takes focus is when
// Dropcatch starts rather than in the middle of your work.
func (c *Chrome) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ensure()
}

// Close shuts Chrome down.
func (c *Chrome) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown()
}

func (c *Chrome) shutdown() {
	if c.stop != nil {
		c.stop()
	}
	c.tab, c.stop = nil, nil
}

// ensure makes sure Chrome is running and its tab answers, relaunching it if
// it crashed or you closed the window.
func (c *Chrome) ensure() error {
	if c.tab != nil && c.tab.Err() == nil {
		ping, cancel := context.WithTimeout(c.tab, 5*time.Second)
		var u string
		err := chromedp.Run(ping, chromedp.Location(&u))
		cancel()
		if err == nil {
			return nil
		}
	}
	c.shutdown()

	opts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(c.cfg.ChromePath),
		chromedp.UserDataDir(c.cfg.ProfileDir),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(1280, 900),
		// A minimized window counts as hidden, and Chrome would otherwise
		// slow its timers and rendering.
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
	}
	actx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	// The first context on a fresh browser attaches to its startup tab, so
	// no second window opens.
	tab, cancelTab := chromedp.NewContext(actx, chromedp.WithErrorf(quietErrorf))
	stop := func() {
		// Cancel closes Chrome cleanly so it writes cookies back to the profile.
		_ = chromedp.Cancel(tab)
		cancelTab()
		cancelAlloc()
	}
	chromedp.ListenTarget(tab, c.onEvent)
	if err := chromedp.Run(tab, network.Enable()); err != nil {
		stop()
		return fmt.Errorf("start chrome: %w", err)
	}
	if err := showWindow(tab, false); err != nil {
		stop()
		return fmt.Errorf("hide chrome window: %w", err)
	}
	c.tab, c.stop = tab, stop
	return nil
}

func (c *Chrome) onEvent(ev any) {
	e, ok := ev.(*network.EventResponseReceived)
	if !ok {
		return
	}
	c.capMu.Lock()
	defer c.capMu.Unlock()
	for _, sub := range c.capture {
		if strings.Contains(e.Response.URL, sub) {
			c.hits[e.RequestID] = e.Response.URL
			return
		}
	}
}

func (c *Chrome) setCapture(subs []string) {
	c.capMu.Lock()
	c.capture, c.hits = subs, map[network.RequestID]string{}
	c.capMu.Unlock()
}

// Load opens url in the background tab and returns what the page showed.
func (c *Chrome) Load(ctx context.Context, url string, capture []string) (reader.Page, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensure(); err != nil {
		return reader.Page{}, err
	}

	c.setCapture(capture)
	defer c.setCapture(nil)

	// A page whose load event never fires would otherwise hold the browser,
	// and every check behind it, forever.
	run, cancel := context.WithTimeout(c.tab, c.cfg.Timeout)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()

	page := reader.Page{Responses: map[string][]byte{}}
	err := chromedp.Run(run,
		chromedp.Navigate(url),
		chromedp.Sleep(c.cfg.Settle),
		chromedp.Location(&page.FinalURL),
		chromedp.OuterHTML("html", &page.HTML, chromedp.ByQuery),
	)
	if err != nil {
		switch {
		case errors.Is(run.Err(), context.DeadlineExceeded):
			err = fmt.Errorf("page took longer than %s to load", c.cfg.Timeout)
		case ctx.Err() != nil:
			err = ctx.Err()
		default:
			c.shutdown() // the browser is in a bad state; relaunch next time
			return page, err
		}
		c.blank()
		return page, err
	}

	c.capMu.Lock()
	hits := c.hits
	c.capMu.Unlock()
	for id, u := range hits {
		var body []byte
		err := chromedp.Run(run, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			body, err = network.GetResponseBody(id).Do(ctx)
			return err
		}))
		if err == nil {
			page.Responses[u] = body
		}
	}
	c.blank()
	return page, nil
}

// blank leaves the tab on an empty page, so a shop's scripts don't keep
// running between checks.
func (c *Chrome) blank() {
	ctx, cancel := context.WithTimeout(c.tab, 10*time.Second)
	defer cancel()
	_ = chromedp.Run(ctx, chromedp.Navigate("about:blank"))
}

// showWindow puts the window on screen and in front, or minimizes it.
func showWindow(ctx context.Context, show bool) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		id, _, err := cdpbrowser.GetWindowForTarget().Do(ctx)
		if err != nil {
			return err
		}
		if !show {
			return cdpbrowser.SetWindowBounds(id, &cdpbrowser.Bounds{WindowState: cdpbrowser.WindowStateMinimized}).Do(ctx)
		}
		// Leave minimized first. A state change can't carry a position.
		if err := cdpbrowser.SetWindowBounds(id, &cdpbrowser.Bounds{WindowState: cdpbrowser.WindowStateNormal}).Do(ctx); err != nil {
			return err
		}
		if err := cdpbrowser.SetWindowBounds(id, &cdpbrowser.Bounds{Left: 120, Top: 80, Width: 1280, Height: 900}).Do(ctx); err != nil {
			return err
		}
		return page.BringToFront().Do(ctx)
	}))
}

// OpenVisible shows the Chrome window at url so you can log in or solve a
// captcha. It returns when done reports true for the current URL, when you
// close the window, or after timeout. Then the window is minimized again.
func (c *Chrome) OpenVisible(ctx context.Context, url string, done func(currentURL string) bool, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensure(); err != nil {
		return err
	}

	run, cancel := context.WithTimeout(c.tab, timeout)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()

	if err := showWindow(run, true); err != nil {
		return err
	}
	if err := chromedp.Run(run, chromedp.Navigate(url)); err != nil {
		return err
	}
	closed := false
	for run.Err() == nil {
		time.Sleep(2 * time.Second)
		cur, open, err := c.tabState(run)
		if err != nil || !open {
			closed = true // you closed the window, or Chrome quit
			break
		}
		if done != nil && done(cur) {
			time.Sleep(3 * time.Second) // let cookies settle
			break
		}
	}

	if !closed && c.tab.Err() == nil {
		hide, cancelHide := context.WithTimeout(c.tab, 10*time.Second)
		err := showWindow(hide, false)
		cancelHide()
		if err == nil {
			c.blank()
			return nil
		}
	}
	// You closed the window. Relaunch now, while you're switching away
	// anyway, rather than taking focus during a later check.
	c.shutdown()
	return c.ensure()
}

// tabState asks Chrome itself, not the page, whether the tab still exists and
// which address it shows. A busy page, a redirect or a dialog can stall the
// page's scripts for many seconds, and that must not look like a closed
// window.
func (c *Chrome) tabState(ctx context.Context) (url string, open bool, err error) {
	id := chromedp.FromContext(c.tab).Target.TargetID
	ask, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	infos, err := chromedp.Targets(ask)
	if err != nil {
		if ctx.Err() != nil {
			return "", true, nil // our own deadline, not a closed window
		}
		return "", false, err
	}
	for _, t := range infos {
		if t.TargetID == id {
			return t.URL, true, nil
		}
	}
	return "", false, nil
}

// quietErrorf drops chromedp's complaints about event types newer Chrome
// versions send that its protocol package doesn't know yet. They're harmless.
func quietErrorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "could not unmarshal event") {
		return
	}
	log.Print(msg)
}
