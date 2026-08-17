//go:build chairlift_e2e

package app

import (
	"os"

	"github.com/frostyard/chairlift/internal/ublue"
)

// applyImageInfoOverride points the unprivileged image-descriptor read at
// $CHAIRLIFT_IMAGE_INFO. It is compiled only into the chairlift_e2e build
// that `make e2e` produces, never into a released binary — see the
// no-op counterpart in imageinfo_override.go for why.
//
// It is called only from the --dry-run branch, so the walkthrough's session
// additionally cannot execute any mutation. The privileged helper is a
// separate binary built without this tag and resolves its own descriptor
// from imageinfo.DescriptorPath regardless.
func applyImageInfoOverride() {
	if path := os.Getenv("CHAIRLIFT_IMAGE_INFO"); path != "" {
		ublue.SetDescriptorOverride(path)
	}
}
