package aws

import (
	"context"
	"fmt"

	"github.com/seqsense/s3sync/v2"
)

type SilentLogger struct{}

func (s *SilentLogger) Logf(format string, v ...interface{}) {}

func (a *AWS) CopyFromS3(ctx context.Context, bucket, prefix, dest string) error {
	syncManager := s3sync.New(*a.config)
	return syncManager.Sync(ctx, fmt.Sprintf("s3://%s/%s", bucket, prefix), dest)
}

func (a *AWS) SyncToS3(ctx context.Context, src, bucket, prefix string, quiet bool) error {
	if quiet {
		s3sync.SetLogger(&SilentLogger{})
	}
	syncManager := s3sync.New(*a.config, s3sync.WithDelete())
	err := syncManager.Sync(ctx, src, fmt.Sprintf("s3://%s/%s", bucket, prefix))
	if quiet {
		s3sync.SetLogger(nil)
	}
	return err
}
