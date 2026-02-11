package app

import (
	"io"

	"github.com/dev-tams/backupkit/internal/compression"
	"github.com/dev-tams/backupkit/internal/encryption"
)

type closeStack []io.Closer

func (cs *closeStack) add(c io.Closer) {
	*cs = append(*cs, c)
}

func (cs closeStack) closeAll() {
	for i := len(cs) - 1; i >= 0; i-- {
		_ = cs[i].Close()
	}
}

func gzipReader(src io.Reader, closers *closeStack) io.Reader {
	pr, pw := io.Pipe()
	closers.add(pr)

	go func() {
		_, err := compression.Gzip(pw, src)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()

	}()
	return pr
}

func encryptReader(src io.Reader, password string, closers *closeStack) io.Reader {
	pr, pw := io.Pipe()
	closers.add(pr)

	go func() {
		_, err := encryption.EncryptAESGCM(pw, src, password)
		_ = pw.CloseWithError(err)
	}()
	return pr
}

func gunzipReader(src io.Reader, closers *closeStack) io.Reader {
	pr, pw := io.Pipe()
	closers.add(pr)

	go func() {
		_, err := compression.Gunzip(pw, src)
		_ = pw.CloseWithError(err)
	}()

	return pr
}

func decryptReader(src io.Reader, password string, closers *closeStack) io.Reader {
	pr, pw := io.Pipe()
	closers.add(pr)

	go func() {
		_, err := encryption.DecryptAESGCM(pw, src, password)
		_ = pw.CloseWithError(err)
	}()

	return pr
}
