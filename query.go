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

type mask struct {
	Rect  image.Rectangle
	Color color.Color
}

type Query struct {
	matcher Matcher
	key     string
	data    map[string]string
	masks   []mask
}

func (f Matcher) New(snapshot string) Query {
	return Query{
		key:     snapshot,
		matcher: f,
		data:    f.data,
	}
}

func (q Query) Mask(rectangle image.Rectangle, color color.Color) Query {
	q.masks = append(q.masks, mask{Rect: rectangle, Color: color})
	return q
}

func (q Query) Metadata(key string, value any) Query {
	q.data[key] = fmt.Sprint(value)
	return q
}

func (q Query) baselineKey() string {
	return q.matcher.prependPathString() + q.key
}

func (q Query) makeTargetHash(target image.Image, x, y int) Hash {
	if q.matcher.normalize {
		croppedTarget := target.(subImage).SubImage(image.Rect(0, 0, x, y))
		return MakeHash(croppedTarget, q.matcher.hashSize)
	}
	return MakeHash(target, q.matcher.hashSize)
}

func (q Query) Match(target image.Image) error {
	if target == nil {
		return errors.New("no target (actual) image set")
	}
	if q.key == "" {
		return errors.New("baseline key is required")
	}
	if q.matcher.approvalEnabled && q.matcher.approvalKey == "" {
		return errors.New("approvalEnabled but approvalKey not defined")
	}
	for _, mask := range q.masks {
		target = Masked{
			Image: target,
			Rect:  mask.Rect,
			Color: mask.Color,
		}
	}

	var (
		err         error
		baselineKey = q.baselineKey()
		baseline    = new(Snapshot)
	)

	// get baseline hash
	err = baseline.Head(baselineKey)

	// force update baseline without matching and exit
	if errors.Is(err, registry.ErrNoSuchKey) || q.matcher.forceUpdate {
		hash := MakeHash(target, q.matcher.hashSize)
		return q.uploadBaseline(baselineKey, hash, target)
	}
	if err != nil {
		return err
	}

	// Comparing the baseline with target
	x, y := baseline.GetSize()
	targetHash := q.makeTargetHash(target, x, y)

	xorHash, equal := baseline.Hash.equal(targetHash, q.matcher.distance)
	if equal {
		return nil
	}
	// update baseline and exit
	if q.matcher.update {
		return q.uploadBaseline(baselineKey, targetHash, target)
	}
	// check if approved
	if q.matcher.approvalEnabled {
		approvals := Approvals{}
		err = approvals.Pull(q.matcher.approvalKey)
		if err != nil && !errors.Is(err, registry.ErrNoSuchKey) {
			return errors.Join(errors.New("can't pull approvals"), err)
		}
		if len(ApprovalsContains(approvals.Value, xorHash, q.matcher.distance)) > 0 {
			return nil
		}
	}

	return Change{
		Key:        baselineKey,
		XorHash:    xorHash,
		TargetHash: targetHash,
		Data:       q.data,
		target:     target,
	}
}

func (q Query) UploadChange(change Change) error {
	var err error

	// upload target image
	change.Target, err = q.uploadSnapshot(change.TargetHash, change.target)
	if err != nil {
		return errors.Join(err, change)
	}

	// no hash matches so we need download the baseline image to make diff between them
	baseline := new(Snapshot)
	err = baseline.Pull(change.Key)
	if err != nil {
		return errors.Join(err, change)
	}

	// upload diff overlay image
	change.Overlay, err = q.uploadSnapshot(change.XorHash, overlay(baseline.Value, change.target))
	if err != nil {
		return errors.Join(err, change)
	}
	return change
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

func (q Query) uploadSnapshot(hash Hash, image image.Image) (key string, err error) {
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

// Compare match with baseline and upload target, overlay and approval report
func (q Query) Compare(actual image.Image) error {
	compareError := q.Match(actual)
	if e, ok := compareError.(Change); ok {
		return q.matcher.SaveChangeForApproval(q.UploadChange(e))
	}
	return compareError
}
