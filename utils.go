package gosnap

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/big"
	"strings"

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

func gzUnpack(gzdata string, value any) error {
	gzr, err := gzip.NewReader(strings.NewReader(gzdata))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, gzr); err != nil {
		return err
	}
	return json.Unmarshal(buf.Bytes(), value)
}

func base64Pack(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(body), nil
}

func base64Unpack(data string, value any) error {
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}

func gzPack(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err = gz.Write(body)
	if err != nil {
		return "", err
	}
	if err = gz.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
