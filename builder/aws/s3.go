package aws

import (
	"fmt"

	"github.com/seqsense/s3sync/v2"
)

type SilentLogger struct{}

func (s *SilentLogger) Logf(format string, v ...interface{}) {}

func (a *AWS) CopyFromS3(bucket, prefix, dest string) error {
	syncManager := s3sync.New(*a.config)
	return syncManager.Sync(a.context, fmt.Sprintf("s3://%s/%s", bucket, prefix), dest)
}

func (a *AWS) SyncToS3(src, bucket, prefix string, quiet bool) error {
	if quiet {
		s3sync.SetLogger(&SilentLogger{})
	}
	syncManager := s3sync.New(*a.config, s3sync.WithDelete())
	err := syncManager.Sync(a.context, src, fmt.Sprintf("s3://%s/%s", bucket, prefix))
	if quiet {
		s3sync.SetLogger(nil)
	}
	return err
}
