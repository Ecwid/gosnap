package gosnap

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/ecwid/gosnap/registry"
	"github.com/google/uuid"
)

var (
	defaultRegistry registry.Abstract
)

func SetRegistry(r registry.Abstract) {
	defaultRegistry = r
}

func getUnixTs() int64 {
	return time.Now().Unix()
}

type Matcher struct {
	runID           string
	approvalEnabled bool
	update          bool
	forceUpdate     bool
	normalize       bool
	approvalKey     string
	distance        int
	hashSize        uint
	data            map[string]string
	sync            Synced
	path            []string
}

func NewMatcher(runID string) Matcher {
	return Matcher{
		runID:           runID,
		approvalKey:     "",
		approvalEnabled: true,
		update:          false,
		forceUpdate:     false,
		normalize:       false,
		distance:        6,
		hashSize:        1024,
		sync:            NewSyncedOps(),
		data:            map[string]string{},
	}
}

func (m Matcher) NormalizeSize(enable bool) Matcher {
	m.normalize = enable
	return m
}

func (m Matcher) Update(enable bool) Matcher {
	m.update = enable
	return m
}

func (m Matcher) ForceUpdate(enable bool) Matcher {
	m.forceUpdate = enable
	return m
}

func (m Matcher) ApprovalEnabled(enable bool, key string) Matcher {
	m.approvalEnabled = enable
	m.approvalKey = key
	return m
}

func (m Matcher) Metadata(key string, value any) Matcher {
	m.data[key] = fmt.Sprint(value)
	return m
}

func (m Matcher) SnapshotSource(args ...string) Matcher {
	m.path = append(m.path, args...)
	return m
}

func (m Matcher) prependPathString() string {
	if len(m.path) > 0 {
		return strings.Join(m.path, "/")
	}
	return ""
}

func (m Matcher) generateKey() string {
	return m.prependPathString() + uuid.NewString()
}

func upload(image image.Image) (string, error) {
	key := uuid.NewString()
	return key, Snapshot{Value: image}.Push(key)
}

func MustUpload(image image.Image) string {
	k, err := upload(image)
	if err != nil {
		panic(err)
	}
	return defaultRegistry.Resolve(k)
}

func DefaultCompare(expected, actual image.Image) error {
	const (
		hashSize = 1024
		distance = 6
	)
	var (
		hash1 = MakeHash(expected, hashSize)
		hash2 = MakeHash(actual, hashSize)
	)
	otherHash, equal := hash1.equal(hash2, distance)
	if equal {
		return nil
	}
	baseline, err := upload(expected)
	if err != nil {
		return err
	}
	target, err := upload(actual)
	if err != nil {
		return err
	}
	overlay, err := upload(difference(expected, actual))
	if err != nil {
		return err
	}
	return Change{
		Key:     baseline,
		Hash:    otherHash,
		Target:  target,
		Overlay: overlay,
	}
}

type Synced struct {
	value *sync.Mutex
}

func (s Synced) Sync(cb func() error) error {
	s.value.Lock()
	defer s.value.Unlock()
	return cb()
}

func NewSyncedOps() Synced {
	return Synced{
		value: &sync.Mutex{},
	}
}

func (s Synced) Accept(key string, hash Hash, approver string) error {
	return s.Sync(func() error {
		var approvals = new(Approvals)
		err := approvals.Pull(key)
		if err != nil && !errors.Is(err, registry.ErrNoSuchKey) {
			return err
		}
		approvals.accept(Approval{Hash: hash, Approver: approver})
		return approvals.Push(key)
	})
}

func (s Synced) Decline(key string, hash Hash) error {
	return s.Sync(func() error {
		var approvals = new(Approvals)
		if err := approvals.Pull(key); err != nil {
			return err
		}
		approvals.decline(hash)
		return approvals.Push(key)
	})
}

func (s Synced) CopySnapshot(src, dest, author string) error {
	return s.Sync(func() error {
		var snapshot = new(Snapshot)
		if err := snapshot.Pull(src); err != nil {
			return err
		}
		snapshot.Metadata["author"] = author
		return snapshot.Push(dest)
	})
}

func (s Synced) DeleteChanges(key string, change *string) error {
	return s.Sync(func() error {
		return deleteChanges(key, change)
	})
}
