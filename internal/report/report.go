// Package report generates HTML QA reports with embedded screenshots.
package report

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexmchughdev/lookout/internal/config"
	"github.com/alexmchughdev/lookout/internal/runner"
)

// Write generates an HTML report and returns the file path.
// When includeScreenshots is true (the default), every test embeds its
// screenshot inline; false produces a minimal report with verdicts only.
func Write(
	results []*runner.Result,
	spec *config.Spec,
	duration time.Duration,
	outputDir string,
	build string,
	includeScreenshots bool,
) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	ts := time.Now().Format("2006-01-02_1504")
	fname := fmt.Sprintf("lookout-report_%s_build-%s.html", ts, build)
	path := filepath.Join(outputDir, fname)

	html := renderHTML(results, spec, duration, build, includeScreenshots)

	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("writing report: %w", err)
	}

	return path, nil
}

func renderHTML(results []*runner.Result, spec *config.Spec, duration time.Duration, build string, includeScreenshots bool) string {
	passC, failC, blockC, skipC := 0, 0, 0, 0
	for _, r := range results {
		switch r.Verdict.Result {
		case "Pass":
			passC++
		case "Fail":
			failC++
		case "Blocked":
			blockC++
		case "Skipped":
			skipC++
		}
	}

	overall := "PASS"
	overallColor := "#22c55e"
	if failC > 0 {
		overall = "FAIL"
		overallColor = "#ef4444"
	}

	// Group by section
	type sectionItem struct {
		ID      string
		Verdict string
		Note    string
		SS      []byte
	}
	sections := make(map[string][]sectionItem)
	sectionOrder := []string{}
	seen := map[string]bool{}
	for _, r := range results {
		if !seen[r.Section] {
			sectionOrder = append(sectionOrder, r.Section)
			seen[r.Section] = true
		}
		sections[r.Section] = append(sections[r.Section], sectionItem{
			ID:      r.TestID,
			Verdict: r.Verdict.Result,
			Note:    r.Verdict.Note,
			SS:      r.Screenshot,
		})
	}

	// Summary table rows
	var summaryRows strings.Builder
	for _, r := range results {
		summaryRows.WriteString(fmt.Sprintf(
			"<tr><td><code>%s</code></td><td style='color:#6b7280;font-size:13px'>%s</td><td>%s</td><td style='font-size:13px;color:#374151'>%s</td></tr>\n",
			r.TestID, r.Section, badge(r.Verdict.Result), r.Verdict.Note,
		))
	}

	// Section detail cards
	var sectionHTML strings.Builder
	for _, sec := range sectionOrder {
		sectionHTML.WriteString(fmt.Sprintf(
			"<h2 style='font-size:17px;font-weight:600;margin:28px 0 12px;text-transform:capitalize;color:#111'>%s</h2>\n",
			sec,
		))
		for _, item := range sections[sec] {
			imgTag := ""
			if includeScreenshots && len(item.SS) > 0 {
				b64 := base64.StdEncoding.EncodeToString(item.SS)
				imgTag = fmt.Sprintf(
					`<img src="data:image/png;base64,%s" style="width:100%%;border:1px solid #e5e7eb;border-radius:6px;margin-top:10px">`,
					b64,
				)
			}
			sectionHTML.WriteString(fmt.Sprintf(`
<div style="border:1px solid #e5e7eb;border-radius:8px;padding:14px;margin-bottom:12px;background:#fff">
  <div style="display:flex;justify-content:space-between;align-items:center">
    <code style="font-size:14px;font-weight:600">%s</code>%s
  </div>
  <p style="margin:6px 0 0;color:#374151;font-size:13px">%s</p>
  %s
</div>`, item.ID, badge(item.Verdict), item.Note, imgTag))
		}
	}

	screenshotNote := "Screenshot captured for every test."
	if !includeScreenshots {
		screenshotNote = "Screenshots omitted (--no-screenshots). Run without the flag to see what the vision model saw."
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>lookout QA Report</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,-apple-system,sans-serif;background:#f9fafb;color:#111827;padding:32px 24px}
.card{background:#fff;border:1px solid #e5e7eb;border-radius:12px;padding:24px;margin-bottom:24px}
table{width:100%%;border-collapse:collapse;background:#fff;border:1px solid #e5e7eb;border-radius:8px;overflow:hidden}
th{background:#f3f4f6;text-align:left;padding:10px 14px;font-size:13px;font-weight:600;color:#374151}
td{padding:10px 14px;font-size:13px;border-top:1px solid #f3f4f6}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(110px,1fr));gap:12px;margin-top:20px}
.stat{background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:14px;text-align:center}
.stat-n{font-size:28px;font-weight:700;line-height:1}
.stat-l{font-size:12px;color:#6b7280;margin-top:4px}
code{background:#f3f4f6;padding:2px 6px;border-radius:4px;font-size:13px}
</style>
</head>
<body>
<div class="card">
  <div style="display:flex;justify-content:space-between;align-items:flex-start">
    <div>
      <div style="display:flex;align-items:center;gap:10px;margin-bottom:6px">
        <span style="font-size:22px;font-weight:700">lookout</span>
        <span style="font-size:13px;color:#6b7280;background:#f3f4f6;padding:2px 8px;border-radius:4px">v0.1.0</span>
      </div>
      <p style="color:#6b7280;font-size:14px">
        %s &nbsp;·&nbsp; %s &nbsp;·&nbsp; %s &nbsp;·&nbsp; %s/%s
      </p>
    </div>
    <span style="font-size:22px;font-weight:700;color:%s">%s</span>
  </div>
  <div class="stats">
    <div class="stat"><div class="stat-n" style="color:#22c55e">%d</div><div class="stat-l">Pass</div></div>
    <div class="stat"><div class="stat-n" style="color:#ef4444">%d</div><div class="stat-l">Fail</div></div>
    <div class="stat"><div class="stat-n" style="color:#f59e0b">%d</div><div class="stat-l">Blocked</div></div>
    <div class="stat"><div class="stat-n" style="color:#6b7280">%d</div><div class="stat-l">Skipped</div></div>
    <div class="stat"><div class="stat-n" style="color:#374151">%d</div><div class="stat-l">Total</div></div>
  </div>
  <p style="margin-top:16px;font-size:13px;color:#9ca3af">Build: <code>%s</code></p>
</div>

<h2 style="font-size:17px;font-weight:600;margin:0 0 12px">All results</h2>
<table>
<thead><tr><th>TC-ID</th><th>Section</th><th>Result</th><th>Notes</th></tr></thead>
<tbody>%s</tbody>
</table>

<div style="margin-top:32px">
  <h2 style="font-size:17px;font-weight:600;margin-bottom:4px">Results with screenshots</h2>
  <p style="font-size:13px;color:#6b7280;margin-bottom:16px">%s</p>
  %s
</div>

<p style="text-align:center;color:#9ca3af;font-size:12px;margin-top:40px">
  Generated by <strong>lookout</strong> &nbsp;·&nbsp;
  <a href="https://github.com/alexmchughdev/lookout" style="color:#9ca3af">github.com/alexmchughdev/lookout</a>
</p>
</body>
</html>`,
		spec.App.URL,
		time.Now().Format("02 Jan 2006 15:04"),
		duration.Round(time.Second),
		spec.Model.Provider, spec.Model.Name,
		overallColor, overall,
		passC, failC, blockC, skipC, passC+failC+blockC+skipC,
		build,
		summaryRows.String(),
		screenshotNote,
		sectionHTML.String(),
	)
}

func badge(result string) string {
	colours := map[string]string{
		"Pass":    "#22c55e",
		"Fail":    "#ef4444",
		"Blocked": "#f59e0b",
		"Skipped": "#6b7280",
	}
	c := colours[result]
	if c == "" {
		c = "#6b7280"
	}
	return fmt.Sprintf(
		`<span style="background:%s;color:#fff;padding:2px 10px;border-radius:4px;font-size:12px;font-weight:600">%s</span>`,
		c, result,
	)
}
