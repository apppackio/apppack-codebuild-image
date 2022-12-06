package build

import (
	"os"
	"testing"

	"github.com/buildpacks/pack/pkg/logging"
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

func TestAppJsonBuildpackPatch(t *testing.T) {
	a := AppJSON{
		Buildpacks: []Buildpack{
			{URL: "heroku/nodejs"},
			{URL: "heroku/python"},
		},
		logger: logging.NewSimpleLogger(os.Stderr),
	}
	expected := []string{"urn:cnb:registry:heroku/nodejs", "heroku/python"}
	if !stringSliceEqual(a.GetBuildpacks(), expected) {
		t.Errorf("expected %v, got %v", expected, a.GetBuildpacks())
	}
}

func TestAppJsonMissing(t *testing.T) {
	a := AppJSON{
		reader: func() ([]byte, error) {
			return nil, os.ErrNotExist
		},
		logger: logging.NewSimpleLogger(os.Stderr),
	}
	err := a.Unmarshal()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
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
		logger: logging.NewSimpleLogger(os.Stderr),
	}
	err := a.Unmarshal()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if a.Stack != "heroku-18" {
		t.Errorf("expected heroku-22, got %s", a.Stack)
	}
}

func TestAppJsonBuilders(t *testing.T) {
	a := AppJSON{
		Stack:  "heroku-22",
		logger: logging.NewSimpleLogger(os.Stderr),
	}
	expected := []string{"heroku/builder-classic:22", "heroku/heroku:22-cnb"}
	if !stringSliceEqual(a.GetBuilders(), expected) {
		t.Errorf("expected %v, got %v", expected, a.GetBuilders())
	}
}
