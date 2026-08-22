package climg

// LightInfo holds lighting metadata for images that emit light or darkness.
// Color stores RGBA components. For lightcasters only RGB is used and alpha
// should be 255. For darkcasters RGB will be zero and alpha specifies the
// darkness intensity. Radius is in pixels and a zero radius indicates the
// image width should be used. Plane mirrors the picture definition plane.
type LightInfo struct {
	Color  [4]byte
	Radius uint16
	Plane  int16
}

// PictDef lighting-related flags.
const (
	PictDefFlagEmitsLight         = 0x0200
	PictDefFlagOnlyAttackPosesLit = 0x0100
	PictDefFlagLightFlicker       = 0x0080
	PictDefFlagLightDarkcaster    = 0x0040
)
