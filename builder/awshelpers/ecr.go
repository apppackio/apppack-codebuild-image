package awshelpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// GetECRPassword returns the password for the given ECR registry
func GetECRLogin() (string, string, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", "", err
	}
	ecrSvc := ecr.NewFromConfig(cfg)
	result, err := ecrSvc.GetAuthorizationToken(context.Background(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(*result.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return "", "", err
	}
	creds := strings.Split(string(decoded), ":")
	if len(creds) != 2 {
		return "", "", fmt.Errorf("invalid credentials returned by AWS")
	}
	return creds[0], creds[1], nil
}
