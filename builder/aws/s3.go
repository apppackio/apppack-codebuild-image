package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/seqsense/s3sync"
)

type SilentLogger struct{}

func (s *SilentLogger) Log(v ...interface{})                 {}
func (s *SilentLogger) Logf(format string, v ...interface{}) {}

func (a *AWS) CopyFromS3(bucket, prefix, dest string) error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return err
	}
	syncManager := s3sync.New(sess)
	return syncManager.Sync(fmt.Sprintf("s3://%s/%s", bucket, prefix), dest)
}

func (a *AWS) SyncToS3(src, bucket, prefix string, quiet bool) error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return err
	}
	var syncManager *s3sync.Manager
	if quiet {
		s3sync.SetLogger(&SilentLogger{})
	}
	syncManager = s3sync.New(sess, s3sync.WithDelete())
	err = syncManager.Sync(src, fmt.Sprintf("s3://%s/%s", bucket, prefix))
	if quiet {
		s3sync.SetLogger(nil)
	}
	return err
}
