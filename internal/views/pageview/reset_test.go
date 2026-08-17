package pageview

import (
	"strings"
	"testing"
)

func TestPowerwashRowReflectsRunState(t *testing.T) {
	fresh := PowerwashRow("")
	if fresh.Title != "Remove Everything I Installed" {
		t.Errorf("PowerwashRow(\"\").Title = %q, want %q", fresh.Title, "Remove Everything I Installed")
	}
	if !strings.Contains(fresh.Subtitle, "Distrobox") {
		t.Errorf("PowerwashRow(\"\").Subtitle = %q, want it to describe what gets removed", fresh.Subtitle)
	}

	done := PowerwashRow("Powerwash complete")
	if done.Subtitle != "Powerwash complete" {
		t.Errorf("PowerwashRow(summary).Subtitle = %q, want the summary verbatim", done.Subtitle)
	}
}

// The confirmation is the one place a user learns Powerwash is irreversible
// and does not touch the image; both facts must be present.
func TestPowerwashConfirmationStatesWhatItDoesAndDoesNot(t *testing.T) {
	title, body := PowerwashConfirmation()
	if !strings.Contains(title, "Remove Everything") {
		t.Errorf("PowerwashConfirmation() title = %q, want it to name the action", title)
	}
	for _, want := range []string{"Flatpak", "Distrobox", "cannot be undone"} {
		if !strings.Contains(body, want) {
			t.Errorf("PowerwashConfirmation() body = %q, want it to contain %q", body, want)
		}
	}
}

func TestFactoryResetRowNamesWhatItReplaces(t *testing.T) {
	row := FactoryResetRow()
	if row.Title != "Factory Reset" {
		t.Errorf("FactoryResetRow().Title = %q, want %q", row.Title, "Factory Reset")
	}
	if !strings.Contains(row.Subtitle, "fresh install") {
		t.Errorf("FactoryResetRow().Subtitle = %q, want it to describe the reset", row.Subtitle)
	}
}

// The one non-negotiable requirement: --experimental must be named in the
// confirmation, not hidden behind friendlier wording, because it is the
// single fact most likely to change a user's mind about proceeding.
func TestFactoryResetConfirmationNamesExperimental(t *testing.T) {
	title, body := FactoryResetConfirmation()
	if !strings.Contains(title, "Factory Reset") {
		t.Errorf("FactoryResetConfirmation() title = %q, want it to name the action", title)
	}
	if !strings.Contains(body, "--experimental") {
		t.Errorf("FactoryResetConfirmation() body = %q, want it to name --experimental", body)
	}
	if !strings.Contains(body, "cannot be undone") {
		t.Errorf("FactoryResetConfirmation() body = %q, want it to state irreversibility", body)
	}
}

func TestFactoryResetResultDefersToARestart(t *testing.T) {
	got := FactoryResetResultSubtitle()
	if !strings.Contains(got, "restart") {
		t.Errorf("FactoryResetResultSubtitle() = %q, want it to ask for a restart", got)
	}
}
