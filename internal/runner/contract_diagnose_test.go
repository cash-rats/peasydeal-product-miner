package runner

import "testing"

func TestDiagnoseContractIssue_MissingStatus(t *testing.T) {
	got := diagnoseContractIssue(`{"url":"x"}`)
	if got != "missing status" {
		t.Fatalf("unexpected diagnosis: %q", got)
	}
}

func TestDiagnoseContractIssue_TruncatedJSON(t *testing.T) {
	got := diagnoseContractIssue(`{"status":"ok","title":"abc"`)
	if got != "invalid or truncated JSON" {
		t.Fatalf("unexpected diagnosis: %q", got)
	}
}

func TestDiagnoseContractIssue_MultipleTopLevel(t *testing.T) {
	got := diagnoseContractIssue(`{"status":"ok"}{"x":1}`)
	if got != "multiple top-level JSON values" {
		t.Fatalf("unexpected diagnosis: %q", got)
	}
}

func TestDiagnoseContractIssue_IgnoreNestedObjectsForMissingStatus(t *testing.T) {
	got := diagnoseContractIssue(`{"status":"ok","variations":[{"title":"v","position":0}]`)
	if got != "invalid or truncated JSON" {
		t.Fatalf("unexpected diagnosis: %q", got)
	}
}
