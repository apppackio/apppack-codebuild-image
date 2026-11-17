package containers

import (
	"slices"
	"strings"
	"testing"
)

func testBuildConfig(env map[string]string, secrets []string) *BuildConfig {
	return &BuildConfig{
		Image:    "repo:abc123",
		CacheDir: "/tmp/cache",
		Env:      env,
		Secrets:  secrets,
	}
}

func TestDockerBuildArgsWithoutSecrets(t *testing.T) {
	config := testBuildConfig(map[string]string{"FOO": "bar", "CI": "true"}, nil)
	args, env, missing := dockerBuildArgs("Dockerfile", config)
	expected := []string{
		"buildx",
		"build",
		"--tag", "repo:abc123",
		"--progress", "plain",
		"--cache-to", "type=local,dest=/tmp/cache",
		"--cache-from", "type=local,src=/tmp/cache",
		"--file", "Dockerfile",
		"--load",
		"--build-arg", "CI=true",
		"--build-arg", "FOO=bar",
		".",
	}
	if !slices.Equal(args, expected) {
		t.Errorf("expected args %v, got %v", expected, args)
	}
	if env != nil {
		t.Errorf("expected no extra env, got %v", env)
	}
	if missing != nil {
		t.Errorf("expected no missing secrets, got %v", missing)
	}
}

func TestDockerBuildArgsWithSecrets(t *testing.T) {
	config := testBuildConfig(
		map[string]string{"FOO": "bar", "NPM_TOKEN": "s3cr3t-value"},
		[]string{"NPM_TOKEN"},
	)
	args, env, missing := dockerBuildArgs("Dockerfile", config)
	for _, a := range args {
		if strings.Contains(a, "s3cr3t-value") {
			t.Errorf("secret value leaked into args: %v", args)
		}
	}
	if slices.Contains(args, "NPM_TOKEN=s3cr3t-value") {
		t.Errorf("secret passed as a build arg: %v", args)
	}
	if !slices.Contains(args, "FOO=bar") {
		t.Errorf("expected FOO to stay a build arg: %v", args)
	}
	i := slices.Index(args, "--secret")
	if i < 0 || args[i+1] != "id=NPM_TOKEN,env=NPM_TOKEN" {
		t.Errorf("expected --secret id=NPM_TOKEN,env=NPM_TOKEN in %v", args)
	}
	if args[len(args)-1] != "." {
		t.Errorf("expected build context last, got %v", args)
	}
	if !slices.Equal(env, []string{"NPM_TOKEN=s3cr3t-value"}) {
		t.Errorf("expected secret in env, got %v", env)
	}
	if missing != nil {
		t.Errorf("expected no missing secrets, got %v", missing)
	}
}

func TestDockerBuildArgsMissingSecret(t *testing.T) {
	config := testBuildConfig(
		map[string]string{"FOO": "bar"},
		[]string{"NPM_TOKEN", "NPM_TOKEN"},
	)
	args, env, missing := dockerBuildArgs("Dockerfile", config)
	if slices.Contains(args, "--secret") {
		t.Errorf("expected no --secret for a missing config var: %v", args)
	}
	if env != nil {
		t.Errorf("expected no extra env, got %v", env)
	}
	if !slices.Equal(missing, []string{"NPM_TOKEN"}) {
		t.Errorf("expected NPM_TOKEN reported missing once, got %v", missing)
	}
}
