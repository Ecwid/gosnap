package gosnap

import (
	"image"
	"math"
	"math/big"
	"math/bits"
	"strings"
)

type Hash struct {
	big *big.Int
}

func (h Hash) String() string {
	return h.big.Text(62)
}

func hashString(text string) Hash {
	b := big.NewInt(0)
	b.SetString(text, 62)
	return Hash{big: b}
}

func (h Hash) Equal(other Hash, distance int) bool {
	return h.Xor(other).onesLess(distance)
}

func (h Hash) onesLess(d int) bool {
	return h.onesCount() <= d
}

func (h Hash) onesCount() int {
	var n = 0
	for _, word := range h.big.Bits() {
		n += bits.OnesCount(uint(word))
	}
	return n
}

func (h Hash) Xor(other Hash) Hash {
	df := big.NewInt(0)
	df.Xor(h.big, other.big)
	return Hash{big: df}
}

func (h Hash) Or(other Hash) Hash {
	df := big.NewInt(0)
	df.Or(h.big, other.big)
	return Hash{big: df}
}

func MakeHash(img image.Image, bits uint) Hash {
	px := int(math.Sqrt(float64(bits)))
	gray := grayScale(img, px, px)
	return Hash{big: grayToBigInt(gray)}
}

func (h Hash) MarshalJSON() ([]byte, error) {
	if h.big == nil {
		return []byte("null"), nil
	}
	return []byte(`"` + h.String() + `"`), nil
}

func (h *Hash) UnmarshalJSON(b []byte) (err error) {
	if string(b) == "null" {
		return nil
	}
	str := strings.Trim(string(b), `"`)
	*h = hashString(str)
	return nil
}
