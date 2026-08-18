package sbom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) Packages {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	packages, err := Parse(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return packages
}

func TestParseAcceptsSyftUnderTheSPDXArtifactType(t *testing.T) {
	packages := loadFixture(t, "booted-syft.json")

	if got := packages["kernel"]; got != "6.17.4-200.fc44" {
		t.Errorf("kernel = %q", got)
	}
	if len(packages) != 6 {
		t.Errorf("parsed %d packages, want 6", len(packages))
	}
}

func TestParseAcceptsRealSPDX(t *testing.T) {
	packages := loadFixture(t, "booted-spdx.json")

	if got := packages["mesa"]; got != "25.2.3-1.fc44" {
		t.Errorf("mesa = %q", got)
	}
}

// A document in neither shape must fail loudly. Every field in both schemas
// is optional, so the natural implementation returns an empty map and the
// UI renders a blank changelog with no error anywhere to notice.
func TestParseRejectsADocumentWithNoPackages(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"unrelated json object", `{"hello": "world"}`},
		{"empty syft artifacts", `{"artifacts": []}`},
		{"empty spdx packages", `{"packages": []}`},
		{"not json at all", `<html>404</html>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.data)); err == nil {
				t.Fatal("Parse accepted a document carrying no packages")
			}
		})
	}
}

func TestDiffCategorizesEveryKindOfChange(t *testing.T) {
	from := loadFixture(t, "booted-syft.json")
	to := loadFixture(t, "target-syft.json")

	result := Diff(from, to)

	assertChanges(t, "upgraded", result.Upgraded, map[string][2]string{
		"kernel": {"6.17.4-200.fc44", "6.17.9-200.fc44"},
		"mesa":   {"25.2.3-1.fc44", "25.10.0-1.fc44"},
	})
	assertChanges(t, "downgraded", result.Downgraded, map[string][2]string{
		"bootc": {"1.7.0-1.fc44", "1.6.2-1.fc44"},
	})
	// Two commit hashes have no order, so the pair must not be reported as
	// either an upgrade or a downgrade.
	assertChanges(t, "changed", result.Changed, map[string][2]string{
		"vendored-lib": {"3f8a2b1c9d4e5f6071829304a5b6c7d8e9f0a1b2", "c7d8e9f0a1b23f8a2b1c9d4e5f6071829304a5b6"},
	})
	assertChanges(t, "added", result.Added, map[string][2]string{
		"new-tool": {"", "1.0.0-1.fc44"},
	})
	assertChanges(t, "removed", result.Removed, map[string][2]string{
		"removed-tool": {"0.4.1-1.fc44", ""},
	})

	// podman is identical in both and must appear nowhere.
	if result.Total() != 6 {
		t.Errorf("Total() = %d, want 6", result.Total())
	}
}

func TestDiffOfIdenticalImagesIsEmpty(t *testing.T) {
	packages := loadFixture(t, "booted-syft.json")

	result := Diff(packages, packages)
	if !result.Empty() {
		t.Errorf("diffing an image against itself produced %d changes", result.Total())
	}
}

func assertChanges(t *testing.T, label string, got []Change, want map[string][2]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s = %v, want %d entries", label, got, len(want))
		return
	}
	for _, change := range got {
		versions, ok := want[change.Name]
		if !ok {
			t.Errorf("%s contains an unexpected package %q", label, change.Name)
			continue
		}
		if change.From != versions[0] || change.To != versions[1] {
			t.Errorf("%s %s = %q -> %q, want %q -> %q",
				label, change.Name, change.From, change.To, versions[0], versions[1])
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Name > got[i].Name {
			t.Errorf("%s is not sorted by name: %v", label, got)
			break
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"6.17.4-200.fc44", "6.17.9-200.fc44", -1},
		// The case a lexical comparison gets wrong.
		{"25.2.3-1.fc44", "25.10.0-1.fc44", -1},
		{"1.7.0-1.fc44", "1.6.2-1.fc44", 1},
		{"1.0", "1.0", 0},
		{"1.0.1", "1.0", 1},
		// A release suffix is a version segment, not noise.
		{"1.0-1.fc44", "1.0-2.fc44", -1},
		// A numeric segment outranks an alphabetic one at the same position.
		{"1.0.1", "1.0.rc", 1},
		// rpm does not read "rc" as a pre-release — that is what the tilde
		// is for, and Fedora versions rely on both behaviors.
		{"1.0rc1", "1.0", 1},
		{"1.0~rc1", "1.0", -1},
		{"1.0~rc1", "1.0~rc2", -1},
		{"2:1.4", "2:1.5", -1},
		// Two hashes have no order.
		{"3f8a2b1c9d4e5f6071829304a5b6c7d8e9f0a1b2", "c7d8e9f0a1b23f8a2b1c9d4e5f6071829304a5b6", 0},
		// A hash against a version has no order either.
		{"3f8a2b1c9d4e5f6071829304a5b6c7d8e9f0a1b2", "1.2.3", 0},
	}

	for _, tt := range tests {
		if got := CompareVersions(tt.a, tt.b); got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		// The comparison must be antisymmetric, or a diff would classify a
		// pair differently depending on which image it called "from".
		if got := CompareVersions(tt.b, tt.a); got != -tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d (antisymmetry)", tt.b, tt.a, got, -tt.want)
		}
	}
}

func TestPinnedReference(t *testing.T) {
	tests := []struct {
		image, digest, want string
	}{
		{"ghcr.io/ublue-os/bluefin:stable", "sha256:abc", "ghcr.io/ublue-os/bluefin@sha256:abc"},
		{"ghcr.io/ublue-os/bluefin", "sha256:abc", "ghcr.io/ublue-os/bluefin@sha256:abc"},
		{"ghcr.io/ublue-os/bluefin@sha256:old", "sha256:abc", "ghcr.io/ublue-os/bluefin@sha256:abc"},
		// A port in the registry host is not a tag.
		{"registry.example.internal:5000/team/image:v1", "sha256:abc", "registry.example.internal:5000/team/image@sha256:abc"},
		{"ghcr.io/ublue-os/bluefin:stable", "", "ghcr.io/ublue-os/bluefin:stable"},
		{"", "sha256:abc", ""},
	}

	for _, tt := range tests {
		if got := PinnedReference(tt.image, tt.digest); got != tt.want {
			t.Errorf("PinnedReference(%q, %q) = %q, want %q", tt.image, tt.digest, got, tt.want)
		}
	}
}

func TestCompareDiffsTwoFetchedImages(t *testing.T) {
	fetch := func(_ context.Context, ref string) ([]byte, error) {
		switch ref {
		case "from":
			return os.ReadFile(filepath.Join("testdata", "booted-syft.json"))
		case "to":
			return os.ReadFile(filepath.Join("testdata", "target-syft.json"))
		}
		return nil, fmt.Errorf("unexpected reference %q", ref)
	}

	result, err := Compare(context.Background(), fetch, "from", "to")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.Total() != 6 {
		t.Errorf("Total() = %d, want 6", result.Total())
	}
}

func TestCompareNamesWhichSideFailed(t *testing.T) {
	fetch := func(_ context.Context, ref string) ([]byte, error) {
		if ref == "from" {
			return os.ReadFile(filepath.Join("testdata", "booted-syft.json"))
		}
		return nil, fmt.Errorf("no such artifact")
	}

	_, err := Compare(context.Background(), fetch, "from", "to")
	if err == nil {
		t.Fatal("Compare succeeded with an unreadable staged SBOM")
	}
	if !strings.Contains(err.Error(), "staged") {
		t.Errorf("error does not say which side failed: %v", err)
	}
}

func TestCompareRejectsAMissingReference(t *testing.T) {
	fetch := func(context.Context, string) ([]byte, error) {
		t.Error("Compare fetched despite a missing reference")
		return nil, nil
	}
	if _, err := Compare(context.Background(), fetch, "", "to"); err == nil {
		t.Fatal("Compare accepted an empty running reference")
	}
}
