//go:build !chairlift_e2e

package app

// applyImageInfoOverride does nothing in an ordinary build.
//
// The screenshot walkthrough needs ChairLift to render the Bluefin-family
// rows on a CI runner, which is not a Bluefin system. The only way to do
// that is to point the image-descriptor read somewhere else — and a shipped
// binary that honors an environment variable for that would let anyone make
// the Features page claim the machine is running an image it is not. That is
// not privilege escalation (the privileged helper always re-reads the real
// imageinfo.DescriptorPath and never consults this), but it is a shipped
// binary telling a user something false about their own system, which is not
// worth a test's convenience.
//
// So the override lives behind the chairlift_e2e build tag in
// imageinfo_override_e2e.go, and `make e2e` is the only target that sets it.
// `make ci` builds untagged for both architectures, so the production binary
// provably has no code path that reads the variable.
func applyImageInfoOverride() {}
