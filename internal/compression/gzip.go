package compression

import (
	"compress/gzip"
	"io"
)

// Create internal/compression/gzip.go.

func Gzip (dst io.Writer, scr io.Reader) (int64, error) {
	
	//wraps dst
	gz := gzip.NewWriter(dst)

	//copy streams
	n, err := io.Copy(gz, scr)
	if err != nil{
		_ = gz.Close()
		return n, err
	}

	// gzip writes data on Close.
	if err := gz.Close(); err != nil {
		return n, err
	}

	return n, nil
}