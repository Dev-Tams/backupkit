package compression

import (
	"compress/gzip"
	"fmt"
	"io"
)

func Gunzip(dst io.Writer, src io.Reader) (int64, error) {
	gr, err := gzip.NewReader(src)
	if err != nil {
		return 0, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	n, err := io.Copy(dst, gr)
	if err != nil {
		return n, fmt.Errorf("gunzip copy: %w", err)
	}
	return n, nil
}
