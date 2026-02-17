package prunable

import (
	"context"
	"time"
)

type ObjectInfo struct {
	Key     string
	Size    int64
	ModTime time.Time
}

type Prunable interface {
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	BasePath() string // optional helper for local storage, can be empty for other storage types
}
