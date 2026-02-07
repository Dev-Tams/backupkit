package storage

import (
	"context"
	"io"
)


type Writer interface {
	io.Writer
	//location returns the identifier (path, s3://,,,, etc)
	Location() string
}

type Storage interface {
	Name() string

	//key is a storage rel path. each backend decides what key means
	OpenWriter(ctx context.Context, key string)(Writer, error)
}