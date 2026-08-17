package pageview

import (
	"strings"
	"testing"
)

func TestBootcRollbackRowDescribesTheOneDestination(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		timestamp string
		wantHas   string
	}{
		{name: "version and timestamp", version: "42.20260810", timestamp: "2026-08-10", wantHas: "version 42.20260810 (2026-08-10)"},
		{name: "version only", version: "42.20260810", wantHas: "version 42.20260810"},
		{name: "timestamp only", timestamp: "2026-08-10", wantHas: "image from 2026-08-10"},
		{name: "neither", wantHas: "No previous system image"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := BootcRollbackRow(test.version, test.timestamp)
			if row.Title != "Roll Back" {
				t.Errorf("BootcRollbackRow().Title = %q, want %q", row.Title, "Roll Back")
			}
			if !strings.Contains(row.Subtitle, test.wantHas) {
				t.Errorf("BootcRollbackRow(%q, %q).Subtitle = %q, want it to contain %q",
					test.version, test.timestamp, row.Subtitle, test.wantHas)
			}
		})
	}
}

// Every populated form must say the change takes effect at a restart, since
// rolling back does not alter the running system.
func TestBootcRollbackRowAlwaysDefersToARestart(t *testing.T) {
	for _, row := range []Row{
		BootcRollbackRow("42", "2026-08-10"),
		BootcRollbackRow("42", ""),
		BootcRollbackRow("", "2026-08-10"),
	} {
		if !strings.Contains(row.Subtitle, "restart") {
			t.Errorf("BootcRollbackRow().Subtitle = %q, want it to name the restart", row.Subtitle)
		}
	}
	if strings.Contains(BootcRollbackRow("", "").Subtitle, "restart") {
		t.Error("the no-rollback-available subtitle should not mention a restart")
	}
}

func TestBootcRollbackResultDoesNotClaimTheRunningSystemChanged(t *testing.T) {
	got := BootcRollbackResultSubtitle()
	if !strings.Contains(got, "restart") {
		t.Errorf("BootcRollbackResultSubtitle() = %q, want it to ask for a restart", got)
	}
}
