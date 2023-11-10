package gosnap

import (
	"errors"
	"fmt"
	"image"
	"sort"
	"strconv"
	"time"

	"github.com/ecwid/gosnap/registry"
)

const (
	dataHash = "Hash"
	keyX     = "X"
	keyY     = "Y"
)

type Snapshot struct {
	Value    image.Image
	Hash     Hash
	Metadata map[string]string
}

func atoi(value string) int {
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return i
}

func (b Snapshot) GetSize() (x int, y int) {
	valueX, ok1 := b.Metadata[keyX]
	valueY, ok2 := b.Metadata[keyY]
	if ok1 && ok2 {
		x = atoi(valueX)
		y = atoi(valueY)
		return x, y
	}
	if b.Value != nil {
		point := b.Value.Bounds().Max
		return point.X, point.Y
	}
	return 0, 0
}

func (s *Snapshot) decode(data registry.Object) error {
	s.Metadata = data.Data
	s.Hash = hashString(data.Data[dataHash])
	var err error
	if data.Body != nil {
		s.Value, err = decodePng(data.Body)
		if err != nil {
			return errors.Join(errors.New("can't decode snapshot png"), err)
		}
	}
	return err
}

func (b Snapshot) encode() (*registry.Object, error) {
	var (
		body []byte
		err  error
	)
	if b.Value != nil {
		body, err = encodePng(b.Value)
		if err != nil {
			return nil, errors.Join(errors.New("can't encode snapshot png"), err)
		}
		x, y := b.GetSize()
		b.Metadata[keyX] = fmt.Sprint(x)
		b.Metadata[keyY] = fmt.Sprint(y)
	}
	data := &registry.Object{
		Body: body,
		Data: b.Metadata,
	}
	data.Data[dataHash] = b.Hash.String()
	return data, nil
}

func (s *Snapshot) Head(key string) error {
	data, err := defaultRegistry.Head(key)
	if err != nil {
		return errors.Join(errors.New("can't pull snapshot"), err)
	}
	s.Metadata = data
	s.Hash = hashString(data[dataHash])
	return nil
}

func (s *Snapshot) Pull(key string) error {
	obj, err := defaultRegistry.Pull(key)
	if err != nil {
		return errors.Join(errors.New("can't pull snapshot"), err)
	}
	return s.decode(*obj)
}

func (s Snapshot) Push(key string) error {
	obj, err := s.encode()
	if err != nil {
		return err
	}
	if err = defaultRegistry.Push(key, *obj); err != nil {
		return errors.Join(errors.New("can't push snapshot"), err)
	}
	return nil
}

type Approval struct {
	Ts       int64  `json:"ts"`
	Hash     Hash   `json:"hash"`
	Approver string `json:"approver"`
}

func (t Approval) Valid() bool {
	return time.Unix(t.Ts, 0).AddDate(0, 2, 0).Compare(time.Now()) == 1
}

type Approvals struct {
	Value []Approval
}

func (b *Approvals) Pull(key string) error {
	return registry.Pull(defaultRegistry, key, &b.Value)
}

func (b Approvals) Push(key string) error {
	return registry.Push(defaultRegistry, key, b.Value)
}

func (b *Approvals) sort() {
	sort.Slice(b.Value, func(i, j int) bool {
		return b.Value[i].Ts < b.Value[j].Ts
	})
	return
}

func (b *Approvals) decline(hash Hash) {
	for n := range b.Value {
		if b.Value[n].Hash.Equal(hash, 0) {
			b.Value[n] = b.Value[len(b.Value)-1]
			b.Value = b.Value[:len(b.Value)-1]
			return
		}
	}
	return
}

var MaxApprovals = 100

func (b *Approvals) accept(patch Approval) {
	patch.Ts = getUnixTs()

	// updating
	for n, val := range b.Value {
		if val.Hash.Equal(patch.Hash, 0) {
			b.Value[n] = patch
			return
		}
	}

	// overflowed
	if len(b.Value) >= MaxApprovals {
		b.sort()
		b.Value[0] = patch
		return
	}

	// a new one
	b.Value = append(b.Value, patch)
}
