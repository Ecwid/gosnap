package gosnap

import (
	"image"
	"math"
	"math/big"
	"math/bits"
	"strings"
)

type Hash struct {
	value *big.Int
}

func (h Hash) String() string {
	return h.value.Text(62)
}

func HashString(text string) Hash {
	return hashString(text)
}

func hashString(text string) Hash {
	b := big.NewInt(0)
	b.SetString(text, 62)
	return Hash{value: b}
}

func (h Hash) Equal(other Hash, distance int) bool {
	_, eq := h.equal(other, distance)
	return eq
}

func (h Hash) equal(other Hash, distance int) (z Hash, equal bool) {
	z = h.Xor(other)
	equal = z.onesCount() <= distance
	return
}

func (h Hash) onesCount() int {
	var n = 0
	for _, word := range h.value.Bits() {
		n += bits.OnesCount(uint(word))
	}
	return n
}

func (h Hash) Xor(other Hash) Hash {
	xor := big.NewInt(0).Xor(h.value, other.value)
	return Hash{value: xor}
}

func (h Hash) Or(other Hash) Hash {
	or := big.NewInt(0).Or(h.value, other.value)
	return Hash{value: or}
}

func MakeGrayAndHash(img image.Image, bits uint) (*image.Gray, Hash) {
	px := int(math.Sqrt(float64(bits)))
	gray := grayScale(img, px, px)
	return gray, Hash{value: grayToBigInt(gray)}
}

func MakeHash(img image.Image, bits uint) Hash {
	_, hash := MakeGrayAndHash(img, bits)
	return hash
}

func (h Hash) MarshalJSON() ([]byte, error) {
	if h.value == nil {
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

func (h *Hash) SquareString(sq int) string {
	s := strings.Builder{}
	pow := (sq * sq) - sq
	sqso := sq - 1
	for n := 0; n < pow; n++ {
		if h.value.Bit(n) == 1 {
			s.WriteByte('1')
		} else {
			s.WriteByte('0')
		}
		if (n+1)%sqso == 0 {
			s.WriteByte('\n')
		}
	}
	return s.String()
}
