package pageview

import (
	"strings"
	"testing"
)

func TestUpdateAllRowDescribesThePlanSize(t *testing.T) {
	tests := []struct {
		name    string
		planned int
		wantHas string
	}{
		{name: "nothing available", planned: 0, wantHas: "Nothing on this system"},
		{name: "one source", planned: 1, wantHas: "the one available source"},
		{name: "everything", planned: 3, wantHas: "in one step"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := UpdateAllRow(test.planned)
			if row.Title != "Update All" {
				t.Errorf("UpdateAllRow().Title = %q, want %q", row.Title, "Update All")
			}
			if !strings.Contains(row.Subtitle, test.wantHas) {
				t.Errorf("UpdateAllRow(%d).Subtitle = %q, want it to contain %q", test.planned, row.Subtitle, test.wantHas)
			}
		})
	}
}

func TestUpdateAllPhaseSubtitleCoversEveryStage(t *testing.T) {
	tests := []struct {
		name    string
		running bool
		detail  string
		want    string
	}{
		{name: "not started", want: "Waiting"},
		{name: "in flight", running: true, want: "Working…"},
		{name: "finished", detail: "Update 42 staged — restart to apply", want: "Update 42 staged — restart to apply"},
		{name: "finished with a failure", detail: "registry unreachable", want: "registry unreachable"},
		// A phase reported as running wins over a stale detail from a
		// previous run, or a re-run would show the old result while working.
		{name: "running after a previous result", running: true, detail: "Up to date", want: "Working…"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := UpdateAllPhaseSubtitle(test.running, test.detail); got != test.want {
				t.Errorf("UpdateAllPhaseSubtitle(%v, %q) = %q, want %q", test.running, test.detail, got, test.want)
			}
		})
	}
}

func TestRestartRowNamesTheStagedVersion(t *testing.T) {
	withVersion := RestartRow("42.20260817")
	if withVersion.Title != "Restart to Apply" {
		t.Errorf("RestartRow().Title = %q, want %q", withVersion.Title, "Restart to Apply")
	}
	if !strings.Contains(withVersion.Subtitle, "42.20260817") {
		t.Errorf("RestartRow(version).Subtitle = %q, want it to name the version", withVersion.Subtitle)
	}

	withoutVersion := RestartRow("")
	if !strings.Contains(withoutVersion.Subtitle, "staged") {
		t.Errorf("RestartRow(\"\").Subtitle = %q, want it to say an update is staged", withoutVersion.Subtitle)
	}
	// Both forms must name the restart as what applies the update, since the
	// row exists only when something really is staged.
	for _, row := range []Row{withVersion, withoutVersion} {
		if !strings.Contains(row.Subtitle, "restart") {
			t.Errorf("RestartRow().Subtitle = %q, want it to name the restart", row.Subtitle)
		}
	}
}
