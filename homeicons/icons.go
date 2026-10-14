package homeicons

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/image/draw"
)

var Builtin = []string{"bulb", "lamp", "strip", "plug", "tv", "fan", "bed", "sofa", "kitchen", "bath", "desk", "plant", "sun", "moon", "door", "home"}

var fileName = regexp.MustCompile(`^[0-9a-f]{16}\.png$`)

func Dir(config string) string { return filepath.Join(config, "icons") }

func ValidFile(f string) bool { return fileName.MatchString(f) }

func ValidKey(k string) bool {
	return (strings.HasPrefix(k, "d:") || strings.HasPrefix(k, "r:")) && len(k) > 2 && len(k) < 200
}

func ValidValue(config, v string) bool {
	switch {
	case v == "":
		return true
	case strings.HasPrefix(v, "b:"):
		for _, b := range Builtin {
			if b == v[2:] {
				return true
			}
		}
	case strings.HasPrefix(v, "i:"):
		if !ValidFile(v[2:]) {
			return false
		}
		_, err := os.Stat(filepath.Join(Dir(config), v[2:]))
		return err == nil
	}
	return false
}

func Import(config string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > 8<<20 {
		return "", errors.New("image must be smaller than 8 MB")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("unsupported image (use PNG, JPEG or GIF)")
	}
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	if side < 4 {
		return "", errors.New("image is too small")
	}
	crop := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2, b.Min.X+(b.Dx()-side)/2+side, b.Min.Y+(b.Dy()-side)/2+side)
	const n = 96
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.NRGBA{}), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return "", err
	}
	sum := sha1.Sum(out.Bytes())
	name := hex.EncodeToString(sum[:8]) + ".png"
	dir := Dir(config)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, name), out.Bytes(), 0o600); err != nil {
		return "", err
	}
	return name, nil
}

func Path(config, file string) string {
	if !ValidFile(file) {
		return ""
	}
	return filepath.Join(Dir(config), file)
}
