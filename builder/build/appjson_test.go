package build

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

var testContext = zerolog.New(os.Stdout).With().Timestamp().Logger().WithContext(context.Background())

func TestAppJsonBuildpackPatch(t *testing.T) {
	a := AppJSON{
		Buildpacks: []Buildpack{
			{URL: "heroku/nodejs"},
			{URL: "heroku/python"},
		},
		Stack: "heroku-20",
		ctx:   log.With().Logger().WithContext(context.Background()),
	}
	expected := []string{"urn:cnb:builder:heroku/nodejs", "urn:cnb:builder:heroku/python"}
	if !stringSliceEqual(a.GetBuildpacks(), expected) {
		t.Errorf("expected %s, got %s", expected, a.GetBuildpacks())
	}
}

func TestAppJsonMissing(t *testing.T) {
	a := AppJSON{
		reader: func() ([]byte, error) {
			return nil, os.ErrNotExist
		},
		ctx: testContext,
	}
	err := a.Unmarshal()
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if a.Stack != DefaultStack {
		t.Errorf("expected %s, got %s", DefaultStack, a.Stack)
	}
}

func TestAppJsonStack(t *testing.T) {
	a := AppJSON{
		reader: func() ([]byte, error) {
			return []byte(`{"stack": "heroku-18"}`), nil
		},
		ctx: testContext,
	}
	err := a.Unmarshal()
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if a.Stack != "heroku-18" {
		t.Errorf("expected heroku-22, got %s", a.Stack)
	}
}

func TestAppJsonBuilders(t *testing.T) {
	a := AppJSON{
		Stack: "heroku-22",
		ctx:   testContext,
	}
	expected := []string{"heroku/builder-classic:22", "heroku/heroku:22-cnb"}
	if !stringSliceEqual(a.GetBuilders(), expected) {
		t.Errorf("expected %s, got %s", expected, a.GetBuilders())
	}
}

func TestAppJsonTestScript(t *testing.T) {
	testScript := "echo test"
	a := AppJSON{
		Environments: map[string]Environment{
			"test": {
				Scripts: map[string]string{
					"test": testScript,
				},
			},
		},
		ctx: testContext,
	}
	actual := a.TestScript()
	if actual != testScript {
		t.Errorf("expected %s, got %s", testScript, actual)
	}
}

func TestAppJsonTestScriptMissing(t *testing.T) {
	a := AppJSON{}
	actual := a.TestScript()
	if actual != "" {
		t.Errorf("expected '', got %s", actual)
	}
}

func TestAppJsonGetTestEnv(t *testing.T) {
	a := AppJSON{
		Environments: map[string]Environment{
			"test": {
				Env: map[string]string{
					"FOO": "BAR",
					"BAZ": "QUX",
				},
			},
		},
	}

	env := a.GetTestEnv()
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

func TestAppJsonToApppackToml(t *testing.T) {
	a := AppJSON{
		Stack: "heroku-22",
		Environments: map[string]Environment{
			"test": {
				Scripts: map[string]string{
					"test": "echo test",
				},
				Env: map[string]string{
					"FOO": "BAR",
				},
			},
		},
		Buildpacks: []Buildpack{
			{URL: "heroku/nodejs"},
			{URL: "heroku/python"},
		},
		ctx: testContext,
	}

	expected := AppPackToml{
		Build: AppPackTomlBuild{
			System:     "buildpack",
			Buildpacks: []string{"urn:cnb:builder:heroku/nodejs", "urn:cnb:builder:heroku/python"},
			Builder:    "heroku/builder-classic:22",
		},
		Test: AppPackTomlTest{
			Command: "echo test",
			Env: []string{
				"FOO=BAR",
			},
		},
	}
	actual := a.ToApppackToml()
	if !reflect.DeepEqual(expected.Build, actual.Build) {
		t.Errorf("expected %s, got %s", expected.Build, actual.Build)
	}
	if !reflect.DeepEqual(expected.Test, actual.Test) {
		t.Errorf("expected %s, got %s", expected.Test, actual.Test)
	}
}
