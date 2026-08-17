// Package gpu identifies the graphics hardware in this machine, so ChairLift
// can tell whether the running OS image carries the right driver.
//
// It exists for one decision: Universal Blue publishes a `-nvidia` variant of
// each image, and a machine with an NVIDIA card running the plain image has
// working graphics but no proprietary driver. Detecting the card is what lets
// ChairLift offer the matching image instead of asking the user to know which
// variant they need.
//
// Detection is by PCI vendor ID from sysfs, not by running a vendor tool.
// `nvidia-smi` only exists once the driver is already installed, so using it
// would make the check answer "no NVIDIA card" on exactly the machines that
// need the offer. Reading /sys/class/drm needs no privilege and no vendor
// software.
package gpu

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Vendor identifies a graphics chip vendor.
type Vendor string

const (
	VendorNVIDIA Vendor = "nvidia"
	VendorAMD    Vendor = "amd"
	VendorIntel  Vendor = "intel"
	// VendorNone means no discrete or integrated GPU was identified — a
	// virtual machine with a paravirtual display, or a sysfs layout this
	// package does not understand. It is not an error, and it must not
	// produce an image recommendation.
	VendorNone Vendor = "none"
)

// DisplayName returns the human-readable vendor name.
func (v Vendor) DisplayName() string {
	switch v {
	case VendorNVIDIA:
		return "NVIDIA"
	case VendorAMD:
		return "AMD"
	case VendorIntel:
		return "Intel"
	default:
		return "None detected"
	}
}

// PCI vendor IDs as sysfs reports them.
const (
	pciNVIDIA = "0x10de"
	pciAMD    = "0x1002"
	pciIntel  = "0x8086"
)

// drmRoot is the sysfs directory enumerating DRM devices.
const drmRoot = "/sys/class/drm"

// Set is the complete set of vendors present, which matters because hybrid
// laptops have two: an Intel or AMD integrated chip plus an NVIDIA discrete
// one. Reporting only the first found would classify a hybrid laptop by
// whichever card sysfs happened to list first.
type Set struct {
	NVIDIA bool
	AMD    bool
	Intel  bool
}

// Vendors returns the detected vendors in a stable order.
func (s Set) Vendors() []Vendor {
	var vendors []Vendor
	if s.NVIDIA {
		vendors = append(vendors, VendorNVIDIA)
	}
	if s.AMD {
		vendors = append(vendors, VendorAMD)
	}
	if s.Intel {
		vendors = append(vendors, VendorIntel)
	}
	return vendors
}

// Primary returns the vendor whose driver choice determines which OS image
// variant is wanted. NVIDIA wins whenever it is present, including on a
// hybrid laptop, because it is the only one of the three whose driver is
// shipped as a separate image; AMD and Intel are handled by the kernel in
// every variant.
func (s Set) Primary() Vendor {
	switch {
	case s.NVIDIA:
		return VendorNVIDIA
	case s.AMD:
		return VendorAMD
	case s.Intel:
		return VendorIntel
	default:
		return VendorNone
	}
}

// Describe returns a human-readable summary of everything detected.
func (s Set) Describe() string {
	vendors := s.Vendors()
	if len(vendors) == 0 {
		return "No graphics hardware detected"
	}
	names := make([]string, 0, len(vendors))
	for _, vendor := range vendors {
		names = append(names, vendor.DisplayName())
	}
	return strings.Join(names, " + ")
}

// Classify builds a Set from the PCI vendor IDs read from sysfs. It is the
// pure half of detection: every hardware combination is covered by a table
// test without needing the hardware.
func Classify(vendorIDs []string) Set {
	var set Set
	for _, id := range vendorIDs {
		switch strings.ToLower(strings.TrimSpace(id)) {
		case pciNVIDIA:
			set.NVIDIA = true
		case pciAMD:
			set.AMD = true
		case pciIntel:
			set.Intel = true
		}
	}
	return set
}

// readVendorIDs is an injection seam for the sysfs walk, so Detect is
// testable on any machine. Its production value reads the real /sys tree.
var readVendorIDs = sysfsVendorIDs

// SetVendorIDs replaces the sysfs read Detect performs.
//
// It exists so the documentation walkthrough can capture the graphics-driver
// row on a machine whose hardware differs from the one being documented. It
// is called exclusively from ChairLift's chairlift_e2e-tagged build; no
// released binary contains a call site, which internal/installcheck asserts.
//
// Like the other walkthrough stubs it is confined to a read-only,
// display-side classification: the privileged helper derives the driver image
// from the system image descriptor and the published-variant table, never
// from this.
// It returns the previous reader so a caller can restore it, matching
// autoupdate.SetProbe's shape — an exported setter with no way back would
// leave a process permanently reporting hardware it does not have. A nil or
// empty list is rejected rather than being treated as "no GPU", since that is
// indistinguishable from a detection failure.
func SetVendorIDs(ids []string) (previous func() []string) {
	previous = readVendorIDs
	if len(ids) == 0 {
		return previous
	}
	replacement := append([]string(nil), ids...)
	readVendorIDs = func() []string { return replacement }
	return previous
}

// Detect identifies the graphics hardware in this machine.
func Detect() Set {
	return Classify(readVendorIDs())
}

// sysfsVendorIDs returns the PCI vendor ID of every DRM card device. Entries
// containing "-" are connectors (card0-DP-1) rather than devices and are
// skipped; a card whose vendor file is unreadable is skipped rather than
// failing the whole scan, since one odd device must not hide the others.
func sysfsVendorIDs() []string {
	entries, err := os.ReadDir(drmRoot)
	if err != nil {
		return nil
	}

	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(drmRoot, name, "device", "vendor"))
		if err != nil {
			continue
		}
		ids = append(ids, strings.TrimSpace(string(data)))
	}
	sort.Strings(ids)
	return ids
}
