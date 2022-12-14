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
	mockedAWS.On(
		"GetParametersByPath",
		fmt.Sprintf("/apppack/apps/%s/config/", appName),
	).Return(map[string]string{"FOO": "bar"}, nil)
	mockedState := emptyState()
	mockedState.On("ReadEnvFile").Return(&map[string]string{"FOO": "override"}, nil)
	b := Build{
		Appname:          appName,
		CodebuildBuildId: CodebuildBuildId,
		aws:              mockedAWS,
		state:            mockedState,
		Log:              logging.NewSimpleLogger(os.Stderr),
	}
	env, err := b.LoadEnv()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if env["FOO"] != "override" {
		t.Errorf("expected FOO=override, got %s", env["FOO"])
	}
}

func TestLoadEnvInheritance(t *testing.T) {
	appName := "test-app"
	pr := "pr/123"
	mockedAWS := new(MockAWS)
	mockedAWS.On(
		"GetParametersByPath",
		fmt.Sprintf("/apppack/pipelines/%s/config/", appName),
	).Return(map[string]string{"FOO": "bar"}, nil)
	mockedAWS.On(
		"GetParametersByPath",
		fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s/config/", appName, pr),
	).Return(map[string]string{"FOO": "bar2"}, nil)
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
	env, err := b.LoadEnv()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if env["FOO"] != "bar2" {
		t.Errorf("expected FOO=bar2, got %s", env["FOO"])
	}

	// Test that env file overrides pipeline and review app config
	envFileCall.Return(&map[string]string{"FOO": "override"}, nil)
	env, err = b.LoadEnv()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if env["FOO"] != "override" {
		t.Errorf("expected FOO=override, got %s", env["FOO"])
	}
}
