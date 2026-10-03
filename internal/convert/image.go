package convert

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // register decoders for image.Decode
	"image/jpeg"
	_ "image/png"
)

const (
	// maxEdge is the longest side, in pixels, the model looks at; bigger
	// images are only downsampled by the API anyway
	maxEdge = 2576
	// maxImageBytes keeps an image under the API's 5 MB limit once it is
	// base64-encoded (which adds a third)
	maxImageBytes = 3_500_000
)

// fitImage shrinks an image that is larger than the model needs, or too
// big to send, and re-encodes it as JPEG. Small images are returned as they
// are, with their own media type.
func fitImage(data []byte, mediaType string) ([]byte, string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if len(data) > maxImageBytes {
			return nil, "", fmt.Errorf("image is %d bytes, too big to send, and can't be shrunk: %v", len(data), err)
		}
		return data, mediaType, nil // a format Go can't decode, e.g. WebP
	}
	if max(cfg.Width, cfg.Height) <= maxEdge && len(data) <= maxImageBytes {
		return data, mediaType, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode %s image: %v", format, err)
	}
	small := shrink(img, maxEdge)
	for q := 90; ; q -= 10 {
		var buf bytes.Buffer
		if err = jpeg.Encode(&buf, small, &jpeg.Options{Quality: q}); err != nil {
			return nil, "", err
		}
		if buf.Len() <= maxImageBytes || q <= 50 {
			return buf.Bytes(), "image/jpeg", nil
		}
	}
}

// shrink scales an image down so its longest side is at most edge pixels,
// averaging each block of source pixels into one (a box filter, which
// keeps thin staff lines visible instead of skipping them).
func shrink(src image.Image, edge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := float64(edge) / float64(max(w, h))
	if scale >= 1 {
		return src
	}
	dw, dh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := b.Min.Y+y*h/dh, b.Min.Y+(y+1)*h/dh
		for x := 0; x < dw; x++ {
			x0, x1 := b.Min.X+x*w/dw, b.Min.X+(x+1)*w/dw
			var r, g, bl, a, n uint64
			for sy := y0; sy < max(y1, y0+1); sy++ {
				for sx := x0; sx < max(x1, x0+1); sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r, g, bl, a, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca), n+1
				}
			}
			dst.Set(x, y, color.RGBA64{uint16(r / n), uint16(g / n), uint16(bl / n), uint16(a / n)})
		}
	}
	return dst
}
