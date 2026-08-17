package installcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/frostyard/chairlift/internal/navigation"
)

const (
	walkthroughDoc = "docs/walkthrough.md"
	screenshotDir  = "docs/screenshots"
)

// markdownImage matches an inline image reference and captures its path.
var markdownImage = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

// screenshotName is the file `make screenshots` writes for the page at the
// given zero-based position in navigation.Items().
func screenshotName(index int, page string) string {
	return fmt.Sprintf("%d-%s.png", index+1, page)
}

// A feature cannot land without a screenshot: every page ChairLift can
// navigate to must have one on disk and be shown in the walkthrough.
//
// This is deliberately a referential check rather than a pixel comparison.
// Regenerating and diffing images in CI fails on font hinting and GTK point
// releases, which is churn rather than signal; what actually needs guarding
// is that the document and the captured set stay in step.
func TestWalkthroughDocumentsEveryPage(t *testing.T) {
	doc := readRepoFile(t, walkthroughDoc)

	items := navigation.Items()
	if len(items) == 0 {
		t.Fatal("navigation.Items() is empty; there are no pages to document")
	}

	for index, item := range items {
		name := screenshotName(index, item.Name)
		t.Run(item.Name, func(t *testing.T) {
			path := filepath.Join(RepoRoot(), screenshotDir, name)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("%s has no screenshot at %s/%s: %v\nrun `make screenshots`", item.Name, screenshotDir, name, err)
			}
			if info.Size() == 0 {
				t.Errorf("%s/%s is empty", screenshotDir, name)
			}
			if !strings.Contains(doc, "screenshots/"+name) {
				t.Errorf("%s does not reference screenshots/%s", walkthroughDoc, name)
			}
			// The page's title is how a reader finds the section, so the
			// document must name it.
			if !strings.Contains(doc, item.Title) {
				t.Errorf("%s never mentions the %q page", walkthroughDoc, item.Title)
			}
		})
	}
}

// Every image the document references must exist, or the published
// walkthrough renders broken images.
func TestWalkthroughImageReferencesResolve(t *testing.T) {
	doc := readRepoFile(t, walkthroughDoc)

	matches := markdownImage.FindAllStringSubmatch(doc, -1)
	if len(matches) == 0 {
		t.Fatalf("%s contains no image references", walkthroughDoc)
	}

	for _, match := range matches {
		reference := match[1]
		t.Run(reference, func(t *testing.T) {
			// References are relative to the document's own directory.
			path := filepath.Join(RepoRoot(), "docs", reference)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s references %s, which does not exist: %v", walkthroughDoc, reference, err)
			}
		})
	}
}

// A screenshot nobody references is a stale capture from a page that has
// since been renamed or removed.
func TestNoOrphanedScreenshots(t *testing.T) {
	doc := readRepoFile(t, walkthroughDoc)

	entries, err := os.ReadDir(filepath.Join(RepoRoot(), screenshotDir))
	if err != nil {
		t.Fatalf("reading %s: %v", screenshotDir, err)
	}

	found := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".png") {
			continue
		}
		found++
		if !strings.Contains(doc, "screenshots/"+entry.Name()) {
			t.Errorf("%s/%s is not referenced by %s; it is a stale capture", screenshotDir, entry.Name(), walkthroughDoc)
		}
	}
	if found != len(navigation.Items()) {
		t.Errorf("%s holds %d screenshots, want one per navigable page (%d)", screenshotDir, found, len(navigation.Items()))
	}
}

// The capture byproducts must never be committed: the .xwd dumps are large,
// and the log and stub files are per-run scratch.
func TestScreenshotDirectoryHoldsOnlyImages(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(RepoRoot(), screenshotDir))
	if err != nil {
		t.Fatalf("reading %s: %v", screenshotDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("%s/%s is a directory; `make screenshots` should have removed it", screenshotDir, entry.Name())
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".png") {
			t.Errorf("%s/%s is not a screenshot; only PNGs belong here", screenshotDir, entry.Name())
		}
	}
}

// Each feature added to ChairLift has to appear in the walkthrough, or the
// document silently stops being a complete tour. Naming them here is what
// makes "all features are documented" an executable claim rather than a
// promise.
func TestWalkthroughCoversEveryUserFacingFeature(t *testing.T) {
	doc := readRepoFile(t, walkthroughDoc)

	features := []struct {
		name    string
		mention string
	}{
		{name: "update all", mention: "Update All"},
		{name: "automatic updates", mention: "Automatic Updates"},
		{name: "rollback", mention: "Roll Back"},
		{name: "release channel", mention: "Release Channel"},
		{name: "developer mode", mention: "Developer Mode"},
		{name: "gaming mode", mention: "Gaming Mode"},
		{name: "homebrew bundles", mention: "Bundles"},
		{name: "tap trust", mention: "trust the tap"},
		{name: "updex features", mention: "updex"},
		{name: "channel table override", mention: "channels.yml"},
	}

	for _, feature := range features {
		t.Run(feature.name, func(t *testing.T) {
			if !strings.Contains(doc, feature.mention) {
				t.Errorf("%s does not document %s (looked for %q)", walkthroughDoc, feature.name, feature.mention)
			}
		})
	}
}

// The document is only discoverable if it is indexed where the repository
// says documents are indexed.
func TestWalkthroughIsIndexed(t *testing.T) {
	index := readRepoFile(t, filepath.Join("docs", "README.md"))
	if !strings.Contains(index, "walkthrough.md") {
		t.Errorf("docs/README.md does not index %s", walkthroughDoc)
	}

	readme := readRepoFile(t, "README.md")
	if !strings.Contains(readme, "docs/walkthrough.md") {
		t.Errorf("README.md does not link to %s", walkthroughDoc)
	}
}
