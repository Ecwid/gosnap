package gosnap

import (
	"errors"
	"fmt"

	"github.com/ecwid/gosnap/registry"
)

var KeyToApproveUrl = func(key string) string {
	return defaultRegistry.Resolve(key)
}

type Otherness struct {
	Ts           int64             `json:"ts"`
	Hash         Hash              `json:"hash"`
	Data         map[string]string `json:"data"` // user data
	Key          string            `json:"key"`  // baseline key
	TargetKey    string            `json:"target"`
	OthernessKey string            `json:"otherness"`
	changesKey   string            `json:"-"`
}

func (e Otherness) GetApproveUrl() string {
	return KeyToApproveUrl(e.changesKey)
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
		e.GetApproveUrl(),
	)
}

type Batch struct {
	Changes []Otherness
}

func (b *Batch) Pull(key string) error {
	return registry.Pull(defaultRegistry, key, &b.Changes)
}

func (b Batch) Push(key string) error {
	return registry.Push(defaultRegistry, key, b.Changes)
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
	if err != nil && !errors.Is(err, registry.ErrNoSuchKey) {
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
