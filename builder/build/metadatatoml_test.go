package build

import (
	"testing"
)

func TestBuildpackMetadataTomlToApppackServices(t *testing.T) {
	m := BuildpackMetadataToml{
		Processes: []BuildpackMetadataTomlProcess{
			{
				Command:     []string{"echo", "ruby web"},
				Type:        "web",
				BuildpackID: "heroku/ruby",
			},
			{
				Command:     []string{"echo", "release"},
				Type:        "release",
				BuildpackID: "heroku/ruby",
			},
			{
				Command:     []string{"echo 'ruby worker'"},
				Type:        "worker",
				BuildpackID: "heroku/ruby",
			},
			{
				Command:     []string{"echo", "rake"},
				Type:        "rake",
				BuildpackID: "heroku/ruby",
			},
		},
	}
	a := AppPackToml{}
	m.UpdateAppPackToml(&a)
	if len(a.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(a.Services))
	}
	expected := "echo 'ruby web'"
	if a.Services["web"].Command != expected {
		t.Errorf("expected %s, got %s", expected, a.Services["web"].Command)
	}
	expected = "echo 'ruby worker'"
	if a.Services["worker"].Command != expected {
		t.Errorf("expected %s, got %s", expected, a.Services["worker"].Command)
	}
	expected = "echo release"
	if a.Deploy.ReleaseCommand != expected {
		t.Errorf("expected %s, got %s", expected, a.Deploy.ReleaseCommand)
	}
}
