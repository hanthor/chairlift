package imageinfo

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The TunaOS case from the feature request: a downstream image that is not
// one of the three Bluefin-family ones becomes switchable by shipping a
// channels.yml, with no code change.
const tunaOSTable = `
images:
  ghcr.io/tuna-os/tromso:
    stable_tags: [latest, stable]
    testing_tags: [testing]
    to_testing:
      latest: testing
      stable: testing
    to_stable:
      testing: stable
`

func applyTable(t *testing.T, document string) {
	t.Helper()

	table, err := ParseTable(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ParseTable() error = %v, want nil", err)
	}
	previous := activeTable
	activeTable = table
	t.Cleanup(func() { activeTable = previous })
}

func TestOverrideAddsADownstreamImage(t *testing.T) {
	applyTable(t, tunaOSTable)

	info := Info{Name: "tromso", Tag: "latest", Ref: "ostree-image-signed:docker://ghcr.io/tuna-os/tromso"}
	if got := info.Channel(); got != ChannelStable {
		t.Errorf("Channel() = %q, want %q", got, ChannelStable)
	}

	target, ok := info.SwitchTarget(ChannelTesting)
	if !ok {
		t.Fatal("SwitchTarget(testing) ok = false, want true for an overridden image")
	}
	if target != "ghcr.io/tuna-os/tromso:testing" {
		t.Errorf("SwitchTarget(testing) = %q, want %q", target, "ghcr.io/tuna-os/tromso:testing")
	}

	back := Info{Name: "tromso", Tag: "testing", Ref: "docker://ghcr.io/tuna-os/tromso"}
	if target, ok := back.SwitchTarget(ChannelStable); !ok || target != "ghcr.io/tuna-os/tromso:stable" {
		t.Errorf("SwitchTarget(stable) = (%q, %v), want (%q, true)", target, ok, "ghcr.io/tuna-os/tromso:stable")
	}
}

// An override must add to the built-ins rather than replace the whole table,
// or shipping a TunaOS channels.yml would silently break Dakota.
func TestOverrideKeepsBuiltinImages(t *testing.T) {
	applyTable(t, tunaOSTable)

	dakota := Info{Name: "dakota", Tag: "latest", Ref: "docker://ghcr.io/projectbluefin/dakota"}
	if target, ok := dakota.SwitchTarget(ChannelTesting); !ok || target != "ghcr.io/projectbluefin/dakota:testing" {
		t.Errorf("Dakota SwitchTarget = (%q, %v), want the built-in mapping intact", target, ok)
	}

	images := KnownImages()
	for _, want := range []string{
		"ghcr.io/projectbluefin/bluefin-lts",
		"ghcr.io/projectbluefin/dakota",
		"ghcr.io/tuna-os/tromso",
		"ghcr.io/ublue-os/bluefin",
	} {
		if !contains(images, want) {
			t.Errorf("KnownImages() = %v, want it to include %q", images, want)
		}
	}
}

// Replacing a built-in entry outright is how a downstream removes a mapping
// it does not publish, not just adds one.
func TestOverrideCanReplaceABuiltinEntry(t *testing.T) {
	applyTable(t, `
images:
  ghcr.io/projectbluefin/dakota:
    stable_tags: [latest]
    testing_tags: [next]
    to_testing:
      latest: next
    to_stable:
      next: latest
`)

	info := Info{Name: "dakota", Tag: "latest", Ref: "docker://ghcr.io/projectbluefin/dakota"}
	target, ok := info.SwitchTarget(ChannelTesting)
	if !ok || target != "ghcr.io/projectbluefin/dakota:next" {
		t.Fatalf("SwitchTarget = (%q, %v), want the override's tag", target, ok)
	}

	// "stable" was in the built-in entry and is not in the override, so it
	// must no longer be recognized.
	stale := Info{Name: "dakota", Tag: "stable", Ref: "docker://ghcr.io/projectbluefin/dakota"}
	if got := stale.Channel(); got != ChannelUnknown {
		t.Errorf("Channel() for a tag the override dropped = %q, want %q", got, ChannelUnknown)
	}
}

// Every rejection below would otherwise produce a `bootc switch` at a
// reference that does not exist, or a one-way switch a host cannot undo.
func TestParseTableRejectsUnusableOverrides(t *testing.T) {
	tests := []struct {
		name     string
		document string
		wantHas  string
	}{
		{
			name:     "unknown key",
			document: "images:\n  ghcr.io/x/y:\n    stabel_tags: [latest]\n",
			wantHas:  "parsing channel table",
		},
		{
			name:     "not a registry path",
			document: "images:\n  tromso:\n    stable_tags: [latest]\n    testing_tags: [testing]\n",
			wantHas:  "full registry path",
		},
		{
			name:     "key carries a tag",
			document: "images:\n  ghcr.io/x/y:latest:\n    stable_tags: [latest]\n    testing_tags: [testing]\n",
			wantHas:  "without a tag",
		},
		{
			name:     "no testing tags",
			document: "images:\n  ghcr.io/x/y:\n    stable_tags: [latest]\n",
			wantHas:  "needs both stable_tags and testing_tags",
		},
		{
			name:     "to_testing source is not a stable tag",
			document: "images:\n  ghcr.io/x/y:\n    stable_tags: [latest]\n    testing_tags: [testing]\n    to_testing:\n      nightly: testing\n    to_stable:\n      testing: latest\n",
			wantHas:  "not in stable_tags",
		},
		{
			name:     "to_testing target is not a testing tag",
			document: "images:\n  ghcr.io/x/y:\n    stable_tags: [latest]\n    testing_tags: [testing]\n    to_testing:\n      latest: nightly\n    to_stable:\n      testing: latest\n",
			wantHas:  "not in testing_tags",
		},
		{
			name:     "one-way switch with no way back",
			document: "images:\n  ghcr.io/x/y:\n    stable_tags: [latest]\n    testing_tags: [testing]\n    to_testing:\n      latest: testing\n",
			wantHas:  "no to_stable mapping",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTable(strings.NewReader(test.document))
			if err == nil {
				t.Fatal("ParseTable() error = nil, want a rejection")
			}
			if !strings.Contains(err.Error(), test.wantHas) {
				t.Errorf("ParseTable() error = %q, want it to mention %q", err, test.wantHas)
			}
		})
	}
}

