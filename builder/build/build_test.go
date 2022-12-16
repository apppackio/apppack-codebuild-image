package build

import (
	"fmt"
	"os"
	"testing"

	"github.com/buildpacks/pack/pkg/logging"
)

func TestLoadEnv(t *testing.T) {
	appName := "test-app"
	mockedAWS := new(MockAWS)
	appConfigPrefix := fmt.Sprintf("/apppack/apps/%s/config/", appName)
	mockedAWS.On(
		"GetParametersByPath",
		appConfigPrefix,
	).Return(map[string]string{appConfigPrefix + "FOO": "bar"}, nil)
	mockedState := emptyState()
	mockedState.On("ReadEnvFile").Return(&map[string]string{"FOO": "override"}, nil)
	b := Build{
		Appname:          appName,
		CodebuildBuildId: CodebuildBuildId,
		aws:              mockedAWS,
		state:            mockedState,
		Log:              logging.NewSimpleLogger(os.Stderr),
	}
	env, err := b.LoadBuildEnv()
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if env["FOO"] != "override" {
		t.Errorf("expected FOO=override, got %s", env["FOO"])
	}
}

func TestLoadEnvInheritance(t *testing.T) {
	appName := "test-app"
	pr := "pr/123"
	pipelineConfigPrefix := fmt.Sprintf("/apppack/pipelines/%s/config/", appName)
	reviewAppConfigPrefix := fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s/config/", appName, pr)
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"GetParametersByPath",
		pipelineConfigPrefix,
	).Return(map[string]string{pipelineConfigPrefix + "FOO": "bar"}, nil)
	mockedAWS.On(
		"GetParametersByPath",
		reviewAppConfigPrefix,
	).Return(map[string]string{reviewAppConfigPrefix + "FOO": "bar2"}, nil)
	mockedState := emptyState()
	envFileCall := mockedState.On("ReadEnvFile").Return(&map[string]string{}, nil)
	b := Build{
		Appname:                appName,
		Pipeline:               true,
		CodebuildBuildId:       CodebuildBuildId,
		CodebuildSourceVersion: pr,
		aws:                    mockedAWS,
		state:                  mockedState,
		Log:                    logging.NewSimpleLogger(os.Stderr),
	}

	// Test that review app config overrides pipeline config
	env, err := b.LoadBuildEnv()
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if env["FOO"] != "bar2" {
		t.Errorf("expected FOO=bar2, got %s", env["FOO"])
	}

	// Test that env file overrides pipeline and review app config
	envFileCall.Return(&map[string]string{"FOO": "override"}, nil)
	env, err = b.LoadBuildEnv()
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if env["FOO"] != "override" {
		t.Errorf("expected FOO=override, got %s", env["FOO"])
	}
}
