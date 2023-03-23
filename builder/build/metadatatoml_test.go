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
	services := m.ToApppackServices()
	if len(services) != 2 {
		t.Errorf("expected 2 services, got %d", len(services))
	}
	expected := "echo 'ruby web'"
	if services["web"].Command != expected {
		t.Errorf("expected %s, got %s", expected, services["web"].Command)
	}
	expected = "echo 'ruby worker'"
	if services["worker"].Command != expected {
		t.Errorf("expected %s, got %s", expected, services["worker"].Command)
	}
}
