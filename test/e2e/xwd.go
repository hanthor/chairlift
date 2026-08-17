package e2e

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math/bits"
	"os"
)

// The X Window Dump format, as written by x11-apps' xwd(1).
//
// The walkthrough captures with xwd rather than ImageMagick's import(1)
// because import only works when ImageMagick was compiled with the X11
// delegate, which is not guaranteed — a Homebrew ImageMagick, for one,
// commonly is not, and fails at capture time with "delegate library support
// not built-in (X11)". xwd ships with the X utilities the harness already
// needs for xdpyinfo and xdotool, so decoding its output here removes an
// entire dependency and its build-configuration failure mode.
//
// Only the one variant Xvfb produces is supported: a ZPixmap dump of a
// TrueColor/DirectColor visual at 24 or 32 bits per pixel. Anything else is
// an explicit error rather than a silently misdecoded image.
const (
	xwdHeaderWords = 25 // fixed-size header fields, each a big-endian uint32
	xwdFileVersion = 7
	xwdFormatZ     = 2 // ZPixmap
	xwdTrueColor   = 4
	xwdDirectColor = 5
	xwdColorEntry  = 12 // sizeof(XWDColor)

	xwdByteOrderLSBFirst = 0
	xwdByteOrderMSBFirst = 1
)

type xwdHeader struct {
	HeaderSize    uint32
	FileVersion   uint32
	PixmapFormat  uint32
	PixmapDepth   uint32
	PixmapWidth   uint32
	PixmapHeight  uint32
	XOffset       uint32
	ByteOrder     uint32
	BitmapUnit    uint32
	BitmapBitPad  uint32
	BitmapPad     uint32
	BitsPerPixel  uint32
	BytesPerLine  uint32
	VisualClass   uint32
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	BitsPerRGB    uint32
	ColormapEntry uint32
	NColors       uint32
	WindowWidth   uint32
	WindowHeight  uint32
	WindowX       uint32
	WindowY       uint32
	WindowBorder  uint32
}

// decodeXWDFile reads and decodes an xwd dump into an image.
func decodeXWDFile(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeXWD(data)
}

func decodeXWD(data []byte) (image.Image, error) {
	if len(data) < xwdHeaderWords*4 {
		return nil, fmt.Errorf("xwd: file is %d bytes, too short for a %d-byte header", len(data), xwdHeaderWords*4)
	}

	words := make([]uint32, xwdHeaderWords)
	for i := range words {
		words[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	header := xwdHeader{
		HeaderSize: words[0], FileVersion: words[1], PixmapFormat: words[2],
		PixmapDepth: words[3], PixmapWidth: words[4], PixmapHeight: words[5],
		XOffset: words[6], ByteOrder: words[7], BitmapUnit: words[8],
		BitmapBitPad: words[9], BitmapPad: words[10], BitsPerPixel: words[11],
		BytesPerLine: words[12], VisualClass: words[13], RedMask: words[14],
		GreenMask: words[15], BlueMask: words[16], BitsPerRGB: words[17],
		ColormapEntry: words[18], NColors: words[19], WindowWidth: words[20],
		WindowHeight: words[21], WindowX: words[22], WindowY: words[23],
		WindowBorder: words[24],
	}

	if header.FileVersion != xwdFileVersion {
		return nil, fmt.Errorf("xwd: file version %d, want %d", header.FileVersion, xwdFileVersion)
	}
	if header.PixmapFormat != xwdFormatZ {
		return nil, fmt.Errorf("xwd: pixmap format %d, want ZPixmap (%d)", header.PixmapFormat, xwdFormatZ)
	}
	if header.VisualClass != xwdTrueColor && header.VisualClass != xwdDirectColor {
		return nil, fmt.Errorf("xwd: visual class %d, want TrueColor (%d) or DirectColor (%d)",
			header.VisualClass, xwdTrueColor, xwdDirectColor)
	}
	if header.BitsPerPixel != 24 && header.BitsPerPixel != 32 {
		return nil, fmt.Errorf("xwd: %d bits per pixel, want 24 or 32", header.BitsPerPixel)
	}
	if header.RedMask == 0 || header.GreenMask == 0 || header.BlueMask == 0 {
		return nil, fmt.Errorf("xwd: incomplete channel masks r=%#x g=%#x b=%#x",
			header.RedMask, header.GreenMask, header.BlueMask)
	}
	if header.PixmapWidth == 0 || header.PixmapHeight == 0 {
		return nil, fmt.Errorf("xwd: zero-sized image %dx%d", header.PixmapWidth, header.PixmapHeight)
	}

	// Pixels follow the header and the colormap, whose size is fixed even
	// for a TrueColor visual.
	offset := int(header.HeaderSize) + int(header.NColors)*xwdColorEntry
	if offset < 0 || offset > len(data) {
		return nil, fmt.Errorf("xwd: pixel data starts at %d, past the %d-byte file", offset, len(data))
	}

	stride := int(header.BytesPerLine)
	bytesPerPixel := int(header.BitsPerPixel) / 8
	width, height := int(header.PixmapWidth), int(header.PixmapHeight)
	if stride < width*bytesPerPixel {
		return nil, fmt.Errorf("xwd: bytes_per_line %d is short for %d pixels at %d bytes each", stride, width, bytesPerPixel)
	}
	if need := offset + stride*height; need > len(data) {
		return nil, fmt.Errorf("xwd: need %d bytes of pixel data, file has %d", need, len(data))
	}

	redShift, redBits := maskShift(header.RedMask)
	greenShift, greenBits := maskShift(header.GreenMask)
	blueShift, blueBits := maskShift(header.BlueMask)

	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := data[offset+y*stride:]
		for x := 0; x < width; x++ {
			pixel := readPixel(row[x*bytesPerPixel:], bytesPerPixel, header.ByteOrder)
			frame.SetRGBA(x, y, color.RGBA{
				R: scaleChannel((pixel&header.RedMask)>>redShift, redBits),
				G: scaleChannel((pixel&header.GreenMask)>>greenShift, greenBits),
				B: scaleChannel((pixel&header.BlueMask)>>blueShift, blueBits),
				A: 0xff,
			})
		}
	}
	return frame, nil
}

// readPixel assembles one pixel's raw value from the dump's byte order.
func readPixel(data []byte, bytesPerPixel int, byteOrder uint32) uint32 {
	var pixel uint32
	switch byteOrder {
	case xwdByteOrderMSBFirst:
		for i := 0; i < bytesPerPixel; i++ {
			pixel = pixel<<8 | uint32(data[i])
		}
	case xwdByteOrderLSBFirst:
		for i := bytesPerPixel - 1; i >= 0; i-- {
			pixel = pixel<<8 | uint32(data[i])
		}
	default:
		// Treated as LSBFirst; the header validation above already
		// restricted the formats reaching here.
		for i := bytesPerPixel - 1; i >= 0; i-- {
			pixel = pixel<<8 | uint32(data[i])
		}
	}
	return pixel
}

// maskShift returns the bit offset and width of a channel mask.
func maskShift(mask uint32) (uint32, uint32) {
	shift := uint32(bits.TrailingZeros32(mask))
	return shift, uint32(bits.OnesCount32(mask))
}

// scaleChannel widens a channel value of width bits to a full 8-bit value.
func scaleChannel(value, width uint32) uint8 {
	switch {
	case width == 0:
		return 0
	case width >= 8:
		return uint8(value >> (width - 8))
	default:
		max := uint32(1)<<width - 1
		return uint8(value * 255 / max)
	}
}
