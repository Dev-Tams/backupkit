package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dev-tams/backupkit/internal/storage/prunable"
)

type Storage struct {
	name string
	base string
}

func New(name, basePath string) *Storage {
	return &Storage{name: name, base: basePath}
}

func (s *Storage) Name() string { return s.name }

func (s *Storage) OpenWriter(_ context.Context, key string) (io.WriteCloser, string, error) {
	finalPath := filepath.Join(s.base, filepath.FromSlash(key))

	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return nil, "", fmt.Errorf("mkdir: %w", err)
	}

	tmpPath := finalPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, "", fmt.Errorf("create temp: %w", err)
	}

	return &Writer{f: f, tmpPath: tmpPath, finalPath: finalPath}, finalPath, nil
}

type Writer struct {
	f         *os.File
	tmpPath   string
	finalPath string
	closed    bool
}

func (w *Writer) Write(p []byte) (int, error) { return w.f.Write(p) }

func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.f.Close(); err != nil {
		_ = os.Remove(w.tmpPath)
		return err
	}
	if err := os.Rename(w.tmpPath, w.finalPath); err != nil {
		_ = os.Remove(w.tmpPath)
		return err
	}
	return nil
}

func (s *Storage) BasePath() string { return s.base }

func (s *Storage) List(_ context.Context, prefix string) ([]prunable.ObjectInfo, error) {
	dir := filepath.Join(s.base, filepath.FromSlash(prefix))

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list dir: %w", err)
	}

	out := make([]prunable.ObjectInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// Skip tmp files (shouldn't exist after successful backups, but safe)
		if filepath.Ext(e.Name()) == ".tmp" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat: %w", err)
		}

		out = append(out, prunable.ObjectInfo{
			Key:     filepath.ToSlash(filepath.Join(prefix, e.Name())),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	return out, nil
}

func (s *Storage) Delete(_ context.Context, key string) error {
	p := filepath.Join(s.base, filepath.FromSlash(key))
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}
