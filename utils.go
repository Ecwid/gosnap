package gosnap

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"math/bits"

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
			if right > left && right-left > 1 {
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
	Color color.Color
	Rect  image.Rectangle
}

func (t Masked) At(x, y int) color.Color {
	if t.Rect.Min.X <= x && x < t.Rect.Max.X &&
		t.Rect.Min.Y <= y && y < t.Rect.Max.Y {
		return t.Color
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

func overlay(a, b image.Image) image.Image {
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

// hash.Equal(or1.Hash.Or(or2.Hash), distance) but word-by-word and with an early exit
func OrDistanceFast(hash, or1, or2 []big.Word, distance int) bool {
	maxBits := max(len(hash), max(len(or1), len(or2)))
	currentDistance := 0

	for i := 0; i < maxBits; i++ {
		var hashWord, orWord1, orWord2 big.Word
		if i < len(hash) {
			hashWord = hash[i]
		}
		if i < len(or1) {
			orWord1 = or1[i]
		}
		if i < len(or2) {
			orWord2 = or2[i]
		}
		currentDistance += bits.OnesCount(uint(hashWord) ^ (uint(orWord1) | uint(orWord2)))
		if currentDistance > distance {
			break
		}
	}
	return currentDistance <= distance
}
