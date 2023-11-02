package gosnap

import (
	"errors"
	"gosnap/registry"
	"image"
	"sort"
	"time"
)

const (
	dataHash     = "Dhash"
	dataVersion  = "Version"
	dataMetadata = "Data"
)

type Snapshot struct {
	Last    time.Time
	Version string
	Hash    Hash
	Value   image.Image
}

func (s *Snapshot) Decode(obj registry.Object) error {
	*s = Snapshot{
		Last:    obj.Last,
		Hash:    hashString(obj.Data[dataHash]),
		Version: obj.Data[dataVersion],
	}
	var err error
	if obj.Body != nil {
		(*s).Value, err = decodePng(obj.Body)
		if err != nil {
			return errors.Join(errors.New("can't decode snapshot png"), err)
		}
	}
	return err
}

func (b Snapshot) Encode() (*registry.Object, error) {
	var (
		body []byte
		err  error
	)
	if b.Value != nil {
		body, err = encodePng(b.Value)
		if err != nil {
			return nil, errors.Join(errors.New("can't encode snapshot png"), err)
		}
	}
	return &registry.Object{
		Body: body,
		Data: map[string]string{
			dataVersion: b.Version,
			dataHash:    b.Hash.String(),
		},
	}, nil
}

func (s *Snapshot) Pull(key string, body bool) error {
	obj, err := defaultRegistry.Pull(key, body)
	if err != nil {
		return errors.Join(errors.New("can't pull snapshot"), err)
	}
	return s.Decode(*obj)
}

func (s Snapshot) Push(key string) error {
	obj, err := s.Encode()
	if err != nil {
		return err
	}
	if err = defaultRegistry.Push(key, *obj); err != nil {
		return errors.Join(errors.New("can't push snapshot"), err)
	}
	return nil
}

var baselineMaxApprovals = 20

type Baseline struct {
	Snapshot
	Approvals []Approval
}

func (b *Baseline) Decode(obj registry.Object) error {
	err := b.Snapshot.Decode(obj)
	if err != nil {
		return err
	}
	if value := obj.Data[dataMetadata]; value != "" {
		err = base64Unpack(value, &b.Approvals)
	}
	return err
}

func (b Baseline) Encode() (*registry.Object, error) {
	obj, err := b.Snapshot.Encode()
	if err != nil {
		return nil, err
	}
	if len(b.Approvals) > 0 {
		data, err := base64Pack(b.Approvals)
		if err != nil {
			return nil, errors.Join(errors.New("can't gzip data"), err)
		}
		obj.Data[dataMetadata] = data
	}
	return obj, nil
}

func (b *Baseline) Pull(key string, body bool) error {
	obj, err := defaultRegistry.Pull(key, body)
	if err != nil {
		return errors.Join(errors.New("can't pull baseline"), err)
	}
	return b.Decode(*obj)
}

func (b Baseline) Push(key string) error {
	obj, err := b.Encode()
	if err != nil {
		return err
	}
	if err = defaultRegistry.Push(key, *obj); err != nil {
		return errors.Join(errors.New("can't push baseline"), err)
	}
	return nil
}

func (b *Baseline) sort() {
	sort.Slice(b.Approvals, func(i, j int) bool {
		return b.Approvals[i].Ts < b.Approvals[j].Ts
	})
	return
}

func (b *Baseline) decline(hash Hash) {
	for n := range b.Approvals {
		if b.Approvals[n].Hash.Equal(hash, 0) {
			b.Approvals[n] = b.Approvals[len(b.Approvals)-1]
			b.Approvals = b.Approvals[:len(b.Approvals)-1]
			return
		}
	}
	return
}

func (b *Baseline) accept(patch Approval) {
	patch.Ts = getUnixTs()

	// updating
	for n, val := range b.Approvals {
		if val.Hash.Equal(patch.Hash, 0) {
			b.Approvals[n] = patch
			return
		}
	}

	// overflowed
	if len(b.Approvals) >= baselineMaxApprovals {
		b.sort()
		b.Approvals[0] = patch
		return
	}

	// a new one
	b.Approvals = append(b.Approvals, patch)
}

type Approval struct {
	Ts       int64  `json:"ts"`
	Hash     Hash   `json:"hash"`
	Approver string `json:"approver"`
}

func (t Approval) Valid() bool {
	return time.Unix(t.Ts, 0).AddDate(0, 2, 0).Compare(time.Now()) == 1
}
