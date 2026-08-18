package sbom

import (
	"context"
	"fmt"
	"strings"
)

// FetchFunc returns the SBOM bytes attached to an image reference. It is the
// seam the changelog is built on: production passes RegistryClient.Fetch, and
// every gated test passes a function over checked-in fixtures, so nothing in
// `make ci` reaches the network.
type FetchFunc func(ctx context.Context, ref string) ([]byte, error)

// PinnedReference rewrites an image reference to name an exact digest, so
// the two SBOMs compared are the two images actually involved rather than
// whatever the tags point at by the time the request lands. A missing digest
// leaves the reference alone.
func PinnedReference(image, digest string) string {
	if image == "" {
		return ""
	}
	if digest == "" {
		return image
	}
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	if slash := strings.LastIndex(image, "/"); slash >= 0 {
		if colon := strings.LastIndex(image[slash:], ":"); colon >= 0 {
			image = image[:slash+colon]
		}
	}
	return image + "@" + digest
}

// Compare fetches both images' SBOMs and returns what differs.
//
// A failure names which side failed. The two sides are not symmetric in
// practice: the running image is always published, while the staged one may
// be an image the registry has since garbage-collected, and "could not read
// the update's SBOM" is a different problem for a user than "could not read
// this system's".
func Compare(ctx context.Context, fetch FetchFunc, fromRef, toRef string) (Result, error) {
	if fromRef == "" || toRef == "" {
		return Result{}, fmt.Errorf("both a running and a staged image reference are needed")
	}

	fromData, err := fetch(ctx, fromRef)
	if err != nil {
		return Result{}, fmt.Errorf("reading the running image's SBOM: %w", err)
	}
	from, err := Parse(fromData)
	if err != nil {
		return Result{}, fmt.Errorf("reading the running image's SBOM: %w", err)
	}

	toData, err := fetch(ctx, toRef)
	if err != nil {
		return Result{}, fmt.Errorf("reading the staged image's SBOM: %w", err)
	}
	to, err := Parse(toData)
	if err != nil {
		return Result{}, fmt.Errorf("reading the staged image's SBOM: %w", err)
	}

	return Diff(from, to), nil
}
