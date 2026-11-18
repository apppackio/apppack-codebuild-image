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
	args, env, missing, err := dockerBuildArgs("Dockerfile", config)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
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
	args, env, missing, err := dockerBuildArgs("Dockerfile", config)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
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
	if i < 0 || args[i+1] != "id=NPM_TOKEN,env=APPPACK_SECRET_NPM_TOKEN" {
		t.Errorf("expected --secret id=NPM_TOKEN,env=APPPACK_SECRET_NPM_TOKEN in %v", args)
	}
	if args[len(args)-1] != "." {
		t.Errorf("expected build context last, got %v", args)
	}
	if !slices.Equal(env, []string{"APPPACK_SECRET_NPM_TOKEN=s3cr3t-value"}) {
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
	args, env, missing, err := dockerBuildArgs("Dockerfile", config)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
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

// A config var sharing a name with a variable the docker CLI itself reads must
// not reach the CLI's environment under that name.
func TestDockerBuildArgsSecretDoesNotShadowCLIEnv(t *testing.T) {
	for _, name := range []string{"PATH", "HOME", "DOCKER_CONFIG"} {
		config := testBuildConfig(map[string]string{name: "clobbered"}, []string{name})
		_, env, _, err := dockerBuildArgs("Dockerfile", config)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		for _, e := range env {
			if strings.HasPrefix(e, name+"=") {
				t.Errorf("%s would shadow the docker CLI's own variable: %v", name, env)
			}
		}
		if !slices.Contains(env, secretEnvPrefix+name+"=clobbered") {
			t.Errorf("expected %s under the prefix, got %v", name, env)
		}
	}
}

// dockerBuildArgs runs in a separate `apppack-builder build` invocation from
// the prebuild that validates apppack.toml, so it rechecks the name itself
// rather than trusting that a prebuild ran.
func TestDockerBuildArgsRejectsInvalidSecretName(t *testing.T) {
	config := testBuildConfig(
		map[string]string{"NPM_TOKEN,src=/root/.aws/credentials": "x"},
		[]string{"NPM_TOKEN,src=/root/.aws/credentials"},
	)
	if _, _, _, err := dockerBuildArgs("Dockerfile", config); err == nil {
		t.Error("expected an error for a secret name that is not an env var name")
	}
}
