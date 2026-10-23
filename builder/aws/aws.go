package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfnTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type AWSInterface interface {
	// SSM
	GetParameter(ctx context.Context, name string) (string, error)
	GetParametersByPath(ctx context.Context, path string) (map[string]string, error)
	SetParameter(ctx context.Context, name string, value string) error
	// CloudFormation
	DescribeStack(ctx context.Context, name string) (*cfnTypes.Stack, error)
	DestroyStack(ctx context.Context, name string) error
	// ECR
	GetECRLogin(ctx context.Context) (string, string, error)
	CopyFromS3(ctx context.Context, bucket, prefix, dest string) error
	SyncToS3(ctx context.Context, src, bucket, prefix string, quiet bool) error
}

type AWS struct {
	config *aws.Config
}

func New(config *aws.Config) *AWS {
	return &AWS{
		config: config,
	}
}

// SSM Parameter Store

func (a *AWS) SetParameter(ctx context.Context, name string, value string) error {
	ssmSvc := ssm.NewFromConfig(*a.config)
	_, err := ssmSvc.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      &name,
		Value:     &value,
		Overwrite: aws.Bool(true),
		Type:      ssmTypes.ParameterTypeString,
	})
	return err
}

func (a *AWS) GetParameter(ctx context.Context, name string) (string, error) {
	ssmSvc := ssm.NewFromConfig(*a.config)
	result, err := ssmSvc.GetParameter(ctx, &ssm.GetParameterInput{
		Name: &name,
	})
	if err != nil {
		return "", err
	}
	return *result.Parameter.Value, nil
}

func (a *AWS) GetParametersByPath(ctx context.Context, path string) (map[string]string, error) {
	ssmSvc := ssm.NewFromConfig(*a.config)
	paginator := ssm.NewGetParametersByPathPaginator(ssmSvc, &ssm.GetParametersByPathInput{
		Path:           &path,
		WithDecryption: aws.Bool(true),
	})
	params := make(map[string]string)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range output.Parameters {
			params[*p.Name] = *p.Value
		}
	}
	return params, nil
}

// Cloudformation

func (a *AWS) DescribeStack(ctx context.Context, stackName string) (*cfnTypes.Stack, error) {
	cfSvc := cloudformation.NewFromConfig(*a.config)
	result, err := cfSvc.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Stacks) != 1 {
		return nil, errors.New("invalid stack returned by AWS")
	}
	return &result.Stacks[0], nil
}

func (a *AWS) DestroyStack(ctx context.Context, stackName string) error {
	cfSvc := cloudformation.NewFromConfig(*a.config)
	_, err := cfSvc.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: &stackName,
	})
	return err
}

// ECR

func decodeECRToken(token string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", err
	}
	creds := strings.Split(string(decoded), ":")
	if len(creds) != 2 {
		return "", "", errors.New("invalid credentials returned by AWS")
	}
	return creds[0], creds[1], nil
}

// GetECRPassword returns the password for the given ECR registry
func (a *AWS) GetECRLogin(ctx context.Context) (string, string, error) {
	ecrSvc := ecr.NewFromConfig(*a.config)
	result, err := ecrSvc.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", "", err
	}
	if len(result.AuthorizationData) != 1 {
		return "", "", errors.New("invalid authorization data returned by AWS")
	}
	return decodeECRToken(*result.AuthorizationData[0].AuthorizationToken)
}
