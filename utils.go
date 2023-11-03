package gosnap

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/big"

	"golang.org/x/image/draw"
)

func grayScale(src image.Image, widthPx, heightPx int) *image.Gray {
	rect := image.Rect(0, 0, widthPx, heightPx)
	var gray = image.NewGray(rect)
	draw.BiLinear.Scale(gray, rect, src, src.Bounds(), draw.Over, nil)
	return gray
}

func grayToBigInt(img *image.Gray) *big.Int {
	var (
		x, y, n     int
		left, right uint8
		r           = img.Bounds()
		hash        = big.NewInt(0)
	)
	for y = r.Min.Y; y < r.Max.Y; y++ {
		left = img.GrayAt(r.Min.X, y).Y
		for x = r.Min.X + 1; x < r.Max.X; x++ {
			right = img.GrayAt(x, y).Y
			if right > left {
				hash.SetBit(hash, n, 1)
			}
			n++
			left = right
		}
	}
	return hash
}

type Masked struct {
	image.Image
	clr  color.RGBA
	mask image.Rectangle
}

func (t Masked) At(x, y int) color.Color {
	if t.mask.Min.X <= x && x < t.mask.Max.X &&
		t.mask.Min.Y <= y && y < t.mask.Max.Y {
		return t.clr
	}
	return t.Image.At(x, y)
}

func encodePng(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodePng(body []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(body))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func difference(a, b image.Image) image.Image {
	var (
		ab   = a.Bounds()
		bb   = b.Bounds()
		w, h = max(ab.Dx(), bb.Dx()), max(ab.Dy(), bb.Dy())
		img  = image.NewNRGBA(image.Rect(0, 0, w, h))
		mag  = color.NRGBA{R: 255, G: 0, B: 255, A: 255}
	)
	var (
		c1, c2                 color.Color
		r1, g1, b1, r2, g2, b2 uint32
	)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c1 = a.At(x, y)
			c2 = b.At(x, y)
			r1, g1, b1, _ = c1.RGBA()
			r2, g2, b2, _ = c2.RGBA()
			if (r1-r2 != 0) || (g1-g2 != 0) || (b1-b2 != 0) {
				img.SetNRGBA(x, y, mag)
				continue
			}
			img.Set(x, y, c1)
		}
	}
	return img
}
