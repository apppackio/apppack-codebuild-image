package filesystem

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
	"github.com/spf13/afero"
)

var envFileFilename = "env.json"

type State interface {
	CreateIfNotExists() error
	WriteSkipBuild(string) error
	ShouldSkipBuild(string) (bool, error)
	WriteEnvFile(*map[string]string) error
	ReadEnvFile() (*map[string]string, error)
	WriteMetadataToml(io.ReadCloser) error
	WriteCommitTxt() error
	MvGitDir() error
	GitSha() (string, error)
}

// State is a struct that holds the state of the build
type FileState struct {
	fs     afero.Afero
	execer func(name string, arg ...string) *exec.Cmd
	log    logging.Logger
}

func New() *FileState {
	return &FileState{
		fs:     afero.Afero{Fs: afero.NewOsFs()},
		execer: exec.Command,
		log:    logging.NewLogWithWriters(color.Stdout(), color.Stderr()),
	}
}

func (f *FileState) CreateIfNotExists() error {
	// touch files codebuild expects to exist
	for _, filename := range []string{"app.json", "build.log", "metadata.toml", "test.log"} {
		_, err := f.fs.Stat(filename)
		if !os.IsNotExist(err) {
			return err
		}
		f.log.Debugf("touching %s", filename)
		err = f.fs.WriteFile(filename, []byte{}, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func skipBuildFilename(id string) string {
	return fmt.Sprintf(".apppack-skip-build-%s", id)
}

func (f *FileState) WriteSkipBuild(id string) error {
	return f.fs.WriteFile(skipBuildFilename((id)), []byte{}, 0644)
}

func (f *FileState) ShouldSkipBuild(id string) (bool, error) {
	_, err := f.fs.Stat(skipBuildFilename(id))
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (f *FileState) WriteEnvFile(env *map[string]string) error {
	name := filepath.Join(os.TempDir(), envFileFilename)
	file, err := f.fs.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(env); err != nil {
		return err
	}
	return nil
}

func (f *FileState) ReadEnvFile() (*map[string]string, error) {
	name := filepath.Join(os.TempDir(), envFileFilename)
	env := map[string]string{}
	file, err := f.fs.Open(name)
	if err != nil {
		return &env, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&env); err != nil {
		return nil, err
	}
	return &env, nil
}

func (f *FileState) WriteMetadataToml(reader io.ReadCloser) error {
	defer reader.Close()
	dest := "metadata.toml"
	file, err := f.fs.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return nil
}
