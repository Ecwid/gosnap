package gosnap

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ecwid/gosnap/registry"
)

var KeyToApproveUrl = func(key string) string {
	return defaultRegistry.Resolve(key)
}

type Otherness struct {
	Ts           int64          `json:"ts"`
	Version      string         `json:"version"`
	Hash         Hash           `json:"hash"`
	TargetHash   Hash           `json:"target_hash"`
	Data         map[string]any `json:"data"` // user data
	Key          string         `json:"key"`  // baseline key
	TargetKey    string         `json:"target"`
	OthernessKey string         `json:"otherness"`
	changesKey   string         `json:"-"`
}

func (e Otherness) Error() string {
	return fmt.Sprintf(`
	the page changed (score %d)
	expected:   %s
	actual:     %s
	difference: %s
	please approve: %s
	`,
		e.Hash.onesCount(),
		defaultRegistry.Resolve(e.Key),
		defaultRegistry.Resolve(e.TargetKey),
		defaultRegistry.Resolve(e.OthernessKey),
		KeyToApproveUrl(e.changesKey),
	)
}

type Batch struct {
	Changes []Otherness
}

func (batch *Batch) Decode(obj registry.Object) error {
	if obj.Body != nil {
		err := json.Unmarshal(obj.Body, &batch.Changes)
		if err != nil {
			return errors.Join(errors.New("can't unmarshal snapshot"), err)
		}
	}
	return nil
}

func (batch Batch) Encode() (*registry.Object, error) {
	var obj = new(registry.Object)
	var err error
	obj.Body, err = json.Marshal(batch.Changes)
	if err != nil {
		return nil, errors.Join(errors.New("can't marshal to snapshot"), err)
	}
	return obj, nil
}

func (b *Batch) Pull(key string) error {
	obj, err := defaultRegistry.Pull(key, true)
	if err != nil {
		return err
	}
	return b.Decode(*obj)
}

func (b Batch) Push(key string) error {
	obj, err := b.Encode()
	if err != nil {
		return err
	}
	return defaultRegistry.Push(key, *obj)
}

func (batch Batch) findIndex(key string) int {
	for n, value := range batch.Changes {
		if value.Key == key {
			return n
		}
	}
	return -1
}

func addChanges(key string, target Otherness) error {
	var batch = new(Batch)
	err := batch.Pull(key)
	if err != nil {
		return err
	}

	target.Ts = getUnixTs()

	// update
	updated := false
	for n := range batch.Changes {
		if batch.Changes[n].Key == target.Key {
			batch.Changes[n] = target
			updated = true
			break
		}
	}

	// a new one
	if !updated {
		batch.Changes = append(batch.Changes, target)
	}

	return batch.Push(key)
}

func deleteChanges(key string, changeKey *string) error {
	var batch = new(Batch)
	err := batch.Pull(key)
	if err != nil {
		return err
	}

	if changeKey == nil { // delete all
		batch.Changes = []Otherness{}
	} else if n := batch.findIndex(*changeKey); n >= 0 { // delete by address
		batch.Changes[n] = batch.Changes[len(batch.Changes)-1]
		batch.Changes = batch.Changes[:len(batch.Changes)-1]
	} else { // nothing to do
		return nil
	}

	return batch.Push(key)
}
