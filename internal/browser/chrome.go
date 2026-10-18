// Package browser drives your installed Chrome through a dedicated profile.
package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
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

// Chrome runs one Chrome process at a time, since a profile can't be opened
// twice. Every call launches Chrome and closes it when done.
type Chrome struct {
	cfg Config
	mu  sync.Mutex
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

func (c *Chrome) launch(ctx context.Context, offscreen bool) (context.Context, context.CancelFunc) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(c.cfg.ChromePath),
		chromedp.UserDataDir(c.cfg.ProfileDir),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(1280, 900),
	}
	if offscreen {
		opts = append(opts, chromedp.Flag("window-position", "-2400,-2400"))
	}
	actx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	bctx, cancelBrowser := chromedp.NewContext(actx, chromedp.WithErrorf(quietErrorf))
	return bctx, func() {
		// Cancel closes Chrome cleanly so it writes cookies back to the profile.
		_ = chromedp.Cancel(bctx)
		cancelBrowser()
		cancelAlloc()
	}
}

// Load opens url offscreen and returns what the page showed.
func (c *Chrome) Load(ctx context.Context, url string, capture []string) (reader.Page, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// A page whose load event never fires would otherwise hold the browser,
	// and every check behind it, forever.
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	bctx, done := c.launch(ctx, true)
	defer done()

	var (
		hitsMu sync.Mutex
		hits   = map[network.RequestID]string{}
	)
	chromedp.ListenTarget(bctx, func(ev any) {
		e, ok := ev.(*network.EventResponseReceived)
		if !ok {
			return
		}
		for _, sub := range capture {
			if strings.Contains(e.Response.URL, sub) {
				hitsMu.Lock()
				hits[e.RequestID] = e.Response.URL
				hitsMu.Unlock()
				return
			}
		}
	})

	page := reader.Page{Responses: map[string][]byte{}}
	err := chromedp.Run(bctx,
		network.Enable(),
		chromedp.Navigate(url),
		chromedp.Sleep(c.cfg.Settle),
		chromedp.Location(&page.FinalURL),
		chromedp.OuterHTML("html", &page.HTML, chromedp.ByQuery),
	)
	if err != nil {
		if ctx.Err() != nil {
			return page, fmt.Errorf("page took longer than %s to load", c.cfg.Timeout)
		}
		return page, err
	}

	hitsMu.Lock()
	defer hitsMu.Unlock()
	for id, u := range hits {
		var body []byte
		err := chromedp.Run(bctx, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			body, err = network.GetResponseBody(id).Do(ctx)
			return err
		}))
		if err == nil {
			page.Responses[u] = body
		}
	}
	return page, nil
}

// OpenVisible shows a normal Chrome window at url so you can log in or solve
// a captcha. It returns when done reports true for the current URL, when you
// close the window, or after timeout.
func (c *Chrome) OpenVisible(ctx context.Context, url string, done func(currentURL string) bool, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	bctx, closeChrome := c.launch(ctx, false)
	defer closeChrome()

	if err := chromedp.Run(bctx, chromedp.Navigate(url)); err != nil {
		return err
	}
	for {
		select {
		case <-bctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return nil // window closed
		case <-time.After(2 * time.Second):
		}
		var cur string
		if err := chromedp.Run(bctx, chromedp.Location(&cur)); err != nil {
			return nil // window closed
		}
		if done != nil && done(cur) {
			time.Sleep(3 * time.Second) // let cookies settle
			return nil
		}
	}
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
