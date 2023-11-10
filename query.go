package gosnap

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/ecwid/gosnap/registry"
)

type Published struct {
	Key string
}

func (p Published) Error() string {
	return fmt.Sprint("snapshot published: ", defaultRegistry.Resolve(p.Key))
}

type subImage interface {
	SubImage(r image.Rectangle) image.Image
}

type Query struct {
	matcher  Matcher
	key      string
	baseline image.Image
	target   image.Image
	data     map[string]string
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

func (q Query) SnapshotFromImage(baseline image.Image) Query {
	q.baseline = baseline
	return q
}

func (q Query) Metadata(key string, value any) Query {
	q.data[key] = fmt.Sprint(value)
	return q
}

func (q Query) baselineKey() string {
	return q.matcher.prependPathString() + "/" + q.key
}

func (q Query) makeTargetHash(baseline *Snapshot) Hash {
	if q.matcher.normalize {
		x, y := baseline.GetSize()
		croppedTarget := q.target.(subImage).SubImage(image.Rect(0, 0, x, y))
		return MakeHash(croppedTarget, q.matcher.hashSize)
	}
	return MakeHash(q.target, q.matcher.hashSize)
}

func (q Query) Compare() error {
	if q.target == nil {
		return errors.New("no target (actual) image set")
	}
	if q.key == "" && q.baseline == nil {
		return errors.New("expected baseline key (or image) is required")
	}
	if q.matcher.approvalEnabled && q.matcher.approvalKey == "" {
		return errors.New("approvalEnabled but approvalKey not defined")
	}

	var (
		err         error
		baselineKey = ""
		baseline    = new(Snapshot)
	)

	if q.baseline == nil {
		baselineKey = q.baselineKey()
		// get baseline hash
		err = baseline.Head(baselineKey)

		// force update baseline without matching and exit
		if errors.Is(err, registry.ErrNoSuchKey) || q.matcher.forceUpdate {
			hash := MakeHash(q.target, q.matcher.hashSize)
			return q.uploadBaseline(baselineKey, hash, q.target)
		}
		if err != nil {
			return err
		}
	} else {
		// comparing two images
		baseline.Value = q.baseline
		baseline.Hash = MakeHash(q.baseline, q.matcher.hashSize)
	}

	// Comparing the baseline with target
	targetHash := q.makeTargetHash(baseline)

	othernessHash, equal := baseline.Hash.equal(targetHash, q.matcher.distance)
	if equal {
		return nil
	}
	// update baseline and exit
	if q.matcher.update && baselineKey != "" {
		return q.uploadBaseline(baselineKey, targetHash, q.target)
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
		if baselineKey != "" {
			baseline.Pull(baselineKey)
		}
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
	err = q.pushSnapshot(key, hash, image)
	return key, err
}

func (q Query) uploadBaseline(key string, newHash Hash, newImage image.Image) error {
	if key == "" {
		return errors.New("can't update baseline snapshot due key is empty")
	}
	err := q.pushSnapshot(key, newHash, newImage)
	if err != nil {
		return err
	}
	return Published{Key: key}
}

func (q Query) pushSnapshot(key string, hash Hash, image image.Image) (err error) {
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
