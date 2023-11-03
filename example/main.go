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

	gosnap.SetRegistry(s3.NewRegistry("id", "secret", "my-bucket"))

	// baseline := MustLoad("https://ecwid-screenshots.s3.amazonaws.com/2023/1/chromium/112/direct-free-control-panel/select-plan")
	target := MustLoad("https://s3.amazonaws.com/2023")

	matcher := gosnap.
		NewMatcher("stable").
		ApprovalSource("approvals2023").
		PrependSnapshotPath("2023", "1", "chromium", "112")

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
