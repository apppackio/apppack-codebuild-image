package build

import (
	"testing"
)

func TestBuildpackMetadataTomlToApppackServices(t *testing.T) {
	m := BuildpackMetadataToml{
		Processes: []BuildpackMetadataTomlProcess{
			{
				Command:     []string{"bash", "-c"},
				Args:        []string{"echo \"ruby web\""},
				Type:        "web",
				BuildpackID: "heroku/ruby",
			},
			{
				Command:     []string{"bash", "-c"},
				Args:        []string{"echo release"},
				Type:        "release",
				BuildpackID: "heroku/ruby",
			},
			{
				Command:     []string{"bash", "-c"},
				Args:        []string{"echo 'ruby worker'"},
				Type:        "worker",
				BuildpackID: "heroku/ruby",
			},
			{
				Command:     []string{"bash", "-c"},
				Args:        []string{"echo rake"},
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
	expected := "bash -c 'echo \"ruby web\"'"
	if a.Services["web"].Command != expected {
		t.Errorf("expected %s, got %s", expected, a.Services["web"].Command)
	}
	expected = "bash -c 'echo '\"'\"'ruby worker'\"'\"''"
	if a.Services["worker"].Command != expected {
		t.Errorf("expected %s, got %s", expected, a.Services["worker"].Command)
	}
	expected = "bash -c 'echo release'"
	if a.Deploy.ReleaseCommand != expected {
		t.Errorf("expected %s, got %s", expected, a.Deploy.ReleaseCommand)
	}
}
