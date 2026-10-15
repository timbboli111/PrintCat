package imageutil

import (
	"image"
	"image/color"
	"testing"
)

// twoPixelImage returns a 2x1 image with a distinctive color on each pixel:
// the left pixel is red, the right pixel is blue. Reading these two pixels
// after rotation is enough to verify orientation.
func twoPixelImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 0, color.NRGBA{B: 255, A: 255})
	return img
}

func TestRotateZeroReturnsInputUnchanged(t *testing.T) {
	src := twoPixelImage()
	got := Rotate(src, 0)
	if got != src {
		t.Error("Rotate(0) must return the same image without allocating")
	}
}

func TestRotate360ReturnsInputUnchanged(t *testing.T) {
	src := twoPixelImage()
	got := Rotate(src, 360)
	if got != src {
		t.Error("Rotate(360) must return the same image without allocating")
	}
}

func TestRotate90Clockwise(t *testing.T) {
	got := Rotate(twoPixelImage(), 90)
	b := got.Bounds()
	if b.Dx() != 1 || b.Dy() != 2 {
		t.Fatalf("bounds = %dx%d, want 1x2", b.Dx(), b.Dy())
	}
	top := color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA)
	bottom := color.NRGBAModel.Convert(got.At(0, 1)).(color.NRGBA)
	if top.R < 200 || top.B > 50 {
		t.Errorf("top pixel = %+v, want red", top)
	}
	if bottom.B < 200 || bottom.R > 50 {
		t.Errorf("bottom pixel = %+v, want blue", bottom)
	}
}

func TestRotate180(t *testing.T) {
	got := Rotate(twoPixelImage(), 180)
	b := got.Bounds()
	if b.Dx() != 2 || b.Dy() != 1 {
		t.Fatalf("bounds = %dx%d, want 2x1", b.Dx(), b.Dy())
	}
	left := color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA)
	right := color.NRGBAModel.Convert(got.At(1, 0)).(color.NRGBA)
	if left.B < 200 || left.R > 50 {
		t.Errorf("left pixel = %+v, want blue", left)
	}
	if right.R < 200 || right.B > 50 {
		t.Errorf("right pixel = %+v, want red", right)
	}
}

func TestRotate270Clockwise(t *testing.T) {
	got := Rotate(twoPixelImage(), 270)
	b := got.Bounds()
	if b.Dx() != 1 || b.Dy() != 2 {
		t.Fatalf("bounds = %dx%d, want 1x2", b.Dx(), b.Dy())
	}
	top := color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA)
	bottom := color.NRGBAModel.Convert(got.At(0, 1)).(color.NRGBA)
	if top.B < 200 || top.R > 50 {
		t.Errorf("top pixel = %+v, want blue", top)
	}
	if bottom.R < 200 || bottom.B > 50 {
		t.Errorf("bottom pixel = %+v, want red", bottom)
	}
}

func TestRotateFourNinetiesReturnsToOriginalOrientation(t *testing.T) {
	img := twoPixelImage()
	for i := 0; i < 4; i++ {
		img = Rotate(img, 90)
	}
	b := img.Bounds()
	if b.Dx() != 2 || b.Dy() != 1 {
		t.Fatalf("after 4 rotations bounds = %dx%d, want 2x1", b.Dx(), b.Dy())
	}
	left := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	right := color.NRGBAModel.Convert(img.At(1, 0)).(color.NRGBA)
	if left.R < 200 {
		t.Errorf("after 4 rotations left pixel = %+v, want red", left)
	}
	if right.B < 200 {
		t.Errorf("after 4 rotations right pixel = %+v, want blue", right)
	}
}

// Multiples of 90° are normalized modulo 360 before dispatch. The test uses
// the canonical rotation for the equivalent positive angle and compares the
// bounds only (pixel content equality is already covered elsewhere).
func TestRotateNormalizesMultiplesOf90(t *testing.T) {
	src := twoPixelImage()
	cases := []struct {
		input     int
		canonical int
	}{
		{-90, 270},
		{-180, 180},
		{-270, 90},
		{-360, 0},
		{450, 90},
		{540, 180},
		{630, 270},
		{720, 0},
		{-450, 270},
	}
	for _, tc := range cases {
		got := Rotate(src, tc.input)
		if tc.canonical == 0 {
			if got != src {
				t.Errorf("Rotate(%d) must return src unchanged", tc.input)
			}
			continue
		}
		want := Rotate(src, tc.canonical)
		if got.Bounds() != want.Bounds() {
			t.Errorf("Rotate(%d) bounds = %v, want %v (canonical %d)",
				tc.input, got.Bounds(), want.Bounds(), tc.canonical)
		}
	}
}

// Non-multiples of 90° return src unchanged, without allocating or
// pretending a rotation occurred. This is the contract that keeps
// corrupted metadata from silently producing a rotated raster.
func TestRotateUnsupportedAngleReturnsSourceUnchanged(t *testing.T) {
	src := twoPixelImage()
	for _, deg := range []int{45, 100, 200, 314, -45, -100, 30, 60, 91} {
		got := Rotate(src, deg)
		if got != src {
			t.Errorf("Rotate(%d) must return the same source image", deg)
		}
	}
}

func TestRotateNilImage(t *testing.T) {
	if got := Rotate(nil, 90); got != nil {
		t.Error("Rotate(nil, 90) must return nil")
	}
	if got := Rotate(nil, 0); got != nil {
		t.Error("Rotate(nil, 0) must return nil")
	}
}
