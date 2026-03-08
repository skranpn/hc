package hc

import (
	"errors"
	"os"

	"github.com/hashicorp/go-envparse"
)

func LoadEnvFile(path string) (m map[string]string, err error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	return envparse.Parse(f)
}
