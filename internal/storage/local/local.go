package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type Storage struct {
	name string
	base string
}

func New(name, basePath string) *Storage {
	return &Storage{name: name, base: basePath}
}

func (s *Storage) Name() string {
	return s.name
}

func (s *Storage) OpenWriter(_ context.Context, key string) (*Writer, error) {
	finalPath := filepath.Join(s.base, filepath.FromSlash(key))

	if err := os.Mkdir(filepath.Dir(finalPath), 0o755); err!= nil{
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	//this writes to a tmp file and rename on close
	tmpPath := finalPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil{
		return nil, fmt.Errorf("create temp: %w", err)
	}

	return &Writer{
		f: f,
		tmpPath: tmpPath,
		finalPath: finalPath,
	}, nil
}

type Writer struct{
	f *os.File
	tmpPath string
	finalPath string
	closed bool
}


func (w *Writer) Write(p []byte)(int, error){
	return w.f.Write(p)
}

func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if err := w.f.Close(); err != nil{
		_ = os.Remove(w.tmpPath)
		return err
	}
	return nil
}

func (w *Writer) Location() string{
	return w.finalPath
}