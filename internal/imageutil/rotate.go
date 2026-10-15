// Package imageutil provides small, dependency-free image transforms that
// are shared between the Fyne canvas preview and the print rasterizer.
//
// The package deliberately knows nothing about documents, Fyne, or printer
// protocols. Callers pass an already-decoded image.Image and receive a new
// image.Image; they are responsible for deciding when to decode and when to
// cache the result.
package imageutil

import "image"

// Rotate returns a new image rotated clockwise by the given number of
// degrees.
//
// Multiples of 90° are normalized modulo 360 before dispatch: -90 becomes
// 270, 450 becomes 90, 720 becomes 0, and so on. Only the four quadrant
// angles (0, 90, 180, 270) are meaningful and produce an actual rotation.
//
// Any value that does not reduce to a quadrant angle — 45, 100, -45, 314,
// and other non-multiples of 90, including corrupted metadata — is not
// silently rounded or approximated. Instead src is returned unchanged,
// without allocating a new buffer, so callers can detect an unsupported
// input by comparing the returned image pointer to the input pointer.
//
// When the normalized angle is 0 the input is returned unchanged, again
// without allocating, which keeps the common case — images that have never
// been rotated — free of additional memory cost.
//
// Rotations of 90 and 270 swap the width and height of the returned image,
// which is the caller's responsibility to reflect in document bounds when
// needed.
func Rotate(src image.Image, degrees int) image.Image {
	if src == nil {
		return nil
	}
	d := ((degrees % 360) + 360) % 360
	switch d {
	case 90:
		return rotate90(src)
	case 180:
		return rotate180(src)
	case 270:
		return rotate270(src)
	default:
		return src
	}
}

func rotate90(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// 90° clockwise: (x, y) -> (h-1-y, x)
			dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate180(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate270(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// 270° clockwise (== 90° counter-clockwise): (x, y) -> (y, w-1-x)
			dst.Set(y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
