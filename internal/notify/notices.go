package notify

import (
	"context"
	"time"

	"dropcatch/internal/store"
)

// Notices sends each failure email once, and again only after the problem
// cleared and came back. Keys: "session:<site>", "link:<id>", "stale".
type Notices struct {
	Store  *store.Store
	Mailer Mailer
	To     func(ctx context.Context) (string, error)
	Now    func() time.Time
}

func (n *Notices) Raise(ctx context.Context, key, subject, text string) error {
	active, err := n.Store.NoticeActive(ctx, key)
	if err != nil || active {
		return err
	}
	to, err := n.To(ctx)
	if err != nil {
		return err
	}
	// Mark it only once the email went out, so a failed send retries.
	if err := n.Mailer.Send(ctx, to, subject, text); err != nil {
		return err
	}
	return n.Store.SetNotice(ctx, key, true, n.Now())
}

func (n *Notices) Clear(ctx context.Context, key string) error {
	active, err := n.Store.NoticeActive(ctx, key)
	if err != nil || !active {
		return err
	}
	return n.Store.SetNotice(ctx, key, false, n.Now())
}
