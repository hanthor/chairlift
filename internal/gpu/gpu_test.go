package gpu

import (
	"reflect"
	"testing"
)

func TestClassifyCoversEveryHardwareCombination(t *testing.T) {
	tests := []struct {
		name         string
		ids          []string
		want         Set
		wantPrimary  Vendor
		wantDescribe string
	}{
		{
			name:         "nvidia only",
			ids:          []string{"0x10de"},
			want:         Set{NVIDIA: true},
			wantPrimary:  VendorNVIDIA,
			wantDescribe: "NVIDIA",
		},
		{
			name:         "amd only",
			ids:          []string{"0x1002"},
			want:         Set{AMD: true},
			wantPrimary:  VendorAMD,
			wantDescribe: "AMD",
		},
		{
			name:         "intel only",
			ids:          []string{"0x8086"},
			want:         Set{Intel: true},
			wantPrimary:  VendorIntel,
			wantDescribe: "Intel",
		},
		{
			// The case that makes Primary non-trivial: a hybrid laptop has
			// two cards, and classifying it by whichever sysfs listed first
			// would send half of them to the wrong image.
			name:         "hybrid intel plus nvidia",
			ids:          []string{"0x8086", "0x10de"},
			want:         Set{NVIDIA: true, Intel: true},
			wantPrimary:  VendorNVIDIA,
			wantDescribe: "NVIDIA + Intel",
		},
		{
			name:         "hybrid amd plus nvidia",
			ids:          []string{"0x1002", "0x10de"},
			want:         Set{NVIDIA: true, AMD: true},
			wantPrimary:  VendorNVIDIA,
			wantDescribe: "NVIDIA + AMD",
		},
		{
			name:         "amd integrated plus amd discrete",
			ids:          []string{"0x1002", "0x1002"},
			want:         Set{AMD: true},
			wantPrimary:  VendorAMD,
			wantDescribe: "AMD",
		},
		{
			name:         "virtual machine with no known vendor",
			ids:          []string{"0x1234"},
			want:         Set{},
			wantPrimary:  VendorNone,
			wantDescribe: "No graphics hardware detected",
		},
		{
			name:         "nothing at all",
			ids:          nil,
			want:         Set{},
			wantPrimary:  VendorNone,
			wantDescribe: "No graphics hardware detected",
		},
		{
			name:         "uppercase and padded ids",
			ids:          []string{"  0X10DE\n"},
			want:         Set{NVIDIA: true},
			wantPrimary:  VendorNVIDIA,
			wantDescribe: "NVIDIA",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Classify(test.ids)
			if got != test.want {
				t.Errorf("Classify(%v) = %+v, want %+v", test.ids, got, test.want)
			}
			if primary := got.Primary(); primary != test.wantPrimary {
				t.Errorf("Primary() = %q, want %q", primary, test.wantPrimary)
			}
			if describe := got.Describe(); describe != test.wantDescribe {
				t.Errorf("Describe() = %q, want %q", describe, test.wantDescribe)
			}
		})
	}
}

func TestVendorsAreReportedInAStableOrder(t *testing.T) {
	set := Set{NVIDIA: true, AMD: true, Intel: true}
	want := []Vendor{VendorNVIDIA, VendorAMD, VendorIntel}
	if !reflect.DeepEqual(set.Vendors(), want) {
		t.Errorf("Vendors() = %v, want %v", set.Vendors(), want)
	}
	if len(Set{}.Vendors()) != 0 {
		t.Error("Vendors() on an empty set is non-empty")
	}
}

func TestVendorDisplayNames(t *testing.T) {
	tests := map[Vendor]string{
		VendorNVIDIA: "NVIDIA",
		VendorAMD:    "AMD",
		VendorIntel:  "Intel",
		VendorNone:   "None detected",
		Vendor("s3"): "None detected",
	}
	for vendor, want := range tests {
		if got := vendor.DisplayName(); got != want {
			t.Errorf("%q.DisplayName() = %q, want %q", vendor, got, want)
		}
	}
}

func TestDetectUsesTheSysfsSeam(t *testing.T) {
	previous := readVendorIDs
	t.Cleanup(func() { readVendorIDs = previous })

	readVendorIDs = func() []string { return []string{"0x10de", "0x8086"} }
	got := Detect()
	if !got.NVIDIA || !got.Intel {
		t.Errorf("Detect() = %+v, want both NVIDIA and Intel", got)
	}

	readVendorIDs = func() []string { return nil }
	if Detect().Primary() != VendorNone {
		t.Error("Detect() with no devices did not report VendorNone")
	}
}

// A machine with no /sys/class/drm at all — a container, or a kernel without
// DRM — must read as "nothing detected", not panic or error.
func TestSysfsVendorIDsToleratesAMissingTree(t *testing.T) {
	if ids := sysfsVendorIDs(); ids == nil {
		return // the ordinary outcome on a host without the tree
	}
	// On a host that does have it, every entry must look like a PCI vendor.
	for _, id := range sysfsVendorIDs() {
		if len(id) == 0 {
			t.Error("sysfsVendorIDs() returned an empty vendor id")
		}
	}
}

// The exported stub must be restorable and must refuse an empty list, so a
// caller cannot leave the process reporting hardware it does not have, and
// cannot silently turn a detection failure into "no GPU".
func TestSetVendorIDsIsRestorableAndRejectsEmpty(t *testing.T) {
	original := readVendorIDs
	t.Cleanup(func() { readVendorIDs = original })

	previous := SetVendorIDs([]string{"0x10de"})
	if !Detect().NVIDIA {
		t.Fatal("SetVendorIDs did not take effect")
	}

	readVendorIDs = previous
	if Detect().NVIDIA && original == nil {
		t.Error("restoring the returned reader did not undo the stub")
	}

	// An empty or nil list must leave the current reader in place.
	SetVendorIDs([]string{"0x1002"})
	before := Detect()
	SetVendorIDs(nil)
	if Detect() != before {
		t.Error("SetVendorIDs(nil) replaced the reader; it must be rejected")
	}
	SetVendorIDs([]string{})
	if Detect() != before {
		t.Error("SetVendorIDs([]) replaced the reader; it must be rejected")
	}

	// The stub must not alias the caller's slice.
	ids := []string{"0x10de"}
	SetVendorIDs(ids)
	ids[0] = "0x1002"
	if !Detect().NVIDIA {
		t.Error("SetVendorIDs aliased the caller's slice")
	}
}
