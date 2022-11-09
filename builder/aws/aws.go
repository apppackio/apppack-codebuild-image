package aws

import (
	"context"
	"encoding/base64"
	"fmt"
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
	GetParameter(name string) (string, error)
	GetParametersByPath(path string) (map[string]string, error)
	SetParameter(name string, value string) error
	// CloudFormation
	DescribeStack(name string) (*cfnTypes.Stack, error)
	DestroyStack(name string) error
	// ECR
	GetECRLogin() (string, string, error)
}

type AWS struct {
	config  *aws.Config
	context context.Context
}

func New(config *aws.Config, context context.Context) *AWS {
	return &AWS{
		config:  config,
		context: context,
	}
}

// SSM Parameter Store

func (a *AWS) SetParameter(name string, value string) error {
	ssmSvc := ssm.NewFromConfig(*a.config)
	_, err := ssmSvc.PutParameter(a.context, &ssm.PutParameterInput{
		Name:      &name,
		Value:     &value,
		Overwrite: aws.Bool(true),
		Type:      ssmTypes.ParameterTypeString,
	})
	return err
}

func (a *AWS) GetParameter(name string) (string, error) {
	ssmSvc := ssm.NewFromConfig(*a.config)
	result, err := ssmSvc.GetParameter(a.context, &ssm.GetParameterInput{
		Name: &name,
	})
	if err != nil {
		return "", err
	}
	return *result.Parameter.Value, nil
}

func (a *AWS) GetParametersByPath(path string) (map[string]string, error) {
	ssmSvc := ssm.NewFromConfig(*a.config)
	result, err := ssmSvc.GetParametersByPath(a.context, &ssm.GetParametersByPathInput{
		Path:           &path,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	for _, p := range result.Parameters {
		params[*p.Name] = *p.Value
	}
	return params, nil
}

// Cloudformation

func (a *AWS) DescribeStack(stackName string) (*cfnTypes.Stack, error) {
	cfSvc := cloudformation.NewFromConfig(*a.config)
	result, err := cfSvc.DescribeStacks(a.context, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Stacks) != 1 {
		return nil, fmt.Errorf("invalid stack returned by AWS")
	}
	return &result.Stacks[0], nil
}

func (a *AWS) DestroyStack(stackName string) error {
	cfSvc := cloudformation.NewFromConfig(*a.config)
	_, err := cfSvc.DeleteStack(a.context, &cloudformation.DeleteStackInput{
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
		return "", "", fmt.Errorf("invalid credentials returned by AWS")
	}
	return creds[0], creds[1], nil
}

// GetECRPassword returns the password for the given ECR registry
func (a *AWS) GetECRLogin() (string, string, error) {
	ecrSvc := ecr.NewFromConfig(*a.config)
	result, err := ecrSvc.GetAuthorizationToken(context.Background(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", "", err
	}
	if len(result.AuthorizationData) != 1 {
		return "", "", fmt.Errorf("invalid authorization data returned by AWS")
	}
	return decodeECRToken(*result.AuthorizationData[0].AuthorizationToken)

}
