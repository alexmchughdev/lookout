// CI-friendly output formats: JUnit XML (for CI systems) and JSON (for custom pipelines).
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alexmchughdev/lookout/internal/config"
	"github.com/alexmchughdev/lookout/internal/runner"
)

// WriteJUnit writes a JUnit-style XML report consumable by GitHub Actions,
// GitLab CI, Jenkins, CircleCI, etc.
// `Blocked` verdicts are reported as errors; `Skipped` as skipped; `Fail` as failures.
func WriteJUnit(results []*runner.Result, spec *config.Spec, duration time.Duration, path string) error {
	type testcase struct {
		XMLName   xml.Name `xml:"testcase"`
		Name      string   `xml:"name,attr"`
		Classname string   `xml:"classname,attr"`
		Time      string   `xml:"time,attr"`
		Failure   *struct {
			XMLName xml.Name `xml:"failure"`
			Message string   `xml:"message,attr"`
			Body    string   `xml:",chardata"`
		} `xml:"failure,omitempty"`
		Error *struct {
			XMLName xml.Name `xml:"error"`
			Message string   `xml:"message,attr"`
			Body    string   `xml:",chardata"`
		} `xml:"error,omitempty"`
		Skipped *struct {
			XMLName xml.Name `xml:"skipped"`
			Message string   `xml:"message,attr"`
		} `xml:"skipped,omitempty"`
	}

	type testsuite struct {
		XMLName   xml.Name   `xml:"testsuite"`
		Name      string     `xml:"name,attr"`
		Tests     int        `xml:"tests,attr"`
		Failures  int        `xml:"failures,attr"`
		Errors    int        `xml:"errors,attr"`
		Skipped   int        `xml:"skipped,attr"`
		Time      string     `xml:"time,attr"`
		Timestamp string     `xml:"timestamp,attr"`
		Testcases []testcase `xml:"testcase"`
	}

	ts := testsuite{
		Name:      "lookout",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Time:      fmt.Sprintf("%.3f", duration.Seconds()),
		Tests:     len(results),
	}

	for _, r := range results {
		tc := testcase{
			Name:      r.TestID,
			Classname: r.Section,
			Time:      fmt.Sprintf("%.3f", r.Duration.Seconds()),
		}
		switch r.Verdict.Result {
		case "Fail":
			tc.Failure = &struct {
				XMLName xml.Name `xml:"failure"`
				Message string   `xml:"message,attr"`
				Body    string   `xml:",chardata"`
			}{Message: r.Verdict.Note, Body: r.Verdict.Note}
			ts.Failures++
		case "Blocked":
			tc.Error = &struct {
				XMLName xml.Name `xml:"error"`
				Message string   `xml:"message,attr"`
				Body    string   `xml:",chardata"`
			}{Message: r.Verdict.Note, Body: r.Verdict.Note}
			ts.Errors++
		case "Skipped":
			tc.Skipped = &struct {
				XMLName xml.Name `xml:"skipped"`
				Message string   `xml:"message,attr"`
			}{Message: r.Verdict.Note}
			ts.Skipped++
		}
		ts.Testcases = append(ts.Testcases, tc)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating junit dir: %w", err)
	}

	data, err := xml.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}
	out := []byte(xml.Header)
	out = append(out, data...)
	out = append(out, '\n')
	return os.WriteFile(path, out, 0644)
}

// WriteJSON writes a machine-readable JSON report.
func WriteJSON(results []*runner.Result, spec *config.Spec, duration time.Duration, build, path string) error {
	type jsonResult struct {
		ID        string `json:"id"`
		Section   string `json:"section"`
		Result    string `json:"result"`
		Note      string `json:"note"`
		Duration  string `json:"duration"`
		Attempts  int    `json:"attempts"`
		PreActErr string `json:"pre_action_error,omitempty"`
	}
	type summary struct {
		Pass, Fail, Blocked, Skipped, Total int
	}

	out := struct {
		Build    string       `json:"build"`
		URL      string       `json:"url"`
		Provider string       `json:"provider"`
		Model    string       `json:"model"`
		Duration string       `json:"duration"`
		Summary  summary      `json:"summary"`
		Results  []jsonResult `json:"results"`
	}{
		Build:    build,
		URL:      spec.App.URL,
		Provider: spec.Model.Provider,
		Model:    spec.Model.Name,
		Duration: duration.Round(time.Millisecond).String(),
	}

	for _, r := range results {
		out.Results = append(out.Results, jsonResult{
			ID:        r.TestID,
			Section:   r.Section,
			Result:    r.Verdict.Result,
			Note:      r.Verdict.Note,
			Duration:  r.Duration.Round(time.Millisecond).String(),
			Attempts:  r.Attempts,
			PreActErr: r.PreActErr,
		})
		out.Summary.Total++
		switch r.Verdict.Result {
		case "Pass":
			out.Summary.Pass++
		case "Fail":
			out.Summary.Fail++
		case "Blocked":
			out.Summary.Blocked++
		case "Skipped":
			out.Summary.Skipped++
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating json dir: %w", err)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
