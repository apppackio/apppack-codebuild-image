package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	cp "github.com/otiai10/copy"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	CreateLogFile(string) (*os.File, error)
}

// State is a struct that holds the state of the build
type FileState struct {
	fs     afero.Afero
	execer func(name string, arg ...string) *exec.Cmd
	copier func(src, dst string, opt ...cp.Options) error
	ctx    context.Context
}

func New(ctx context.Context) *FileState {
	return &FileState{
		fs:     afero.Afero{Fs: afero.NewOsFs()},
		execer: exec.Command,
		copier: cp.Copy,
		ctx:    ctx,
	}
}

func (f *FileState) Log() *zerolog.Logger {
	return log.Ctx(f.ctx)
}

func (f *FileState) CreateIfNotExists() error {
	// touch files codebuild expects to exist
	for _, filename := range []string{"app.json", "build.log", "metadata.toml", "test.log"} {
		_, err := f.fs.Stat(filename)
		if !os.IsNotExist(err) {
			return err
		}
		f.Log().Debug().Str("filename", filename).Msg("touching file")
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
	f.Log().Debug().Str("filename", name).Msg("writing override env vars to file")
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
		return nil, err
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

// os.Rename doesn't work across filesystems, so we need to copy the file
func (f *FileState) CopyFile(src, dest string) error {
	stat, err := f.fs.Stat(src)
	if err != nil {
		return err
	}
	byteArr, err := f.fs.ReadFile(src)
	if err != nil {
		return err
	}
	return f.fs.WriteFile(dest, byteArr, stat.Mode())
}

// can't mock with afero because we need to pass it as an os.File to multiwriter
func (f *FileState) CreateLogFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
}
