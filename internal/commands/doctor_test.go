package commands

import "testing"

func TestSummarizeDoctor(t *testing.T) {
	tests := []struct {
		name        string
		checks      []CheckResult
		strict      bool
		wantSummary string
		wantCode    int
	}{
		{
			name: "all OK",
			checks: []CheckResult{
				{Status: StatusOK},
				{Status: StatusOK},
			},
			wantSummary: "All checks passed",
			wantCode:    0,
		},
		{
			name: "warning in non-strict mode",
			checks: []CheckResult{
				{Status: StatusOK},
				{Status: StatusWarning},
			},
			wantSummary: "All required checks passed (with warnings)",
			wantCode:    0,
		},
		{
			name: "warning in strict mode",
			checks: []CheckResult{
				{Status: StatusOK},
				{Status: StatusWarning},
			},
			strict:      true,
			wantSummary: "Some checks have warnings (strict mode)",
			wantCode:    2,
		},
		{
			name: "failure",
			checks: []CheckResult{
				{Status: StatusOK},
				{Status: StatusWarning},
				{Status: StatusFail},
			},
			wantSummary: "Some checks failed",
			wantCode:    3,
		},
		{
			name: "only skipped",
			checks: []CheckResult{
				{Status: StatusSkipped},
			},
			wantSummary: "No checks ran",
			wantCode:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, exitCode := summarizeDoctor(tt.checks, tt.strict)
			if summary != tt.wantSummary {
				t.Errorf("summarizeDoctor() summary = %q, want %q", summary, tt.wantSummary)
			}
			if exitCode != tt.wantCode {
				t.Errorf("summarizeDoctor() exit code = %d, want %d", exitCode, tt.wantCode)
			}
		})
	}
}
