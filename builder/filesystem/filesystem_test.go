package filesystem

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/spf13/afero"
)

var testLogger = logging.NewSimpleLogger(os.Stdout)

func TestCreateIfNotExists(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	if _, err := fs.Stat("app.json"); !os.IsNotExist(err) {
		t.Error("app.json should not exist")
	}
	s := &FileState{
		fs:  fs,
		log: testLogger,
	}
	s.CreateIfNotExists()
	if _, err := fs.Stat("app.json"); os.IsNotExist(err) {
		t.Error("app.json should exist")
	}
}

func TestWriteSkipBuild(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	cbId := "codebuild-123"
	filename := skipBuildFilename(cbId)
	if _, err := fs.Stat(filename); !os.IsNotExist(err) {
		t.Error("skip file should not exist")
	}
	s := &FileState{
		fs:  fs,
		log: testLogger,
	}
	s.WriteSkipBuild(cbId)
	if _, err := fs.Stat(filename); os.IsNotExist(err) {
		t.Error("skip file should exist")
	}
}

func TestShouldSkipBuild(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	cbId := "codebuild-123"
	s := &FileState{
		fs:  fs,
		log: testLogger,
	}
	skip, err := s.ShouldSkipBuild(cbId)
	if err != nil {
		t.Error(err)
	}
	if skip {
		t.Error("should not skip build")
	}
	s.WriteSkipBuild(cbId)
	skip, err = s.ShouldSkipBuild(cbId)
	if err != nil {
		t.Error(err)
	}
	if !skip {
		t.Error("should skip build")
	}
}

func TestWriteEnvFile(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	s := &FileState{
		fs:  fs,
		log: testLogger,
	}
	env := map[string]string{
		"FOO": "bar",
		"BAR": "baz",
	}
	err := s.WriteEnvFile(&env)
	if err != nil {
		t.Error(err)
	}
	newEnv, err := s.ReadEnvFile()
	if err != nil {
		t.Error(err)
	}
	if (*newEnv)["FOO"] != "bar" {
		t.Error("env not written correctly")
	}
	if len(*newEnv) != 2 {
		t.Error("env does not have two items")
	}
}

func TestWriteMetadataToml(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	s := &FileState{
		fs:  fs,
		log: testLogger,
	}
	dummyText := "hello world"
	readClose := ioutil.NopCloser(strings.NewReader(dummyText))
	err := s.WriteMetadataToml(readClose)
	if err != nil {
		t.Error(err)
	}
	// read metadata.toml and assert it has the correct contents
	f, err := fs.Open("metadata.toml")
	if err != nil {
		t.Error(err)
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if string(b) != dummyText {
		t.Error("metadata.toml does not have the correct contents")
	}
}
