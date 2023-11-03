package registry

import (
	"encoding/json"
	"errors"
)

var ErrNoSuchKey = errors.New("no such key")

type Object struct {
	Body []byte
	Data map[string]string
}

type Abstract interface {
	Head(key string) (map[string]string, error)
	Pull(key string) (*Object, error)
	Push(key string, value Object) error
	Resolve(key string) string
}

// Pull json object
func Pull(registry Abstract, key string, data any) error {
	src, err := registry.Pull(key)
	if err != nil {
		return err
	}
	if src.Body != nil {
		err := json.Unmarshal(src.Body, data)
		if err != nil {
			return err
		}
	}
	return nil
}

// Push json object
func Push(registry Abstract, key string, data any) error {
	var dest = new(Object)
	var err error
	dest.Body, err = json.Marshal(data)
	if err != nil {
		return err
	}
	return registry.Push(key, *dest)
}
