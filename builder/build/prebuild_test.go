package build

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
)

const CodebuildBuildId = "codebuild-build-id"

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

func (m *MockAWS) CopyFromS3(bucket, prefix, dest string) error {
	args := m.Called(bucket, dest)
	return args.Error(0)
}
func (m *MockAWS) SyncToS3(src, bucket, prefix string, quiet bool) error {
	args := m.Called(src, bucket, prefix, quiet)
	return args.Error(0)
}

type MockFilesystem struct {
	mock.Mock
}

func (m *MockFilesystem) CreateIfNotExists() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) WriteSkipBuild(s string) error {
	args := m.Called(s)
	return args.Error(0)
}
func (m *MockFilesystem) ShouldSkipBuild(s string) (bool, error) {
	args := m.Called(s)
	return args.Bool(0), args.Error(1)
}
func (m *MockFilesystem) UnpackTarArchive(r io.ReadCloser) error {
	args := m.Called(r)
	return args.Error(0)
}
func (m *MockFilesystem) WriteEnvFile(e *map[string]string) error {
	args := m.Called(e)
	return args.Error(0)
}
func (m *MockFilesystem) ReadEnvFile() (*map[string]string, error) {
	args := m.Called()
	return args.Get(0).(*map[string]string), args.Error(1)
}
func (m *MockFilesystem) WriteCommitTxt() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) MvGitDir() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockFilesystem) GitSha() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockFilesystem) CreateLogFile(s string) (*os.File, error) {
	args := m.Called(s)
	return args.Get(0).(*os.File), args.Error(0)
}
func (m *MockFilesystem) FileExists(s string) (bool, error) {
	args := m.Called(s)
	return args.Bool(0), args.Error(1)
}
func (m *MockFilesystem) WriteTomlToFile(s string, v interface{}) error {
	args := m.Called(s, v)
	return args.Error(0)
}
func (m *MockFilesystem) WriteJsonToFile(s string, v interface{}) error {
	args := m.Called(s, v)
	return args.Error(0)
}

type MockContainers struct {
	mock.Mock
}

func (c *MockContainers) Close() error {
	args := c.Called()
	return args.Error(0)
}
func (c *MockContainers) CreateNetwork(s string) error {
	args := c.Called(s)
	return args.Error(0)
}
func (c *MockContainers) PullImage(s string) error {
	args := c.Called(s)
	return args.Error(0)
}
func (c *MockContainers) PushImage(s string) error {
	args := c.Called(s)
	return args.Error(0)
}
func (c *MockContainers) BuildImage(s string, b *containers.BuildConfig) error {
	args := c.Called(s, b)
	return args.Error(0)
}
func (c *MockContainers) CreateContainer(s1 string, cfg *container.Config) (*string, error) {
	args := c.Called(s1, cfg)
	return args.Get(0).(*string), args.Error(1)
}
func (c *MockContainers) RunContainer(s1 string, s2 string, cfg *container.Config) error {
	args := c.Called(s1, s2, cfg)
	return args.Error(0)
}
func (c *MockContainers) GetContainerFile(s1 string, s2 string) (io.ReadCloser, error) {
	args := c.Called(s1, s2)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (c *MockContainers) WaitForExit(s string) (int, error) {
	args := c.Called(s)
	return args.Int(0), args.Error(1)
}
func (c *MockContainers) AttachLogs(s string, w1, w2 io.Writer) error {
	args := c.Called(s, w1, w2)
	return args.Error(0)
}
func (c *MockContainers) DeleteContainer(s string) error {
	args := c.Called(s)
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
		Ctx:                    testContext,
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
		Ctx:                    testContext,
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
		Ctx:                    testContext,
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
	mockedState.On("WriteSkipBuild", CodebuildBuildId).Return(nil)
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
		CodebuildBuildId:       CodebuildBuildId,
		aws:                    mockedAWS,
		state:                  mockedState,
		Ctx:                    testContext,
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
		CodebuildBuildId:       CodebuildBuildId,
		aws:                    mockedAWS,
		state:                  mockedState,
		Ctx:                    testContext,
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
		Ctx:                    testContext,
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
		CodebuildBuildId:       CodebuildBuildId,
		aws:                    mockedAWS,
		state:                  mockedState,
		Ctx:                    testContext,
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
		CodebuildBuildId:       CodebuildBuildId,
		aws:                    mockedAWS,
		state:                  mockedState,
		Ctx:                    testContext,
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
		CodebuildBuildId:       CodebuildBuildId,
		aws:                    mockedAWS,
		state:                  mockedState,
		Ctx:                    testContext,
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
		CodebuildBuildId:       CodebuildBuildId,
		aws:                    mockedAWS,
		state:                  mockedState,
		Ctx:                    testContext,
	}
	if b.HandlePR() != nil {
		t.Error("HandlePR should return nil")
	}
	mockedAWS.AssertExpectations(t)
	mockedState.AssertExpectations(t)
}

func TestRemoveDuplicateStr(t *testing.T) {
	slice := []string{"a", "b", "c", "a", "b"}
	expected := []string{"a", "b", "c"}
	actual := removeDuplicateStr(slice)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestStartAddons(t *testing.T) {
	mockedContainers := new(MockContainers)
	mockedContainers.On(
		"RunContainer",
		"redis", CodebuildBuildId, &container.Config{Image: "redis:alpine"},
	).Return(nil)
	mockedContainers.On(
		"RunContainer",
		"db", CodebuildBuildId, &container.Config{Image: "postgres:alpine"},
	).Return(nil)
	b := Build{
		CodebuildBuildId: CodebuildBuildId,
		Ctx:              testContext,
		AppJSON: &AppJSON{
			Environments: map[string]Environment{
				"test": {
					Addons: []string{"heroku-redis:in-dyno", "heroku-postgresql:in-dyno"},
				},
			},
		},
		containers: mockedContainers,
	}
	env, err := b.StartAddons()
	if err != nil {
		t.Error("StartAddons should not return an error")
	}
	if env["REDIS_URL"] != "redis://redis:6379" {
		t.Error("REDIS_URL not set")
	}
	if env["DATABASE_URL"] != "postgres://postgres:postgres@db:5432/postgres" {
		t.Error("DATABASE_URL not set")
	}
}

func TestImageName(t *testing.T) {
	mockedState := emptyState()
	repo := "123456789012.dkr.ecr.us-east-1.amazonaws.com/test-app"
	sha := "1234567890123456789012345678901234567890"
	b := Build{
		ECRRepo: repo,
		state:   mockedState,
		Ctx:     testContext,
	}
	mockedState.On("GitSha").Return(sha, nil)
	expected := repo + ":" + sha
	actual, err := b.ImageName()
	if err != nil {
		t.Errorf("imageName returned an error: %s", err)
	}
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}
