// Package browser manages the Chrome process and CDP session.
package browser

import (
	"context"
	"encoding/json"
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

// New launches a Chrome session. If profileDir is empty a temp dir is created
// and cleaned up on Cancel. Pass a stable path to persist the profile — the
// dir survives the run, IndexedDB / localStorage / Service Workers stick
// around, and the next session reuses them. This is what session-auth needs
// for local-first apps where "log in once" isn't enough because the vault
// lives in IndexedDB.
func New(headless bool, profileDir string) (*Session, error) {
	persistent := profileDir != ""
	if !persistent {
		d, err := os.MkdirTemp("", "lookout_chrome_")
		if err != nil {
			return nil, fmt.Errorf("creating temp profile dir: %w", err)
		}
		profileDir = d
	} else if err := os.MkdirAll(profileDir, 0700); err != nil {
		return nil, fmt.Errorf("creating profile dir %s: %w", profileDir, err)
	}

	// Build options from scratch — chromedp.DefaultExecAllocatorOptions
	// includes chromedp.Headless, and there's no way to reliably remove it
	// from an option slice once set.
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.UserDataDir(profileDir),
		// Desktop viewport — without this chromedp defaults to a tiny
		// window that forces responsive/mobile layouts in most apps.
		chromedp.WindowSize(1440, 900),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("force-device-scale-factor", "1"),
	}
	if headless {
		opts = append(opts, chromedp.Headless)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	// Suppress noisy CDP protocol parse errors (e.g. cookiePartitionKey) that
	// chromedp logs when Chrome returns newer event payloads than the client
	// understands. These are harmless and distract users from real output.
	quiet := log.New(io.Discard, "", 0)
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(quiet.Printf))

	// Combined cancel: cancels both contexts. Only clean up the profile dir
	// if we created it; leave a user-supplied one alone so IndexedDB etc.
	// persist for the next run.
	combinedCancel := func() {
		cancel()
		allocCancel()
		if !persistent {
			os.RemoveAll(profileDir)
		}
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
// Supports three selector flavours so specs converted from Playwright / Cypress
// don't all need rewriting:
//
//   - plain CSS:        "button.primary"
//   - Playwright text:  "text=Sign in"            → first visible element whose
//                                                    text contains "Sign in"
//   - Playwright has-text: 'button:has-text("Save")' → first <button> whose
//                                                    text contains "Save"
//   - comma-separated list: any of the above; first match wins. Lists are
//     split on commas that sit outside parens/quotes so the inside of
//     :has-text("a, b") stays intact.
func (s *Session) Click(selector string) error {
	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()

	// Fast path: pure CSS, no Playwright extensions. Use chromedp directly so
	// behaviour on existing specs is unchanged.
	if !needsTextMatching(selector) {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Click(selector, chromedp.ByQuery),
		)
	}

	// Slow path: try each comma-split clause until one clicks.
	for _, clause := range splitSelectorList(selector) {
		if err := clickSingle(ctx, clause); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no selector matched: %s", selector)
}

// needsTextMatching reports whether the selector uses any Playwright-flavoured
// text extension that plain chromedp/CSS can't handle.
func needsTextMatching(sel string) bool {
	return strings.Contains(sel, ":has-text(") || strings.HasPrefix(strings.TrimSpace(sel), "text=")
}

// clickSingle clicks one selector clause (no commas at the top level).
func clickSingle(ctx context.Context, sel string) error {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return fmt.Errorf("empty selector")
	}
	if strings.HasPrefix(sel, "text=") {
		return clickByText(ctx, strings.TrimPrefix(sel, "text="))
	}
	if strings.Contains(sel, ":has-text(") {
		return clickHasText(ctx, sel)
	}
	return chromedp.Run(ctx,
		chromedp.WaitVisible(sel, chromedp.ByQuery),
		chromedp.Click(sel, chromedp.ByQuery),
	)
}

// clickByText finds the innermost element whose visible text contains the
// given string and dispatches a real mouse click at its center.
// Matches Playwright's text= locator semantics closely enough for QA-spec
// conversion.
func clickByText(ctx context.Context, text string) error {
	text = strings.Trim(text, `"`)
	js := fmt.Sprintf(`
(function() {
  var needle = %q;
  var xp = "//*[not(self::script)][not(self::style)][contains(normalize-space(.), " + JSON.stringify(needle) + ")][not(.//*[contains(normalize-space(.), " + JSON.stringify(needle) + ")])]";
  var it = document.evaluate(xp, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
  var el = it.singleNodeValue;
  if (!el) return "";
  var r = el.getBoundingClientRect();
  if (r.width === 0 || r.height === 0) return "";
  el.scrollIntoView({block: "center", behavior: "instant"});
  r = el.getBoundingClientRect();
  return JSON.stringify({x: r.left + r.width/2, y: r.top + r.height/2});
})()`, text)
	return clickAtEvaluatedPoint(ctx, js, fmt.Sprintf("text= did not match: %s", text))
}

// clickHasText handles selectors of the form "tag[attrs]:has-text(\"...\")".
// Queries the CSS part, filters by textContent, dispatches a real mouse click
// at the first match's center.
func clickHasText(ctx context.Context, sel string) error {
	i := strings.Index(sel, ":has-text(")
	if i < 0 {
		return fmt.Errorf("not a :has-text() selector: %s", sel)
	}
	css := strings.TrimSpace(sel[:i])
	rest := sel[i+len(":has-text("):]
	end := strings.LastIndex(rest, ")")
	if end < 0 {
		return fmt.Errorf("unterminated :has-text() in %s", sel)
	}
	text := strings.Trim(strings.TrimSpace(rest[:end]), `"'`)
	if css == "" {
		css = "*"
	}
	js := fmt.Sprintf(`
(function() {
  var nodes = document.querySelectorAll(%q);
  for (var i = 0; i < nodes.length; i++) {
    if (nodes[i].textContent && nodes[i].textContent.indexOf(%q) !== -1) {
      var r = nodes[i].getBoundingClientRect();
      if (r.width === 0 || r.height === 0) continue;
      nodes[i].scrollIntoView({block: "center", behavior: "instant"});
      r = nodes[i].getBoundingClientRect();
      return JSON.stringify({x: r.left + r.width/2, y: r.top + r.height/2});
    }
  }
  return "";
})()`, css, text)
	return clickAtEvaluatedPoint(ctx, js, fmt.Sprintf(":has-text() did not match: %s", sel))
}

// clickAtEvaluatedPoint runs the JS, expects it to return a JSON {"x":n,"y":n}
// locating the element's click target, then dispatches a real chromedp mouse
// click at those coordinates so React / framework event handlers fire the
// same as they would for a human click.
func clickAtEvaluatedPoint(ctx context.Context, js, missErr string) error {
	// Give the element a moment to render if it just appeared. Poll up to 5s
	// because React-heavy SPAs sometimes mount controls asynchronously after
	// the initial DOMContentLoaded.
	var coords string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &coords)); err != nil {
			return err
		}
		if coords != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if coords == "" {
		return fmt.Errorf("%s", missErr)
	}
	var pt struct {
		X, Y float64
	}
	if err := json.Unmarshal([]byte(coords), &pt); err != nil {
		return fmt.Errorf("parsing click coords: %w", err)
	}
	return chromedp.Run(ctx, chromedp.MouseClickXY(pt.X, pt.Y))
}

// splitSelectorList splits a comma-separated selector list, respecting paren
// and quote nesting so :has-text("a, b") stays a single clause.
func splitSelectorList(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == '(' || c == '[':
			depth++
			cur.WriteByte(c)
		case c == ')' || c == ']':
			depth--
			cur.WriteByte(c)
		case c == ',' && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
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
			"  Ubuntu/Debian:  sudo apt install chromium\n" +
			"  Arch:           sudo pacman -S chromium\n" +
			"  Fedora:         sudo dnf install chromium\n" +
			"  macOS:          brew install --cask chromium\n" +
			"Or: lookout install-browsers",
	)
}
