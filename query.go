package gosnap

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/ecwid/gosnap/registry"
)

var ErrBaselinePublished = errors.New("published now (baseline was not found)")

type Query struct {
	matcher Matcher
	key     string
	target  image.Image
	data    map[string]string
}

func (f Matcher) New(actual image.Image) Query {
	return Query{
		matcher: f,
		target:  actual,
		data:    f.data,
	}
}

func (q Query) Mask(rectangle image.Rectangle, color color.RGBA) Query {
	q.target = Masked{
		Image: q.target,
		mask:  rectangle,
		clr:   color,
	}
	return q
}

func (q Query) Snapshot(key string) Query {
	q.key = key
	return q
}

func (q Query) Metadata(key string, value any) Query {
	q.data[key] = fmt.Sprint(value)
	return q
}

func (q Query) baselineKey() string {
	return q.matcher.prependPathString() + "/" + q.key
}

func (q Query) Compare() error {
	if q.target == nil {
		return errors.New("no target (actual) image set")
	}
	if q.key == "" {
		return errors.New("baseline key is required")
	}
	if q.matcher.approvalEnabled && q.matcher.approvalKey == "" {
		return errors.New("approvalEnabled but approvalKey not defined")
	}
	targetHash := MakeHash(q.target, q.matcher.hashSize)

	var (
		err         error
		baselineKey = q.baselineKey()
		baseline    = new(Snapshot)
	)

	var updateBaseline = func() error {
		err = q.updateSnapshotKey(baselineKey, targetHash, q.target)
		if err != nil {
			return err
		}
		return ErrBaselinePublished
	}

	// force update baseline without matching and exit
	if q.matcher.forceUpdate {
		return updateBaseline()
	}

	if err = baseline.Head(baselineKey); err != nil {
		if errors.Is(err, registry.ErrNoSuchKey) {
			return updateBaseline()
		}
		return err
	}

	othernessHash, equal := Compare(baseline.Hash, targetHash, q.matcher.distance)
	if equal {
		return nil
	}
	// update baseline and exit
	if q.matcher.update {
		return updateBaseline()
	}
	// check if approved
	if q.matcher.approvalEnabled {
		approvals := Approvals{}
		err = approvals.Pull(q.matcher.approvalKey)
		if err != nil && !errors.Is(err, registry.ErrNoSuchKey) {
			return errors.Join(errors.New("can't pull approvals"), err)
		}
		if len(ApprovalsContains(approvals.Value, othernessHash, q.matcher.distance)) > 0 {
			return nil
		}
	}

	const uploadOtherness = true
	var (
		targetKey    string
		othernessKey string
	)
	if uploadOtherness {
		// upload target image
		targetKey, err = q.UploadSnapshot(targetHash, q.target)
		if err != nil {
			return err
		}

		// no hash matches so we need download the baseline image to make diff between them
		baseline.Pull(baselineKey)
		// upload otherness image
		other := difference(baseline.Value, q.target)
		othernessKey, err = q.UploadSnapshot(othernessHash, other)
		if err != nil {
			return err
		}
	}

	return Otherness{
		Key:          baselineKey,
		Hash:         othernessHash,
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

func (q Query) UploadSnapshot(hash Hash, image image.Image) (key string, err error) {
	key = q.matcher.generateKey()
	err = q.updateSnapshotKey(key, hash, image)
	return key, err
}

func (q Query) updateSnapshotKey(key string, hash Hash, image image.Image) (err error) {
	upload := Snapshot{
		Hash:     hash,
		Value:    image,
		Metadata: map[string]string{},
	}
	for k, v := range q.data {
		upload.Metadata[k] = v
	}
	if err = upload.Push(key); err != nil {
		err = errors.Join(errors.New("can't upload snapshot image"), err)
	}
	return err
}

func (q Query) CompareAndSaveForApproval() error {
	compareError := q.Compare()
	if err, ok := compareError.(Otherness); ok {
		syncError := q.matcher.sync.Sync(func() error {
			return addChanges(q.matcher.runID, err)
		})
		if syncError != nil {
			return errors.Join(compareError, errors.New("can't add changes for approval"), syncError)
		}
		err.changesKey = q.matcher.runID
		return err
	}
	return compareError
}
