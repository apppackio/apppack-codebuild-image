package build

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/buildpacks/pack/pkg/logging"
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

type MockFilesystem struct {
	mock.Mock
}

func (m *MockFilesystem) CreateIfNotExists() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) WriteCommitTxt() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) WriteSkipBuild(string) error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) ShouldSkipBuild(string) (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}
func (m *MockFilesystem) WriteMetadataToml(io.ReadCloser) error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) MvGitDir() error {
	args := m.Called()
	return args.Error(0)
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
		Log:                    logging.NewSimpleLogger(os.Stderr),
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
		"GetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
	).Return(
		"",
		fmt.Errorf("parameter does not exist"),
	)
	mockedAWS.On(
		"SetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
		fmt.Sprintf("{\"pull_request\":\"%s\",\"status\":\"created\"}", pr),
	).Return(nil)

	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CreateReviewApp:        true,
		aws:                    mockedAWS,
		Log:                    logging.NewSimpleLogger(os.Stderr),
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
		"GetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
	).Return(
		"",
		fmt.Errorf("parameter does not exist"),
	)
	mockedAWS.On(
		"SetParameter",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Return(fmt.Errorf("failed to set parameter"))

	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CreateReviewApp:        true,
		aws:                    mockedAWS,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() == nil {
		t.Error("HandlePR should return error AWS fails")
	}
	mockedAWS.AssertExpectations(t)
}

func emptyState() *MockFilesystem {
	mockedState := new(MockFilesystem)
	mockedState.On("CreateIfNotExists").Return(nil)
	mockedState.On("WriteCommitTxt").Return(nil)
	mockedState.On("WriteSkipBuild").Return(nil)
	return mockedState
}

func reviewAppStatus(appName string, pr string, status string) *MockAWS {
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"GetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
	).Return(
		fmt.Sprintf("{\"pull_request\":\"%s\",\"status\":\"%s\"}", pr, status),
		nil,
	)
	return mockedAWS
}

func TestHandlePROpened(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedState := emptyState()
	mockedAWS.On(
		"GetParameter",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", appName, pr),
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
		CodebuildWebhookEvent:  "PULL_REQUEST_CREATED",
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
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
	mockedState := emptyState()
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_UPDATED",
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
}

func TestHandlePRUpdatedReviewAppCreated(t *testing.T) {
	// no action needed for review apps that are created
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := reviewAppStatus(appName, pr, "created")
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_UPDATED",
		aws:                    mockedAWS,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
}

func TestHandlePRUpdatedClosed(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := reviewAppStatus(appName, pr, "closed")
	mockedState := emptyState()
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_UPDATED",
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
}

func TestHandlePRPushDoesNotExist(t *testing.T) {
	// this is a weird one
	// it would happen if the backing parameter changed between the two times it is read
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
	mockedState := emptyState()
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
}

func TestHandlePRMerged(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"DescribeStack",
		fmt.Sprintf("apppack-reviewapp-%s%s", appName, strings.Split(pr, "/")[1]),
	).Return(&types.Stack{}, fmt.Errorf("stack does not exist"))
	mockedState := emptyState()
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_MERGED",
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
}

func TestHandlePRMergedAndDestroy(t *testing.T) {
	pr := "pr/123"
	appName := "test-app"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"DescribeStack",
		fmt.Sprintf("apppack-reviewapp-%s%s", appName, strings.Split(pr, "/")[1]),
	).Return(&types.Stack{}, nil)
	mockedAWS.On(
		"DestroyStack",
		fmt.Sprintf("apppack-reviewapp-%s%s", appName, strings.Split(pr, "/")[1]),
	).Return(nil)
	mockedState := emptyState()
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildSourceVersion: pr,
		CodebuildWebhookEvent:  "PULL_REQUEST_MERGED",
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
}
