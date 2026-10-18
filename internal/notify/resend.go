// Package notify sends email through Resend.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, text string) error
}

// From is Resend's shared sender. It can only deliver to the address your
// Resend account was created with, which is all a personal tracker needs.
const From = "Dropcatch <onboarding@resend.dev>"

type Resend struct {
	APIKey  string
	BaseURL string // default https://api.resend.com
	HTTP    *http.Client
}

func (r *Resend) Send(ctx context.Context, to, subject, text string) error {
	if r.APIKey == "" {
		return errors.New("no Resend API key set")
	}
	if to == "" {
		return errors.New("no email address set")
	}
	base, client := r.BaseURL, r.HTTP
	if base == "" {
		base = "https://api.resend.com"
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	body, _ := json.Marshal(map[string]any{"from": From, "to": []string{to}, "subject": subject, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return fmt.Errorf("resend: %s: %s", resp.Status, msg)
	}
	return nil
}
