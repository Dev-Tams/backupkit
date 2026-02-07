package storage

import (
	"context"
	"io"
)

type Storage interface {
	Name() string

	//key is a storage rel path. each backend decides what key means
	OpenWriter(ctx context.Context, key string) (io.WriteCloser, string, error)
}
