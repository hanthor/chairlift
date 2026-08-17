package pageview

import (
	"strings"
	"testing"
)

func TestGraphicsDriverRowCoversEveryState(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		hardware    string
		recommended string
		wantHas     string
	}{
		{
			name:    "nothing to offer",
			current: "Standard", hardware: "AMD",
			wantHas: "AMD · running the Standard image",
		},
		{
			name:    "a switch is offered",
			current: "Standard", hardware: "NVIDIA + Intel", recommended: "NVIDIA (proprietary)",
			wantHas: "switch to the NVIDIA (proprietary) image",
		},
		{
			name:    "already on the driver image",
			current: "NVIDIA (proprietary)", hardware: "NVIDIA",
			wantHas: "running the NVIDIA (proprietary) image",
		},
		{
			name:     "hardware known but image unrecognized",
			hardware: "Intel",
			wantHas:  "Intel",
		},
		{
			name:    "nothing detected",
			wantHas: "No graphics hardware detected",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := GraphicsDriverRow(test.current, test.hardware, test.recommended)
			if row.Title != "Graphics Driver" {
				t.Errorf("GraphicsDriverRow().Title = %q, want %q", row.Title, "Graphics Driver")
			}
			if !strings.Contains(row.Subtitle, test.wantHas) {
				t.Errorf("GraphicsDriverRow(%q, %q, %q).Subtitle = %q, want it to contain %q",
					test.current, test.hardware, test.recommended, row.Subtitle, test.wantHas)
			}
		})
	}
}

// Only the offering state may mention a restart; the informational states
// describe the machine and must not imply an action is pending.
func TestGraphicsDriverRowMentionsRestartOnlyWhenOffering(t *testing.T) {
	offering := GraphicsDriverRow("Standard", "NVIDIA", "NVIDIA (proprietary)")
	if !strings.Contains(offering.Subtitle, "restart") {
		t.Errorf("the offering subtitle %q does not mention the restart", offering.Subtitle)
	}
	for _, row := range []Row{
		GraphicsDriverRow("Standard", "AMD", ""),
		GraphicsDriverRow("NVIDIA (proprietary)", "NVIDIA", ""),
		GraphicsDriverRow("", "", ""),
	} {
		if strings.Contains(row.Subtitle, "restart") {
			t.Errorf("informational subtitle %q should not mention a restart", row.Subtitle)
		}
	}
}

func TestGraphicsDriverResultDefersToARestart(t *testing.T) {
	got := GraphicsDriverResultSubtitle("NVIDIA (proprietary)")
	if !strings.Contains(got, "restart to apply") {
		t.Errorf("GraphicsDriverResultSubtitle() = %q, want it to ask for a restart", got)
	}
	if !strings.Contains(got, "NVIDIA (proprietary)") {
		t.Errorf("GraphicsDriverResultSubtitle() = %q, want it to name the driver", got)
	}
}
