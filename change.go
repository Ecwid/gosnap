package gosnap

import (
	"errors"
	"fmt"

	"github.com/ecwid/gosnap/registry"
)

var KeyToApproveUrl = func(key string) string {
	return defaultRegistry.Resolve(key)
}

type Change struct {
	Ts      int64             `json:"ts"`
	Key     string            `json:"key"`
	Hash    Hash              `json:"hash"`
	Data    map[string]string `json:"data"`
	Target  string            `json:"target"`
	Overlay string            `json:"overlay"`

	approveLabel string `json:"-"`
}

func (e Change) GetApproveUrl() string {
	return KeyToApproveUrl(e.approveLabel)
}

func (e Change) Error() string {
	s := fmt.Sprintf(`
	the page changed (score %d)
	expected:   %s
	actual:     %s
	overlay:    %s
	`,
		e.Hash.onesCount(),
		defaultRegistry.Resolve(e.Key),
		defaultRegistry.Resolve(e.Target),
		defaultRegistry.Resolve(e.Overlay),
	)
	if e.approveLabel != "" {
		s += fmt.Sprintf("please approve: %s\n", e.GetApproveUrl())
	}
	return s
}

type Batch struct {
	Changes []Change
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

func addChanges(key string, target Change) error {

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
		batch.Changes = []Change{}
	} else if n := batch.findIndex(*changeKey); n >= 0 { // delete by address
		batch.Changes[n] = batch.Changes[len(batch.Changes)-1]
		batch.Changes = batch.Changes[:len(batch.Changes)-1]
	} else { // nothing to do
		return nil
	}

	return batch.Push(key)
}
