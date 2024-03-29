package build

import "testing"

func TestAppPackTomlValidateBuildpackAndDockerfile(t *testing.T) {
	c := AppPackToml{
		Build: AppPackTomlBuild{
			System:     "dockerfile",
			Buildpacks: []string{"heroku/ruby"},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error")
	}
}

func TestAppPackTomlValidateBuildpackAndService(t *testing.T) {
	c := AppPackToml{
		Build: AppPackTomlBuild{
			Buildpacks: []string{"heroku/ruby"},
		},
		Services: map[string]AppPackTomlService{"web": {Command: "echo hello"}},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error")
	}
}

func TestAppPackTomlValidateEnv(t *testing.T) {
	c := AppPackToml{
		Test: AppPackTomlTest{
			Env: []string{"FOOBAR"},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error")
	}
}

func TestAppPackTomlValidateValid(t *testing.T) {
	c := AppPackToml{
		Build: AppPackTomlBuild{
			System: "dockerfile",
		},
		Services: map[string]AppPackTomlService{"web": {Command: "echo hello"}},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestAppPackTomlGetTestEnv(t *testing.T) {
	c := AppPackToml{
		Test: AppPackTomlTest{
			Env: []string{"FOO=BAR", "BAZ=QUX"},
		},
	}
	env := c.GetTestEnv()
	if len(env) != 3 {
		t.Errorf("expected 2 env vars, got %d", len(env))
	}
	if env["FOO"] != "BAR" {
		t.Errorf("expected FOO=BAR, got %s", env["FOO"])
	}
	if env["BAZ"] != "QUX" {
		t.Errorf("expected BAZ=QUX, got %s", env["BAZ"])
	}
	if env["CI"] != "true" {
		t.Errorf("expected CI=true, got %s", env["CI"])
	}
}

func TestAppPackTomlTestEnvEmpty(t *testing.T) {
	c := AppPackToml{}
	env := c.GetTestEnv()
	if len(env) != 1 {
		t.Errorf("expected 1 env vars, got %d", len(env))
	}
	if env["CI"] != "true" {
		t.Errorf("expected CI=true, got %s", env["CI"])
	}
}
