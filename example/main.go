package main

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"

	"github.com/ecwid/gosnap"
	"github.com/ecwid/gosnap/registry/s3"
)

func saveImg(img image.Image, name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func MustLoad(url string) image.Image {
	value, err := load(url)
	if err != nil {
		panic(err)
	}
	return value
}

func load(url string) (image.Image, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return png.Decode(r.Body)
}

func main() {
	gosnap.SetRegistry(s3.NewRegistry("", "", ""))
	target := MustLoad("https://s3.amazonaws.com/31d662d7-1dd4-45bf-8826-8b4c90d0d7ca")

	matcher := gosnap.
		NewMatcher("stable").
		ApprovalEnabled(true, "approvals_2023").
		SnapshotSource("2023", "chromium", "117")

	err := matcher.New(target).
		Snapshot("direct-free-control-panel/select-plan").
		Mask(image.Rect(0, 0, 100, 100), color.RGBA{}).
		Metadata("user", "me").
		Metadata("os", "darwin").
		CompareAndSaveForApproval()

	if err != nil {
		panic(err)
	}
}
