package homeicons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestImportCropsAndStores(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 300, 100))
	for x := 0; x < 300; x++ {
		for y := 0; y < 100; y++ {
			src.Set(x, y, color.NRGBA{200, 40, 40, 255})
		}
	}
	var in bytes.Buffer
	_ = png.Encode(&in, src)
	dir := t.TempDir()
	name, err := Import(dir, in.Bytes())
	if err != nil || !ValidFile(name) {
		t.Fatalf("import failed: %q %v", name, err)
	}
	if !ValidValue(dir, "i:"+name) || ValidValue(dir, "i:../x.png") || !ValidValue(dir, "b:lamp") || ValidValue(dir, "b:nope") {
		t.Fatal("value validation is wrong")
	}
	f, err := os.Open(Path(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil || im.Bounds().Dx() != 96 || im.Bounds().Dy() != 96 {
		t.Fatalf("stored icon should be 96x96: %v", err)
	}
	if _, err := Import(dir, []byte("not an image")); err == nil {
		t.Fatal("garbage should be rejected")
	}
}
