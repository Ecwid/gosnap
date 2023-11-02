package registry

import (
	"errors"
	"time"
)

var ErrNoSuchKey = errors.New("no such key")

type Object struct {
	Last time.Time
	Body []byte
	Data map[string]string
}

type Abstract interface {
	Pull(key string, downloadBody bool) (*Object, error)
	Push(key string, value Object) error
	Resolve(key string) string
}
