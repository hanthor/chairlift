package pageview

import (
	"strings"
	"testing"
)

func TestAIStackRowNamesTheSelectedAccelerator(t *testing.T) {
	row := AIStackRow("ROCm", true)

	if row.Title != "Local AI Model Server" {
		t.Errorf("title = %q", row.Title)
	}
	if !strings.Contains(row.Subtitle, "ROCm") {
		t.Errorf("subtitle does not name the accelerator: %q", row.Subtitle)
	}
}

func TestAIStackRowWarnsThatACPUHostIsSlow(t *testing.T) {
	row := AIStackRow("CPU", false)

	for _, want := range []string{"No GPU", "slower"} {
		if !strings.Contains(row.Subtitle, want) {
			t.Errorf("subtitle %q does not mention %q", row.Subtitle, want)
		}
	}
}

func TestAIStackResultSubtitleGivesTheUserTheAddress(t *testing.T) {
	enabled := AIStackResultSubtitle(true, 8080)

	if !strings.Contains(enabled, "http://localhost:8080") {
		t.Errorf("enabled subtitle does not carry the API address: %q", enabled)
	}
	if !strings.Contains(enabled, "GB") {
		t.Errorf("enabled subtitle does not warn about the download: %q", enabled)
	}

	disabled := AIStackResultSubtitle(false, 8080)
	if !strings.Contains(disabled, "kept") {
		t.Errorf("disabled subtitle does not say the model was kept: %q", disabled)
	}
}
