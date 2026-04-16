// Session capture and restore — for apps behind MFA / SSO where the login
// flow can't be automated. The user logs in once via a headed browser;
// lookout serialises cookies + localStorage and replays them before runs.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

// SessionState is the JSON schema written to disk.
type SessionState struct {
	URL          string            `json:"url"`
	SavedAt      time.Time         `json:"saved_at"`
	Cookies      []SavedCookie     `json:"cookies"`
	LocalStorage map[string]string `json:"local_storage,omitempty"`
}

// SavedCookie is a plain-JSON representation of a browser cookie.
type SavedCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"http_only,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"same_site,omitempty"`
}

// CaptureSession snapshots cookies (all origins) and the app origin's
// localStorage to the given path, chmod 0600. Caller should have
// navigated the browser to appURL before calling.
func (s *Session) CaptureSession(appURL, path string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 20*time.Second)
	defer cancel()

	// storage.GetCookies returns cookies across all origins — needed because
	// SSO flows (Microsoft / Okta / Google) set cookies on the IdP domain in
	// addition to the app domain.
	var cookies []*network.Cookie
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = storage.GetCookies().Do(ctx)
		return err
	})); err != nil {
		return fmt.Errorf("reading cookies: %w", err)
	}

	// LocalStorage is per-origin; we capture whatever's on the current page.
	var localStorage map[string]string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`Object.assign({}, localStorage)`, &localStorage),
	); err != nil {
		// Non-fatal: some pages block localStorage access.
		localStorage = nil
	}

	state := SessionState{
		URL:          appURL,
		SavedAt:      time.Now().UTC(),
		Cookies:      make([]SavedCookie, 0, len(cookies)),
		LocalStorage: localStorage,
	}
	for _, c := range cookies {
		state.Cookies = append(state.Cookies, SavedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		})
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating session dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// RestoreSession loads cookies and localStorage from a session file.
// The browser must already be on the app origin (for localStorage writes).
func (s *Session) RestoreSession(path string) (*SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session file %s not found — run 'lookout auth' first", path)
		}
		return nil, fmt.Errorf("reading session file: %w", err)
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing session file: %w", err)
	}

	ctx, cancel := context.WithTimeout(s.Ctx, 20*time.Second)
	defer cancel()

	if len(state.Cookies) > 0 {
		params := make([]*network.CookieParam, 0, len(state.Cookies))
		for _, c := range state.Cookies {
			p := &network.CookieParam{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				HTTPOnly: c.HTTPOnly,
				Secure:   c.Secure,
			}
			if c.Expires > 0 {
				t := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
				p.Expires = &t
			}
			if c.SameSite != "" {
				p.SameSite = network.CookieSameSite(c.SameSite)
			}
			params = append(params, p)
		}
		if err := chromedp.Run(ctx, network.SetCookies(params)); err != nil {
			return nil, fmt.Errorf("restoring cookies: %w", err)
		}
	}

	if len(state.LocalStorage) > 0 {
		b, _ := json.Marshal(state.LocalStorage)
		js := fmt.Sprintf(
			`(function(items) { try { for (var k in items) localStorage.setItem(k, items[k]); } catch(e) {} })(%s)`,
			string(b),
		)
		_ = chromedp.Run(ctx, chromedp.Evaluate(js, nil))
	}

	return &state, nil
}
