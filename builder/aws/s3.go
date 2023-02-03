package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/seqsense/s3sync"
)

func (a *AWS) CopyFromS3(bucket, prefix, dest string) error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return err
	}
	syncManager := s3sync.New(sess)
	return syncManager.Sync(fmt.Sprintf("s3://%s/%s", bucket, prefix), dest)
}

func (a *AWS) SyncToS3(src, bucket, prefix string) error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return err
	}
	syncManager := s3sync.New(sess, s3sync.WithDelete())
	return syncManager.Sync(src, fmt.Sprintf("s3://%s/%s", bucket, prefix))
}
