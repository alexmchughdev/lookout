// Package browser manages the Chrome process and CDP session.
package browser

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Session wraps a chromedp context and the temp profile directory.
type Session struct {
	Ctx        context.Context
	Cancel     context.CancelFunc
	profileDir string
}

// New launches a headless Chrome session.
func New(headless bool) (*Session, error) {
	profileDir, err := os.MkdirTemp("", "lookout_chrome_")
	if err != nil {
		return nil, fmt.Errorf("creating temp profile dir: %w", err)
	}

	opts := chromedp.DefaultExecAllocatorOptions[:]
	opts = append(opts,
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.NoSandbox,
	)

	if headless {
		opts = append(opts, chromedp.Headless)
	} else {
		// Remove the headless flag when running headed
		filtered := opts[:0]
		for _, o := range opts {
			filtered = append(filtered, o)
		}
		opts = filtered
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	// Suppress noisy CDP protocol parse errors (e.g. cookiePartitionKey) that
	// chromedp logs when Chrome returns newer event payloads than the client
	// understands. These are harmless and distract users from real output.
	quiet := log.New(io.Discard, "", 0)
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(quiet.Printf))

	// Combined cancel: cancels both contexts
	combinedCancel := func() {
		cancel()
		allocCancel()
		os.RemoveAll(profileDir)
	}

	// Warm up the browser
	if err := chromedp.Run(ctx); err != nil {
		combinedCancel()
		return nil, fmt.Errorf("starting Chrome: %w", err)
	}

	return &Session{
		Ctx:        ctx,
		Cancel:     combinedCancel,
		profileDir: profileDir,
	}, nil
}

// Navigate goes to a URL and waits for the page to be ready.
func (s *Session) Navigate(url string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 20*time.Second)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
	)
}

// Screenshot captures the visible viewport as PNG bytes.
func (s *Session) Screenshot() ([]byte, error) {
	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return nil, fmt.Errorf("capturing screenshot: %w", err)
	}
	return buf, nil
}

// FullPageScreenshot captures the entire scrollable page as PNG bytes.
func (s *Session) FullPageScreenshot() ([]byte, error) {
	ctx, cancel := context.WithTimeout(s.Ctx, 20*time.Second)
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.FullScreenshot(&buf, 90),
	); err != nil {
		return nil, fmt.Errorf("capturing full-page screenshot: %w", err)
	}
	return buf, nil
}

// WaitForSelector blocks until the selector is visible, or timeout.
func (s *Session) WaitForSelector(selector string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(s.Ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.WaitVisible(selector, chromedp.ByQuery))
}

// CurrentURL returns the current page URL.
func (s *Session) CurrentURL() (string, error) {
	ctx, cancel := context.WithTimeout(s.Ctx, 5*time.Second)
	defer cancel()

	var u string
	if err := chromedp.Run(ctx, chromedp.Location(&u)); err != nil {
		return "", err
	}
	return u, nil
}

// WaitForURLChange waits until the URL no longer contains the given substring.
func (s *Session) WaitForURLExcludes(substr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		u, err := s.CurrentURL()
		if err == nil && !strings.Contains(u, substr) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("URL still contains %q after %s", substr, timeout)
}

// Fill fills an input element matching the selector.
func (s *Session) Fill(selector, value string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Clear(selector, chromedp.ByQuery),
		chromedp.SendKeys(selector, value, chromedp.ByQuery),
	)
}

// Click clicks the first element matching the selector.
func (s *Session) Click(selector string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// ClickIfExists clicks the selector if it exists, ignoring errors.
func (s *Session) ClickIfExists(selector string) {
	ctx, cancel := context.WithTimeout(s.Ctx, 3*time.Second)
	defer cancel()
	_ = chromedp.Run(ctx,
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// Eval runs JavaScript in the page context.
func (s *Session) Eval(js string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 5*time.Second)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.Evaluate(js, nil),
	)
}

// SendKeys types text into the focused element.
func (s *Session) SendKeys(selector, keys string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.SendKeys(selector, keys, chromedp.ByQuery),
	)
}

// Sleep waits for the given duration.
func (s *Session) Sleep(d time.Duration) {
	time.Sleep(d)
}

// Reload reloads the current page.
func (s *Session) Reload() error {
	ctx, cancel := context.WithTimeout(s.Ctx, 15*time.Second)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.Reload(),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
	)
}

// DragAndDrop performs a React DnD compatible drag operation.
// Uses mouse events with a hold delay to allow React to hydrate.
func (s *Session) DragAndDrop(sourceSelector, targetSelector string, holdMs int) error {
	if holdMs <= 0 {
		holdMs = 500
	}

	ctx, cancel := context.WithTimeout(s.Ctx, 15*time.Second)
	defer cancel()

	var pos map[string]float64
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(sourceSelector, chromedp.ByQuery),
		chromedp.EvaluateAsDevTools(fmt.Sprintf(`
			(function() {
				const src = document.querySelector(%q);
				const tgt = document.querySelector(%q);
				if (!src || !tgt) return null;
				const sr = src.getBoundingClientRect();
				const tr = tgt.getBoundingClientRect();
				return {
					sx: sr.left + sr.width/2,
					sy: sr.top + sr.height/2,
					tx: tr.left + tr.width/2,
					ty: tr.top + 50
				};
			})()`, sourceSelector, targetSelector), &pos),
	); err != nil {
		return err
	}

	if pos == nil {
		return fmt.Errorf("source or target element not found")
	}

	srcX, srcY := pos["sx"], pos["sy"]
	tgtX, tgtY := pos["tx"], pos["ty"]

	dragJS := fmt.Sprintf(`
		(function() {
			function dispatch(el, type, x, y, buttons) {
				el.dispatchEvent(new MouseEvent(type, {
					bubbles: true, cancelable: true,
					clientX: x, clientY: y, buttons: buttons
				}));
			}
			const src = document.elementFromPoint(%f, %f);
			const tgt = document.elementFromPoint(%f, %f);
			if (!src || !tgt) return false;
			dispatch(src, 'mousedown', %f, %f, 1);
			setTimeout(() => {
				dispatch(src, 'mousemove', %f, %f, 1);
				dispatch(tgt, 'mousemove', %f, %f, 1);
				dispatch(tgt, 'mouseup',   %f, %f, 0);
				dispatch(tgt, 'drop',      %f, %f, 0);
			}, %d);
			return true;
		})()`,
		srcX, srcY, tgtX, tgtY,
		srcX, srcY,
		(srcX+tgtX)/2, (srcY+tgtY)/2,
		tgtX, tgtY,
		tgtX, tgtY,
		tgtX, tgtY,
		holdMs,
	)

	return chromedp.Run(ctx, chromedp.Evaluate(dragJS, nil))
}

// FindChrome returns the path to a Chrome/Chromium binary.
// chromedp finds it automatically, but this is useful for validation.
func FindChrome() (string, error) {
	candidates := []string{
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
	}

	// Playwright bundled Chromium
	home, _ := os.UserHomeDir()
	pattern := filepath.Join(home, ".cache", "ms-playwright", "chromium-*", "chrome-linux", "chrome")
	if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
		candidates = append([]string{matches[0]}, candidates...)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf(
		"no Chrome/Chromium binary found\n" +
			"Install Chromium: sudo apt install chromium-browser\n" +
			"Or: lookout install-browsers",
	)
}
