package awshelpers

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

func DescribeStack(awsCfg aws.Config, stackName string) (*types.Stack, error) {
	cfSvc := cloudformation.NewFromConfig(awsCfg)
	result, err := cfSvc.DescribeStacks(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil {
		return nil, err
	}
	return &result.Stacks[0], nil
}

func DestroyStack(awsCfg aws.Config, stackName string) error {
	cfSvc := cloudformation.NewFromConfig(awsCfg)
	_, err := cfSvc.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
		StackName: &stackName,
	})
	return err
}
