package convert

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngOf(w, h int) []byte {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.SetGray(x, h/2, color.Gray{255})
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestFitImageLeavesSmallImagesAlone(t *testing.T) {
	data := pngOf(100, 50)
	got, mt, err := fitImage(data, "image/png")
	if err != nil || mt != "image/png" || !bytes.Equal(got, data) {
		t.Fatalf("small image changed: type %q, err %v", mt, err)
	}
}

func TestFitImageShrinksLargeImages(t *testing.T) {
	got, mt, err := fitImage(pngOf(maxEdge*2, maxEdge), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if mt != "image/jpeg" || cfg.Width != maxEdge || cfg.Height != maxEdge/2 {
		t.Fatalf("got %s %dx%d, want image/jpeg %dx%d", mt, cfg.Width, cfg.Height, maxEdge, maxEdge/2)
	}
}

func TestFitImagePassesThroughUndecodable(t *testing.T) {
	data := []byte("RIFF....WEBP")
	if got, mt, err := fitImage(data, "image/webp"); err != nil || mt != "image/webp" || !bytes.Equal(got, data) {
		t.Fatalf("undecodable small image should pass through, got %q %v", mt, err)
	}
}
