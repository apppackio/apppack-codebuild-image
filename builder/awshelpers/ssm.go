package awshelpers

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func SetParameter(awsCfg aws.Config, name string, value string) error {
	ssmSvc := ssm.NewFromConfig(awsCfg)
	_, err := ssmSvc.PutParameter(context.Background(), &ssm.PutParameterInput{
		Name:      &name,
		Value:     &value,
		Overwrite: aws.Bool(true),
		Type:      types.ParameterTypeString,
	})
	return err
}

func GetParameter(awsCfg aws.Config, name string) (string, error) {
	ssmSvc := ssm.NewFromConfig(awsCfg)
	result, err := ssmSvc.GetParameter(context.Background(), &ssm.GetParameterInput{
		Name: &name,
	})
	if err != nil {
		return "", err
	}
	return *result.Parameter.Value, nil
}

func GetParametersByPath(awsCfg aws.Config, path string) (map[string]string, error) {
	ssmSvc := ssm.NewFromConfig(awsCfg)
	result, err := ssmSvc.GetParametersByPath(context.Background(), &ssm.GetParametersByPathInput{
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
