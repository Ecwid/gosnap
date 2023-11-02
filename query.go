package gosnap

import (
	"errors"
	"gosnap/registry"
	"image"
)

var ErrBaselinePublished = errors.New("published now (baseline was not found)")

type Query struct {
	matcher Matcher
	key     string
	target  image.Image
	data    map[string]any
}

func (f Matcher) New(actual image.Image) Query {
	return Query{
		matcher: f,
		data:    map[string]any{},
		target:  actual,
	}
}

func (q Query) WithBaseline(key string) Query {
	q.key = key
	return q
}

func (q Query) WithUserData(key string, value any) Query {
	q.data[key] = value
	return q
}

func (q Query) baselineKey() string {
	return q.matcher.prefixString() + "/" + q.key
}

func (q Query) Compare() error {
	if q.target == nil {
		return errors.New("no target image set")
	}
	if q.matcher.version == "" {
		return errors.New("target's version is required")
	}
	if q.key == "" {
		return errors.New("baseline address is required")
	}
	targetHash := MakeHash(q.target, q.matcher.hashSize)

	var err error
	var baselineKey = q.baselineKey()
	var baseline = new(Baseline)

	if err = baseline.Pull(baselineKey, false); err != nil {
		if errors.Is(err, registry.ErrNoSuchKey) {
			upload := Snapshot{
				Version: q.matcher.version,
				Hash:    targetHash,
				Value:   q.target,
			}
			if err = upload.Push(baselineKey); err != nil {
				return err
			}
			return ErrBaselinePublished
		}
		return err
	}

	othernessHash, equal := Compare(baseline.Snapshot.Hash, targetHash, q.matcher.distance)
	if equal {
		return nil
	}
	if q.matcher.approvalEnabled {
		switch len(ApprovalsContains(baseline.Approvals, othernessHash, q.matcher.distance)) {
		case 1, 2: // one or two hash equals
			return nil
		}
	}

	// no hash matches so we need download image to make diff
	baseline.Snapshot.Pull(baselineKey, true)

	// upload target image
	targetKey, err := q.matcher.UploadSnapshot(targetHash, q.target)
	if err != nil {
		return err
	}

	// upload otherness image
	other := difference(baseline.Snapshot.Value, q.target)
	othernessKey, err := q.matcher.UploadSnapshot(othernessHash, other)
	if err != nil {
		return err
	}

	return Otherness{
		Version:      q.matcher.version,
		Key:          baselineKey,
		Hash:         othernessHash,
		TargetHash:   targetHash,
		Data:         q.data,
		TargetKey:    targetKey,
		OthernessKey: othernessKey,
	}
}

func Compare(baseline Hash, target Hash, distance int) (hash Hash, equal bool) {
	hash = baseline.Xor(target) // difference hash
	if hash.onesLess(distance) {
		return hash, true
	}
	return hash, false
}

func ApprovalsContains(approvals []Approval, hash Hash, distance int) []Approval {
	for _, tar := range approvals {
		if hash.Equal(tar.Hash, distance) {
			return []Approval{tar}
		}
	}
	/**/
	for _, tar1 := range approvals {
		for _, tar2 := range approvals {
			if hash.Equal(tar1.Hash.Or(tar2.Hash), distance) {
				return []Approval{tar1, tar2}
			}
		}
	}
	return []Approval{}
}

func (q Query) CompareAndSaveForApproval() error {
	uerr := q.Compare()
	if err, ok := uerr.(Otherness); ok {
		changesKey := q.matcher.tag.String()
		err1 := q.matcher.sync.Sync(func() error {
			return addChanges(changesKey, err)
		})
		if err1 != nil {
			return errors.Join(err1, err)
		}
		err.changesKey = changesKey
		return err
	}
	return uerr
}
