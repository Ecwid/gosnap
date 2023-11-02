package gosnap

import (
	"errors"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/ecwid/gosnap/registry"
	"github.com/google/uuid"
)

var (
	defaultRegistry registry.Abstract
	TagPrefix       = "manual-approvals"
)

func SetRegistry(r registry.Abstract) {
	defaultRegistry = r
}

func getUnixTs() int64 {
	return time.Now().Unix()
}

type Tag struct {
	Name   string
	Prefix string
}

func (t Tag) String() string {
	return t.Prefix + "/" + t.Name
}

type Matcher struct {
	tag             Tag
	version         string
	approvalEnabled bool
	distance        int
	hashSize        uint
	sync            Synced
	prefix          []string
}

func NewMatcher(snapshotVersion string) Matcher {
	return Matcher{
		version:         snapshotVersion,
		approvalEnabled: true,
		distance:        6,
		hashSize:        1024,
		sync:            NewSyncedOps(),
		tag: Tag{
			Prefix: TagPrefix,
			Name:   uuid.NewString(),
		},
	}
}

func (m Matcher) GroupByTag(tag string) Matcher {
	m.tag.Name = tag
	return m
}

func (m Matcher) WithPrefix(p ...string) Matcher {
	m.prefix = append(m.prefix, p...)
	return m
}

func (m Matcher) prefixString() string {
	return strings.Join(m.prefix, "/")
}

func (m Matcher) generateKey() string {
	return m.prefixString() + "/" + uuid.NewString()
}

func (m Matcher) UploadSnapshot(hash Hash, image image.Image) (key string, err error) {
	upload := Snapshot{
		Version: m.version,
		Hash:    hash,
		Value:   image,
	}
	key = m.generateKey()
	if err = upload.Push(key); err != nil {
		err = errors.Join(errors.New("can't upload snapshot image"), err)
	}
	return key, err
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

func (s Synced) Accept(other Otherness, approver string) error {
	return s.Sync(func() error {
		var baseline = new(Baseline)
		if err := baseline.Pull(other.Key, false); err != nil {
			return err
		}
		baseline.accept(Approval{Hash: other.Hash, Approver: approver})
		return baseline.Push(other.Key)
	})
}

func (s Synced) Decline(other Otherness) error {
	return s.Sync(func() error {
		var baseline = new(Baseline)
		if err := baseline.Pull(other.Key, false); err != nil {
			return err
		}
		baseline.decline(other.Hash)
		return baseline.Push(other.Key)
	})
}

func (s Synced) CopySnapshot(src, dest string) error {
	return s.Sync(func() error {
		var err error
		var baseline = new(Baseline)
		// don't load snapshot body
		if err = baseline.Pull(dest, false); err != nil {
			return err
		}
		if err = baseline.Snapshot.Pull(src, true); err != nil {
			return err
		}
		return baseline.Push(dest)
	})
}

func (s Synced) DeleteChanges(key string, change *string) error {
	return s.Sync(func() error {
		return deleteChanges(key, change)
	})
}
