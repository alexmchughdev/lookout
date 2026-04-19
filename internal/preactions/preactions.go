// Package preactions implements the built-in pre-action library.
package preactions

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexmchughdev/lookout/internal/browser"
	"github.com/alexmchughdev/lookout/internal/config"
)

// Run executes a pre-action defined in the spec.
func Run(s *browser.Session, baseURL string, pa *config.PreAction) error {
	if pa == nil {
		return nil
	}

	switch pa.Type {
	case "click":
		return runClick(s, pa)
	case "type_and_verify":
		return runTypeAndVerify(s, baseURL, pa)
	case "new_item":
		return runNewItem(s, pa)
	case "open_first":
		return runOpenFirst(s, pa)
	case "drag":
		return runDrag(s, pa)
	case "select_option":
		return runSelectOption(s, pa)
	case "reload":
		return runReload(s, pa)
	case "wait":
		ms := pa.Ms
		if ms <= 0 {
			ms = 2000
		}
		s.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	default:
		return fmt.Errorf("unknown pre_action type: %q", pa.Type)
	}
}

// click — click an element and wait.
func runClick(s *browser.Session, pa *config.PreAction) error {
	if err := s.Click(pa.Selector); err != nil {
		return fmt.Errorf("click(%s): %w", pa.Selector, err)
	}
	ms := pa.WaitMs
	if ms <= 0 {
		ms = 1000
	}
	s.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}

// type_and_verify — click item, type text, blur to save, reload, reopen.
func runTypeAndVerify(s *browser.Session, baseURL string, pa *config.PreAction) error {
	// Click the item to open it
	if err := s.Click(pa.ClickSelector); err != nil {
		return fmt.Errorf("clicking item (%s): %w", pa.ClickSelector, err)
	}
	s.Sleep(1 * time.Second)

	// Click editor and type
	editorSel := pa.EditorSelector
	if editorSel == "" {
		editorSel = `[contenteditable="true"]`
	}
	if err := s.Click(editorSel); err != nil {
		return fmt.Errorf("clicking editor: %w", err)
	}

	// Move to end and type
	if err := s.Eval(`document.activeElement.blur()`); err == nil {
		s.Click(editorSel) // refocus
	}
	// chromedp's SendKeys translates printable runes into keystrokes; escape
	// sequences like Ctrl+End don't work as a prefix string, so just send
	// the literal text. The preceding click() places the caret at the end
	// of existing content for most rich-text editors.
	if err := s.SendKeys(editorSel, pa.Text); err != nil {
		return fmt.Errorf("typing into editor: %w", err)
	}

	// Programmatic blur to trigger autosave
	s.Eval(`document.activeElement.blur()`)

	waitMs := pa.WaitMs
	if waitMs <= 0 {
		waitMs = 4000
	}
	s.Sleep(time.Duration(waitMs) * time.Millisecond)

	// Skip reload+reopen if the spec asks us to — some apps close the editor
	// on reload and refuse to re-open the same note by URL (no stable route
	// per note, or route redirects to the list). Screenshot happens while
	// the editor is still open.
	if pa.SkipReload {
		return nil
	}

	// Capture URL before reload
	noteURL, _ := s.CurrentURL()

	if err := s.Reload(); err != nil {
		return fmt.Errorf("reload after edit: %w", err)
	}

	// Navigate back to specific note if URL contained an ID
	appURL := strings.TrimRight(baseURL, "/")
	relativePath := strings.TrimPrefix(noteURL, appURL)
	if strings.Count(relativePath, "/") > 1 {
		// Has a path segment beyond the section — e.g. /notes/uuid
		if err := s.Navigate(noteURL); err != nil {
			// Fall back to clicking by text
			s.ClickIfExists(fmt.Sprintf(`text="%s"`, pa.Text))
		}
	} else {
		// Re-click the item by its selector
		s.ClickIfExists(pa.ClickSelector)
		s.Sleep(1 * time.Second)
	}

	return nil
}

// new_item — click a New/Create button.
func runNewItem(s *browser.Session, pa *config.PreAction) error {
	if err := s.Click(pa.Selector); err != nil {
		return fmt.Errorf("new_item click(%s): %w", pa.Selector, err)
	}
	ms := pa.WaitMs
	if ms <= 0 {
		ms = 1000
	}
	s.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}

// open_first — click first item in a list, fallback to creating new.
func runOpenFirst(s *browser.Session, pa *config.PreAction) error {
	err := s.Click(pa.Selector)
	if err != nil && pa.FallbackButton != "" {
		if err2 := s.Click(pa.FallbackButton); err2 != nil {
			return fmt.Errorf("open_first: main selector failed (%v), fallback also failed (%v)", err, err2)
		}
	} else if err != nil {
		return fmt.Errorf("open_first click(%s): %w", pa.Selector, err)
	}

	ms := pa.WaitMs
	if ms <= 0 {
		ms = 2000
	}
	s.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}

// drag — drag a source to a target (React DnD compatible).
func runDrag(s *browser.Session, pa *config.PreAction) error {
	holdMs := pa.HoldMs
	if holdMs <= 0 {
		holdMs = 500
	}
	if err := s.DragAndDrop(pa.Source, pa.Target, holdMs); err != nil {
		return fmt.Errorf("drag: %w", err)
	}
	s.Sleep(500 * time.Millisecond)

	if pa.ReloadAfter {
		if err := s.Reload(); err != nil {
			return fmt.Errorf("reload after drag: %w", err)
		}
	}
	return nil
}

// select_option — click first option/item in a selector list.
func runSelectOption(s *browser.Session, pa *config.PreAction) error {
	if err := s.Click(pa.Selector); err != nil {
		return fmt.Errorf("select_option click(%s): %w", pa.Selector, err)
	}
	ms := pa.WaitMs
	if ms <= 0 {
		ms = 1500
	}
	s.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}

// reload — reload the page.
func runReload(s *browser.Session, pa *config.PreAction) error {
	if err := s.Reload(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	ms := pa.WaitMs
	if ms <= 0 {
		ms = 2000
	}
	s.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}
