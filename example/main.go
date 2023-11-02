package main

import (
	"gosnap"
	"gosnap/registry/s3"
	"image"
	"image/png"
	"net/http"
	"os"
)

func saveImg(img image.Image, name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
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

	baseline, err := load("https://s3.amazonaws.com/example")
	if err != nil {
		panic(err)
	}

	matcher := gosnap.NewMatcher("g10b1184cf1abd2").
		GroupByTag("stable").
		WithPrefix("2023", "1", "chromium", "112")

	err = matcher.New(baseline).
		WithBaseline("direct-my-ecwid-com/my-ecwid-com-register").
		WithUserData("user", "me").
		WithUserData("os", "darwin").
		CompareAndSaveForApproval()

	if err != nil {
		panic(err)
	}
}
