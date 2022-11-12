package build

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/stretchr/testify/mock"
)

type MockAWS struct {
	mock.Mock
}

// SSM Parameter Store

func (m *MockAWS) SetParameter(name string, value string) error {
	args := m.Called(name, value)
	return args.Error(0)
}

func (m *MockAWS) GetParameter(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockAWS) GetParametersByPath(path string) (map[string]string, error) {
	args := m.Called(path)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockAWS) DescribeStack(stackName string) (*types.Stack, error) {
	args := m.Called(stackName)
	return args.Get(0).(*types.Stack), args.Error(1)
}

func (m *MockAWS) DestroyStack(stackName string) error {
	args := m.Called(stackName)
	return args.Error(0)
}

func (m *MockAWS) GetECRLogin() (string, string, error) {
	args := m.Called()
	return args.String(0), args.String(1), args.Error(2)
}

func TestHandlePRSkip(t *testing.T) {
	b := Build{
		Pipeline: false,
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil when Pipeline is false")
	}
	b = Build{
		Pipeline:               true,
		CodebuildSourceVersion: "refs/head/main",
	}
	if b.HandlePR() == nil {
		t.Error("HandlePR should return an error when CodebuildSourceVersion does not start with `pr/`")
	}
}

func TestHandlePRReviewAppCreated(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"SetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
		fmt.Sprintf("{\"pull_request\":\"%s\",\"status\":\"created\"}", pr),
	).Return(nil)

	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		ReviewAppStatus:        "created",
		aws:                    mockedAWS,
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil when setting the PR status")
	}
	mockedAWS.AssertExpectations(t)
}

func TestHandlePRAWSFailed(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"SetParameter",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Return(fmt.Errorf("failed to set parameter"))

	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		ReviewAppStatus:        "created",
		aws:                    mockedAWS,
	}
	if b.HandlePR() == nil {
		t.Error("HandlePR should return error AWS fails")
	}
	mockedAWS.AssertExpectations(t)
}

func TestHandlePROpened(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"SetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
		fmt.Sprintf("{\"pull_request\":\"%s\",\"status\":\"open\"}", pr),
	).Return(nil)

	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_CREATED",
		aws:                    mockedAWS,
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil when setting the PR status")
	}
	mockedAWS.AssertExpectations(t)
}

func TestHandlePRUpdatedNotExists(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"GetParameter",
		mock.AnythingOfType("string"),
	).Return(
		"",
		fmt.Errorf("parameter does not exist"),
	)
	mockedAWS.On(
		"SetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
		fmt.Sprintf("{\"pull_request\":\"%s\",\"status\":\"open\"}", pr),
	).Return(nil)

	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_UPDATED",
		aws:                    mockedAWS,
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil when setting the PR status")
	}
	mockedAWS.AssertExpectations(t)
}
