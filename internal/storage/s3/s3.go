package s3store

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type Storage struct {
	name   string
	bucket string
	prefix string
	client *s3.Client
	region string
}

type Options struct {
	Name      string
	Bucket    string
	Region    string
	Prefix    string
	AccessKey string
	SecretKey string
}

func New(ctx context.Context, opt Options) (*Storage, error) {
	if opt.Bucket == "" || opt.Region == "" {
		return nil, fmt.Errorf("s3: bucket and region are required")
	}

	creds := credentials.NewStaticCredentialsProvider(opt.AccessKey, opt.SecretKey, "")

	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(opt.Region),
		awsconfig.WithCredentialsProvider(creds),
	)

	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return &Storage{
		name:   opt.Name,
		bucket: opt.Bucket,
		region: opt.Region,
		prefix: strings.Trim(opt.Prefix, "/"),
		client: s3.NewFromConfig(cfg),
	}, nil
}

func (s *Storage) Name() string {
	return s.name
}

func (s *Storage) OpenWriter(ctx context.Context, key string) (Writer, error) {

	//turns the streaming pipeline into an s3 req body
	pr, pw := io.Pipe()

	fullKey := key
	if s.prefix != "" {

		// S3 usually use forward slashes
		fullKey = path.Join(s.prefix, key)
	}

	w := &uploadWriter{
		pw:      pw,
		loc:     fmt.Sprintf("s3://%s/%s", s.bucket, fullKey),
		done:    make(chan error, 1),
		closeCh: make(chan struct{}),
	}

	//start upload in gorountine

	// PutObject reads from pr while your app writes to pw.
	go func() {
		defer close(w.closeCh)
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(fullKey),
			Body:   pr,
		})

		//checks the reader is closed
		_ = pr.CloseWithError(err)

		if err != nil {
			if apiErr, ok := err.(smithy.APIError); ok {
				w.done <- fmt.Errorf(" s3 putobjet failed: %s: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
				return
			}
			w.done <- fmt.Errorf("s3 putobjet failed: %w", err)
			return
		}
		w.done <- nil
	}()

	return w, nil
}

type Writer interface {
	io.WriteCloser
	Location() string
}

type uploadWriter struct {
	pw      *io.PipeWriter
	loc     string
	done    chan error
	closeCh chan struct{}
	closed  bool
}

func (w *uploadWriter) Write(p []byte) (int, error) {
	return w.pw.Write(p)
}

//Close waits for the upload to finish, then the CLI report success/Failure
func (w *uploadWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	// Closing writer signals EOF to S3 upload
	_ = w.pw.Close()

	// Wait for upload to finish (success or failure)
	return <-w.done
}

func (w *uploadWriter) Location() string { return w.loc }
