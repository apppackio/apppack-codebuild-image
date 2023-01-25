package build

import "testing"

func TestAppPackTomlValidateBuildpackAndDockerfile(t *testing.T) {
	c := AppPackToml{
		Build: AppPackTomlBuild{
			Buildpacks: []string{"heroku/ruby"},
			Dockerfile: "Dockerfile",
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
		Services: map[string]AppPackTomlServices{"web": {Command: "echo hello"}},
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
			Dockerfile: "Dockerfile",
		},
		Services: map[string]AppPackTomlServices{"web": {Command: "echo hello"}},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}
