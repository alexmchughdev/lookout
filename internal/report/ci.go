// CI-friendly output formats: JUnit XML for CI systems.
package report

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AlexMcHugh1/lookout/internal/config"
	"github.com/AlexMcHugh1/lookout/internal/runner"
)

// WriteJUnit writes a JUnit-style XML report consumable by GitHub Actions,
// GitLab CI, Jenkins, CircleCI, etc.
// `Blocked` verdicts are reported as errors; `Skipped` as skipped; `Fail` as failures.
func WriteJUnit(results []*runner.Result, spec *config.Spec, duration time.Duration, path string) error {
	type testcase struct {
		XMLName  xml.Name `xml:"testcase"`
		Name     string   `xml:"name,attr"`
		Classname string  `xml:"classname,attr"`
		Time     string   `xml:"time,attr"`
		Failure  *struct {
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