func TestParseTableAcceptsAnEmptyDocument(t *testing.T) {
	for _, document := range []string{"", "images:\n", "images: {}\n"} {
		table, err := ParseTable(strings.NewReader(document))
		if err != nil {
			t.Fatalf("ParseTable(%q) error = %v, want nil", document, err)
		}
		if !reflect.DeepEqual(table, builtinTable()) {
			t.Errorf("ParseTable(%q) changed the table, want the built-ins unchanged", document)
		}
	}
}

func TestLoadTableAppliesTheFirstExistingCandidate(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "etc-channels.yml")
	second := filepath.Join(dir, "usr-channels.yml")

	if err := os.WriteFile(second, []byte(tunaOSTable), 0o644); err != nil {
		t.Fatalf("writing maintainer table: %v", err)
	}

	t.Cleanup(ResetTable)

	// Only the second candidate exists — it must be applied.
	applied, err := LoadTable([]string{first, second})
	if err != nil {
		t.Fatalf("LoadTable() error = %v, want nil", err)
	}
	if applied != second {
		t.Fatalf("LoadTable() applied %q, want %q", applied, second)
	}
	if !contains(KnownImages(), "ghcr.io/tuna-os/tromso") {
		t.Error("LoadTable() did not apply the maintainer table")
	}

	// Now the administrator file exists and must win.
	adminTable := strings.ReplaceAll(tunaOSTable, "tromso", "razorfin")
	if err := os.WriteFile(first, []byte(adminTable), 0o644); err != nil {
		t.Fatalf("writing administrator table: %v", err)
	}
	applied, err = LoadTable([]string{first, second})
	if err != nil {
		t.Fatalf("LoadTable() error = %v, want nil", err)
	}
	if applied != first {
		t.Fatalf("LoadTable() applied %q, want the administrator file %q", applied, first)
	}
	images := KnownImages()
	if !contains(images, "ghcr.io/tuna-os/razorfin") {
		t.Error("administrator table was not applied")
	}
	if contains(images, "ghcr.io/tuna-os/tromso") {
		t.Error("the maintainer table was also applied; only the first existing candidate may win")
	}
}

func TestLoadTableWithNoCandidatesKeepsBuiltins(t *testing.T) {
	t.Cleanup(ResetTable)

	applied, err := LoadTable([]string{filepath.Join(t.TempDir(), "absent.yml")})
	if err != nil {
		t.Fatalf("LoadTable() error = %v, want nil for an absent override", err)
	}
	if applied != "" {
		t.Errorf("LoadTable() applied %q, want \"\"", applied)
	}
	if !reflect.DeepEqual(activeTable, builtinTable()) {
		t.Error("LoadTable() changed the active table when no candidate existed")
	}
}

// A broken override must not degrade into a partially applied table: the
// previously active one stays in force and the caller reports the error.
func TestLoadTableLeavesTheActiveTableOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "channels.yml")
	if err := os.WriteFile(path, []byte("images:\n  ghcr.io/x/y:\n    stable_tags: [latest]\n"), 0o644); err != nil {
		t.Fatalf("writing broken table: %v", err)
	}

	t.Cleanup(ResetTable)
	before := builtinTable()

	if _, err := LoadTable([]string{path}); err == nil {
		t.Fatal("LoadTable() error = nil, want a rejection for an invalid override")
	}
	if !reflect.DeepEqual(activeTable, before) {
		t.Error("LoadTable() mutated the active table despite failing")
	}
}

// The GUI and the privileged helper must resolve the same table, so both
// read the same fixed, root-owned locations. A user-writable candidate here
// would let a local user redirect a PolicyKit-authenticated bootc switch.
func TestSystemTablePathsAreRootOwnedLocations(t *testing.T) {
	want := []string{
		"/etc/chairlift/channels.yml",
		"/usr/share/chairlift/channels.yml",
	}
	if !reflect.DeepEqual(SystemTablePaths, want) {
		t.Fatalf("SystemTablePaths = %v, want %v", SystemTablePaths, want)
	}
	for _, path := range SystemTablePaths {
		if !strings.HasPrefix(path, "/") {
			t.Errorf("channel table path %q is relative; it must not resolve against the working directory", path)
		}
	}
}

func TestResetTableRestoresTheBuiltins(t *testing.T) {
	applyTable(t, tunaOSTable)
	ResetTable()
	if contains(KnownImages(), "ghcr.io/tuna-os/tromso") {
		t.Error("ResetTable() left the override in place")
	}
	if !contains(KnownImages(), "ghcr.io/projectbluefin/dakota") {
		t.Error("ResetTable() dropped the built-in images")
	}
}
