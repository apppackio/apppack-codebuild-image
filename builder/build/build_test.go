package build

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
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
		logger:           &testLogger,
	}
	env, err := b.LoadBuildEnv(testContext)
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if env["CI"] != "true" {
		t.Errorf("expected CI=true, got %s", env["CI"])
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
		logger:                 &testLogger,
	}

	// Test that review app config overrides pipeline config
	env, err := b.LoadBuildEnv(testContext)
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if env["FOO"] != "bar2" {
		t.Errorf("expected FOO=bar2, got %s", env["FOO"])
	}

	// Test that env file overrides pipeline and review app config
	envFileCall.Return(&map[string]string{"FOO": "override"}, nil)
	env, err = b.LoadBuildEnv(testContext)
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if env["FOO"] != "override" {
		t.Errorf("expected FOO=override, got %s", env["FOO"])
	}
}

func TestLoadBuildEnvCIVars(t *testing.T) {
	appName := "test-app"
	mockedAWS := new(MockAWS)
	appConfigPrefix := fmt.Sprintf("/apppack/apps/%s/config/", appName)
	mockedAWS.On("GetParametersByPath", appConfigPrefix).Return(map[string]string{}, nil)
	mockedState := emptyState()
	mockedState.On("ReadEnvFile").Return(&map[string]string{}, nil)

	t.Setenv("CODEBUILD_WEBHOOK_HEAD_REF", "refs/heads/main")
	t.Setenv("CODEBUILD_RESOLVED_SOURCE_VERSION", "deadbeef1234")
	t.Setenv("CODEBUILD_START_TIME", "1234567890")
	t.Setenv("CODEBUILD_SOURCE_REPO_URL", "https://github.com/org/repo")

	b := Build{
		Appname:          appName,
		CodebuildBuildId: CodebuildBuildId,
		aws:              mockedAWS,
		state:            mockedState,
		logger:           &testLogger,
	}
	env, err := b.LoadBuildEnv(testContext)
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	if env["CI_COMMIT_REF"] != "refs/heads/main" {
		t.Errorf("expected CI_COMMIT_REF=refs/heads/main, got %s", env["CI_COMMIT_REF"])
	}
	if env["CI_COMMIT_SHA"] != "deadbeef1234" {
		t.Errorf("expected CI_COMMIT_SHA=deadbeef1234, got %s", env["CI_COMMIT_SHA"])
	}
	if env["CI_BUILD_STARTED_AT"] != "1234567890" {
		t.Errorf("expected CI_BUILD_STARTED_AT=1234567890, got %s", env["CI_BUILD_STARTED_AT"])
	}
	if env["CI_REPOSITORY_URL"] != "https://github.com/org/repo" {
		t.Errorf("expected CI_REPOSITORY_URL=https://github.com/org/repo, got %s", env["CI_REPOSITORY_URL"])
	}
}

func TestLoadBuildEnvCIVarsFallback(t *testing.T) {
	// When CODEBUILD_WEBHOOK_HEAD_REF is not set, CI_COMMIT_REF should fall back to CODEBUILD_SOURCE_VERSION
	appName := "test-app"
	mockedAWS := new(MockAWS)
	appConfigPrefix := fmt.Sprintf("/apppack/apps/%s/config/", appName)
	mockedAWS.On("GetParametersByPath", appConfigPrefix).Return(map[string]string{}, nil)
	mockedState := emptyState()
	mockedState.On("ReadEnvFile").Return(&map[string]string{}, nil)

	t.Setenv("CODEBUILD_WEBHOOK_HEAD_REF", "")
	t.Setenv("CODEBUILD_SOURCE_VERSION", "refs/heads/feature-branch")

	b := Build{
		Appname:          appName,
		CodebuildBuildId: CodebuildBuildId,
		aws:              mockedAWS,
		state:            mockedState,
		logger:           &testLogger,
	}
	env, err := b.LoadBuildEnv(testContext)
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	if env["CI_COMMIT_REF"] != "refs/heads/feature-branch" {
		t.Errorf("expected CI_COMMIT_REF=refs/heads/feature-branch, got %s", env["CI_COMMIT_REF"])
	}
}

func TestGenerateDockerEnvStrings(t *testing.T) {
	env := map[string]string{
		"FOO": "bar",
		"BAR": "baz",
	}
	expected := []string{
		"FOO=bar",
		"BAR=baz",
	}
	actual := generateDockerEnvStrings(env)
	if len(actual) != len(expected) {
		t.Errorf("expected %d elements, got %d", len(expected), len(actual))
	}
}

func TestWarnDeprecatedBuilder(t *testing.T) {
	tests := []struct {
		builder string
		warns   bool
	}{
		{"heroku/builder:22", true},
		{"heroku/builder:24", false},
		{"heroku/builder:26", false},
		{"custom/builder:latest", false},
	}
	for _, tt := range tests {
		t.Run(tt.builder, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			b := Build{logger: &logger}
			b.warnDeprecatedBuilder(tt.builder)
			out := buf.String()
			if tt.warns {
				if !strings.Contains(out, `"level":"warn"`) || !strings.Contains(out, "heroku/builder:22 is deprecated") {
					t.Errorf("expected deprecation warning, got %q", out)
				}
			} else if out != "" {
				t.Errorf("expected no output, got %q", out)
			}
		})
	}
}

// The warning must fire for every way of arriving at builder 22:
// the default, app.json, and apppack.toml.
func TestBuildpackBuildersDeprecationSources(t *testing.T) {
	tests := []struct {
		name  string
		build Build
		warns bool
	}{
		{"default", Build{AppJSON: &AppJSON{Stack: DefaultStack}}, true},
		{"app.json 22", Build{AppJSON: &AppJSON{Stack: "heroku-22"}}, true},
		{"app.json 24", Build{AppJSON: &AppJSON{Stack: "heroku-24"}}, false},
		{"app.json 26", Build{AppJSON: &AppJSON{Stack: "heroku-26"}}, false},
		{"apppack.toml 22", Build{AppJSON: &AppJSON{Stack: "heroku-24"}, AppPackToml: &AppPackToml{Build: AppPackTomlBuild{Builder: "heroku/builder:22"}}}, true},
		{"apppack.toml 24", Build{AppJSON: &AppJSON{Stack: DefaultStack}, AppPackToml: &AppPackToml{Build: AppPackTomlBuild{Builder: "heroku/builder:24"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			b := tt.build
			b.logger = &logger
			if b.AppPackToml == nil {
				b.AppPackToml = &AppPackToml{}
			}
			b.warnDeprecatedBuilder(b.BuildpackBuilders()[0])
			if got := strings.Contains(buf.String(), "deprecated"); got != tt.warns {
				t.Errorf("warned=%v, want %v (output %q)", got, tt.warns, buf.String())
			}
		})
	}
}
