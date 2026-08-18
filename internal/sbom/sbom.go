// Package sbom parses the software bill of materials attached to a bootc
// image and diffs two of them, so ChairLift can answer "what actually
// changes if I take this update?" from the registry rather than from a
// hand-written changelog.
//
// Everything in this file is pure: it turns bytes into a package map and two
// package maps into a diff. The registry round-trip that produces those bytes
// lives in fetch.go behind a seam, so the diff is covered by fixtures rather
// than by a network call.
package sbom

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Packages maps a package name to its version.
type Packages map[string]string

// syftDocument is the shape Universal Blue actually attaches.
type syftDocument struct {
	Artifacts []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"artifacts"`
}

// spdxDocument is the shape the artifact type advertises.
type spdxDocument struct {
	Packages []struct {
		Name        string `json:"name"`
		VersionInfo string `json:"versionInfo"`
	} `json:"packages"`
}

// Parse reads an attached SBOM into a package map.
//
// The referrer is advertised as `application/vnd.spdx+json`, but Universal
// Blue attaches Syft JSON under that artifact type — verified against
// ghcr.io/ublue-os/bluefin:stable on 2026-08-17, whose blob has a top-level
// `artifacts` array and no `packages` at all. Both shapes are therefore
// accepted, and a document matching neither is an error rather than an empty
// map: every field in both schemas is optional, so a silent zero-package
// parse would render an empty changelog on every machine with nothing
// anywhere in the chain reporting a failure. finupdate shipped exactly that
// bug and documents it in its own source.
func Parse(data []byte) (Packages, error) {
	packages := make(Packages)

	var syft syftDocument
	if err := json.Unmarshal(data, &syft); err == nil {
		for _, artifact := range syft.Artifacts {
			if artifact.Name == "" {
				continue
			}
			packages[artifact.Name] = artifact.Version
		}
	}
	if len(packages) > 0 {
		return packages, nil
	}

	var spdx spdxDocument
	if err := json.Unmarshal(data, &spdx); err != nil {
		return nil, fmt.Errorf("parsing SBOM: %w", err)
	}
	for _, pkg := range spdx.Packages {
		if pkg.Name == "" {
			continue
		}
		packages[pkg.Name] = pkg.VersionInfo
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("SBOM contains no packages in either Syft or SPDX form")
	}
	return packages, nil
}

// Change is one package that differs between two images.
type Change struct {
	Name string
	// From is the version on the running image, empty for an addition.
	From string
	// To is the version on the staged image, empty for a removal.
	To string
}

// Result is the categorized difference between two images.
type Result struct {
	// Upgraded, Downgraded, and Changed partition the packages present in
	// both images with differing versions. Changed holds the pairs whose
	// order could not be established — two git hashes, or versions in
	// unrelated formats — because presenting an unknown direction as an
	// upgrade is how a rollback comes to look like an update.
	Upgraded   []Change
	Downgraded []Change
	Changed    []Change
	Added      []Change
	Removed    []Change
}

// Total returns the number of packages that differ.
func (r Result) Total() int {
	return len(r.Upgraded) + len(r.Downgraded) + len(r.Changed) + len(r.Added) + len(r.Removed)
}

// Empty reports whether the two images carry identical packages.
func (r Result) Empty() bool {
	return r.Total() == 0
}

// Diff categorizes the difference between the running and staged images.
// Every returned slice is sorted by package name, so the same pair of images
// always produces the same list.
func Diff(from, to Packages) Result {
	var result Result

	for name, fromVersion := range from {
		toVersion, present := to[name]
		if !present {
			result.Removed = append(result.Removed, Change{Name: name, From: fromVersion})
			continue
		}
		if toVersion == fromVersion {
			continue
		}
		change := Change{Name: name, From: fromVersion, To: toVersion}
		switch CompareVersions(fromVersion, toVersion) {
		case -1:
			result.Upgraded = append(result.Upgraded, change)
		case 1:
			result.Downgraded = append(result.Downgraded, change)
		default:
			result.Changed = append(result.Changed, change)
		}
	}

	for name, toVersion := range to {
		if _, present := from[name]; !present {
			result.Added = append(result.Added, Change{Name: name, To: toVersion})
		}
	}

	for _, changes := range [][]Change{result.Upgraded, result.Downgraded, result.Changed, result.Added, result.Removed} {
		sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	}
	return result
}

// CompareVersions orders two version strings, returning -1 when a sorts
// before b, 1 when it sorts after, and 0 when they are equal or when the
// comparison cannot be made.
//
// It is a segment-wise comparison in the shape rpm uses: runs of digits
// compare numerically (so 10 follows 9), runs of letters compare
// lexically, and separators are skipped. It deliberately does not try to
// order two commit hashes or two version strings in unrelated formats —
// those return 0 and land in Result.Changed, which is honest about not
// knowing rather than guessing a direction.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	// A hash is not a version, and one hash is not "newer" than another.
	if isHash(a) || isHash(b) {
		return 0
	}

	aSegments := versionSegments(a)
	bSegments := versionSegments(b)

	for i := 0; i < len(aSegments) && i < len(bSegments); i++ {
		left, right := aSegments[i], bSegments[i]

		// rpm's tilde rule: a tilde-prefixed segment sorts before
		// everything, which is how Fedora expresses a pre-release
		// (1.0~rc1 precedes 1.0). Without it a release candidate reads as
		// newer than the release it precedes.
		if left == tildeSegment || right == tildeSegment {
			if left == right {
				continue
			}
			if left == tildeSegment {
				return -1
			}
			return 1
		}

		aNumeric := isNumeric(left)
		bNumeric := isNumeric(right)

		switch {
		case aNumeric && bNumeric:
			left = strings.TrimLeft(left, "0")
			right = strings.TrimLeft(right, "0")
			if len(left) != len(right) {
				if len(left) < len(right) {
					return -1
				}
				return 1
			}
			if left != right {
				if left < right {
					return -1
				}
				return 1
			}
		case aNumeric != bNumeric:
			// A numeric segment outranks an alphabetic one, which is what
			// makes 1.0 newer than 1.0rc.
			if aNumeric {
				return 1
			}
			return -1
		default:
			if left != right {
				if left < right {
					return -1
				}
				return 1
			}
		}
	}

	// One version ran out of segments. The longer one is newer, unless its
	// next segment is a tilde — 1.0 is newer than 1.0~rc1.
	switch {
	case len(aSegments) < len(bSegments):
		if bSegments[len(aSegments)] == tildeSegment {
			return 1
		}
		return -1
	case len(aSegments) > len(bSegments):
		if aSegments[len(bSegments)] == tildeSegment {
			return -1
		}
		return 1
	}
	return 0
}

// tildeSegment marks rpm's pre-release separator in a segment list.
const tildeSegment = "~"

// versionSegments splits a version into comparable runs of digits and
// letters, dropping the separators between them.
func versionSegments(version string) []string {
	var segments []string
	var current strings.Builder
	var currentNumeric bool

	flush := func() {
		if current.Len() > 0 {
			segments = append(segments, current.String())
			current.Reset()
		}
	}

	for _, r := range version {
		switch {
		case unicode.IsDigit(r):
			if current.Len() > 0 && !currentNumeric {
				flush()
			}
			currentNumeric = true
			current.WriteRune(r)
		case unicode.IsLetter(r):
			if current.Len() > 0 && currentNumeric {
				flush()
			}
			currentNumeric = false
			current.WriteRune(r)
		case r == '~':
			flush()
			segments = append(segments, tildeSegment)
		default:
			flush()
		}
	}
	flush()
	return segments
}

func isNumeric(segment string) bool {
	for _, r := range segment {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return segment != ""
}

// isHash reports whether a version is a git commit or content hash rather
// than a version number.
func isHash(version string) bool {
	if len(version) < 32 {
		return false
	}
	for _, r := range version {
		if !unicode.Is(unicode.Hex_Digit, r) {
			return false
		}
	}
	return true
}
