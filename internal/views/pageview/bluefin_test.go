package pageview

import (
	"strings"
	"testing"
)

func TestBluefinGroupDescriptionNamesVariantAndTag(t *testing.T) {
	tests := []struct {
		name    string
		variant string
		tag     string
		want    string
	}{
		{name: "dakota", variant: "Dakota", tag: "latest", want: "Dakota · latest"},
		{name: "bluefin", variant: "Bluefin", tag: "stable", want: "Bluefin · stable"},
		{name: "bluefin lts", variant: "Bluefin LTS", tag: "lts-testing", want: "Bluefin LTS · lts-testing"},
		{name: "no tag", variant: "Dakota", tag: "", want: "Dakota"},
		{name: "no variant", variant: "", tag: "latest", want: "Bluefin-family features"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := BluefinGroupDescription(test.variant, test.tag); got != test.want {
				t.Errorf("BluefinGroupDescription(%q, %q) = %q, want %q", test.variant, test.tag, got, test.want)
			}
		})
	}
}

func TestChannelRowExplainsEveryState(t *testing.T) {
	tests := []struct {
		name       string
		onTesting  bool
		switchable bool
		tag        string
		wantHas    string
	}{
		{name: "on stable, switchable", switchable: true, tag: "latest", wantHas: "restart to apply"},
		{name: "on testing, switchable", onTesting: true, switchable: true, tag: "testing", wantHas: "return to stable"},
		{name: "unswitchable tag names the tag", tag: "stable", wantHas: "no testing channel for the stable tag"},
		{name: "no tag at all", wantHas: "does not publish a testing channel"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := ChannelRow(test.onTesting, test.switchable, test.tag)
			if row.Title != "Testing Channel" {
				t.Errorf("ChannelRow().Title = %q, want %q", row.Title, "Testing Channel")
			}
			if !strings.Contains(row.Subtitle, test.wantHas) {
				t.Errorf("ChannelRow().Subtitle = %q, want it to contain %q", row.Subtitle, test.wantHas)
			}
		})
	}
}

// bootc stages the switch; the running system is unchanged until reboot. No
// result subtitle may imply otherwise.
func TestChannelSwitchResultAlwaysAsksForARestart(t *testing.T) {
	for _, toTesting := range []bool{true, false} {
		got := ChannelSwitchResultSubtitle(toTesting)
		if !strings.Contains(got, "restart to apply") {
			t.Errorf("ChannelSwitchResultSubtitle(%v) = %q, want it to ask for a restart", toTesting, got)
		}
	}
	if ChannelSwitchResultSubtitle(true) == ChannelSwitchResultSubtitle(false) {
		t.Error("ChannelSwitchResultSubtitle does not distinguish the two directions")
	}
}

func TestDeveloperRowNamesActiveGroups(t *testing.T) {
	active := DeveloperRow(true, []string{"docker", "libvirt"})
	if active.Title != "Developer Mode" {
		t.Errorf("DeveloperRow().Title = %q, want %q", active.Title, "Developer Mode")
	}
	if !strings.Contains(active.Subtitle, "docker, libvirt") {
		t.Errorf("DeveloperRow(true).Subtitle = %q, want it to list the joined groups", active.Subtitle)
	}

	inactive := DeveloperRow(false, nil)
	if strings.Contains(inactive.Subtitle, "Active") {
		t.Errorf("DeveloperRow(false).Subtitle = %q, want it not to claim active", inactive.Subtitle)
	}
}

// Group membership only applies to new login sessions, so neither outcome
// may read as immediately effective.
func TestDeveloperResultAlwaysAsksForALogout(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		got := DeveloperResultSubtitle(enabled)
		if !strings.Contains(got, "log out") {
			t.Errorf("DeveloperResultSubtitle(%v) = %q, want it to ask for a re-login", enabled, got)
		}
	}
	if DeveloperResultSubtitle(true) == DeveloperResultSubtitle(false) {
		t.Error("DeveloperResultSubtitle does not distinguish the two directions")
	}
}

func TestGamingRowCarriesTheSummary(t *testing.T) {
	row := GamingRow("All 6 gaming components installed")
	if row.Title != "Gaming Mode" {
		t.Errorf("GamingRow().Title = %q, want %q", row.Title, "Gaming Mode")
	}
	if row.Subtitle != "All 6 gaming components installed" {
		t.Errorf("GamingRow().Subtitle = %q, want the summary verbatim", row.Subtitle)
	}
}

func TestGamingResultReportsCountsAndFailures(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		changed int
		failed  int
		wantHas string
	}{
		{name: "installed", enabled: true, changed: 6, wantHas: "6 component(s) installed"},
		{name: "removed", changed: 4, wantHas: "4 component(s) removed"},
		{name: "partial failure", enabled: true, changed: 4, failed: 2, wantHas: "2 failed"},
		{name: "nothing to do", enabled: true, wantHas: "No components needed changing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GamingResultSubtitle(test.enabled, test.changed, test.failed)
			if !strings.Contains(got, test.wantHas) {
				t.Errorf("GamingResultSubtitle(%v, %d, %d) = %q, want it to contain %q",
					test.enabled, test.changed, test.failed, got, test.wantHas)
			}
		})
	}
}
