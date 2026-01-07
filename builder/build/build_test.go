package build

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
		Ctx:              testContext,
	}
	env, err := b.LoadBuildEnv()
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
		Ctx:                    testContext,
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

func TestGetMaxCacheSizeGB(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{"unset uses default", "", DefaultMaxCacheSizeGB},
		{"valid value", "10", 10},
		{"zero disables limit", "0", 0},
		{"invalid string falls back to default", "abc", DefaultMaxCacheSizeGB},
		{"negative falls back to default", "-5", DefaultMaxCacheSizeGB},
		{"float falls back to default", "7.5", DefaultMaxCacheSizeGB},
		{"whitespace falls back to default", " ", DefaultMaxCacheSizeGB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv(MaxCacheSizeEnvVar)
			} else {
				os.Setenv(MaxCacheSizeEnvVar, tt.envValue)
			}
			defer os.Unsetenv(MaxCacheSizeEnvVar)

			got := getMaxCacheSizeGB()
			if got != tt.want {
				t.Errorf("getMaxCacheSizeGB() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDirSize(t *testing.T) {
	// Create a temp directory with known file sizes
	tmpDir, err := os.MkdirTemp("", "dirsize-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files with known sizes
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file1, make([]byte, 1000), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, make([]byte, 500), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subdirectory with file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	file3 := filepath.Join(subDir, "file3.txt")
	if err := os.WriteFile(file3, make([]byte, 250), 0644); err != nil {
		t.Fatal(err)
	}

	size, err := dirSize(tmpDir)
	if err != nil {
		t.Errorf("dirSize() error = %v", err)
	}
	expectedSize := int64(1750)
	if size != expectedSize {
		t.Errorf("dirSize() = %d, want %d", size, expectedSize)
	}
}

func TestDirSizeNonExistent(t *testing.T) {
	_, err := dirSize("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("dirSize() expected error for non-existent path, got nil")
	}
}
