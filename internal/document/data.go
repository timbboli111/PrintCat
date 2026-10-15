package document

type TextData struct {
	Content   string `json:"content"`
	FontSize  Unit   `json:"fontSize"`
	Alignment string `json:"alignment,omitempty"`
}

// ImageData carries the pixel payload of an image element together with
// rotation metadata applied at render time.
//
// Rotation is expressed in degrees, clockwise, and must be one of 0, 90,
// 180, 270. The zero value means "no rotation", so documents serialized
// before this field existed deserialize to 0 and render exactly as before.
// The source bytes in Data are never rewritten for the purpose of rotation:
// both the Fyne preview and the print rasterizer apply Rotation to the
// decoded image, which keeps the two views in agreement and avoids
// cumulative JPEG generation loss.
//
// BaseSize is the unrotated, unconstrained physical size of the image in
// document units (micrometres). It is used by the editor to preserve the
// element's intrinsic size across rotations: when a rotation forces a
// fit-to-page scale-down, the next rotation recomputes bounds from BaseSize
// rather than from the currently-shrunk Bounds, so repeated rotations do
// not shrink the element further. Legacy documents have BaseSize zero; the
// editor lazily populates it from the current Bounds on the first rotation,
// which is correct because legacy elements always have Rotation=0 and
// therefore their current bounds ARE the unrotated size.
type ImageData struct {
	Data     []byte `json:"data"`
	MimeType string `json:"mimeType"`
	Rotation int    `json:"rotation,omitempty"`
	BaseSize Size   `json:"baseSize,omitempty"`
}
