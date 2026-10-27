package build

import (
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func stringSliceEqual(a, b []string) bool {
	return slices.Equal(a, b)
}

var testContext = zerolog.New(os.Stdout).With().Timestamp().Logger().WithContext(context.Background())

var testLogger = zerolog.New(os.Stdout).With().Timestamp().Logger()

func TestAppJsonMissing(t *testing.T) {
	a := AppJSON{
		reader: func() ([]byte, error) {
			return nil, os.ErrNotExist
		},
		logger: &testLogger,
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
			return []byte(`{"stack": "heroku-22"}`), nil
		},
		logger: &testLogger,
	}
	err := a.Unmarshal()
	if err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if a.Stack != "heroku-22" {
		t.Errorf("expected heroku-22, got %s", a.Stack)
	}
}

func TestAppJsonBuilders(t *testing.T) {
	tests := []struct {
		stack    string
		expected []string
	}{
		{"heroku-22", []string{"heroku/builder:22", "heroku/heroku:22-cnb"}},
		{"heroku-24", []string{"heroku/builder:24", "heroku/heroku:24"}},
		{"heroku-26", []string{"heroku/builder:26", "heroku/heroku:26"}},
		{"custom/builder:latest", []string{"custom/builder:latest"}},
	}
	for _, tt := range tests {
		a := AppJSON{Stack: tt.stack, logger: &testLogger}
		if !stringSliceEqual(a.GetBuilders(), tt.expected) {
			t.Errorf("stack %s: expected %s, got %s", tt.stack, tt.expected, a.GetBuilders())
		}
	}
}

func TestAppJsonEOLStackError(t *testing.T) {
	for _, stack := range EOLStacks {
		a := AppJSON{
			reader: func() ([]byte, error) {
				return []byte(`{"stack": "` + stack + `"}`), nil
			},
			logger: &testLogger,
		}
		err := a.Unmarshal()
		if err == nil {
			t.Errorf("stack %s: expected error for EOL stack, got nil", stack)
		} else if !strings.Contains(err.Error(), "end-of-life") {
			t.Errorf("stack %s: expected end-of-life error, got: %s", stack, err)
		}
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
		logger: &testLogger,
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
		logger: &testLogger,
	}

	expected := AppPackToml{
		Build: AppPackTomlBuild{
			System:     "buildpack",
			Buildpacks: []string{"urn:cnb:builder:heroku/nodejs", "urn:cnb:builder:heroku/python"},
			Builder:    "heroku/builder:22",
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

func TestAppJsonGetBuildersDoesNotExposeMap(t *testing.T) {
	a := AppJSON{Stack: "heroku-24", logger: &testLogger}
	a.GetBuilders()[0] = "mutated"
	if got := a.GetBuilders()[0]; got != "heroku/builder:24" {
		t.Errorf("expected heroku/builder:24, got %s", got)
	}
}

func TestAppJsonPatchIncludedBuildpacks(t *testing.T) {
	for _, stack := range []string{"heroku-22", "heroku-24", "heroku-26"} {
		t.Run(stack, func(t *testing.T) {
			a := AppJSON{
				Stack: stack,
				Buildpacks: []Buildpack{
					{URL: "heroku/python"},
					{URL: "heroku/procfile"},
					{URL: "https://github.com/example/custom-buildpack"},
				},
				logger: &testLogger,
			}
			expected := []string{
				"urn:cnb:builder:heroku/python",
				"urn:cnb:builder:heroku/procfile",
				"https://github.com/example/custom-buildpack",
			}
			if got := a.GetBuildpacks(); !stringSliceEqual(got, expected) {
				t.Errorf("expected %s, got %s", expected, got)
			}
		})
	}
}

func TestAppJsonUnknownStackBuildpacksUnpatched(t *testing.T) {
	a := AppJSON{Stack: "custom/builder:latest", Buildpacks: []Buildpack{{URL: "heroku/python"}}}
	if got := a.GetBuildpacks(); !stringSliceEqual(got, []string{"heroku/python"}) {
		t.Errorf("expected unpatched buildpack, got %s", got)
	}
}
