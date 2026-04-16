package vision

import (
	"strings"
	"testing"
)

func TestParseResponse_Normal(t *testing.T) {
	v := parseResponse("RESULT: Pass\nNOTE: The login form is visible.")
	if v.Result != "Pass" {
		t.Errorf("Result: got %q, want Pass", v.Result)
	}
	if v.Note != "The login form is visible." {
		t.Errorf("Note: got %q", v.Note)
	}
}

func TestParseResponse_CaseInsensitive(t *testing.T) {
	v := parseResponse("result: fail\nnote: nothing rendered")
	if v.Result != "Fail" {
		t.Errorf("Result: got %q, want Fail", v.Result)
	}
	if v.Note != "nothing rendered" {
		t.Errorf("Note: got %q", v.Note)
	}
}

func TestParseResponse_MissingResultDefaultsToBlocked(t *testing.T) {
	v := parseResponse("some free-form reply that doesn't follow the contract")
	if v.Result != "Blocked" {
		t.Errorf("Result: got %q, want Blocked", v.Result)
	}
}

func TestParseResponse_NoteTruncation(t *testing.T) {
	long := strings.Repeat("x", 500)
	v := parseResponse("RESULT: Pass\nNOTE: " + long)
	if len(v.Note) != 200 {
		t.Errorf("Note length: got %d, want 200", len(v.Note))
	}
}

func TestParseResponse_Blocked(t *testing.T) {
	v := parseResponse("RESULT: Blocked\nNOTE: page did not load")
	if v.Result != "Blocked" {
		t.Errorf("Result: got %q, want Blocked", v.Result)
	}
}

func TestParseResponse_Skipped(t *testing.T) {
	v := parseResponse("RESULT: Skipped\nNOTE: feature not present")
	if v.Result != "Skipped" {
		t.Errorf("Result: got %q, want Skipped", v.Result)
	}
}
