package filesystem

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
	cp "github.com/otiai10/copy"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

const envFileFilename = "env.json"

type State interface {
	CreateIfNotExists() error
	FileExists(string) (bool, error)
	WriteSkipBuild(string) error
	ShouldSkipBuild(string) (bool, error)
	WriteEnvFile(*map[string]string) error
	ReadEnvFile() (*map[string]string, error)
	UnpackTarArchive(io.ReadCloser) error
	WriteCommitTxt() error
	MvGitDir() error
	GitSha() (string, error)
	EndLogging(*os.File, string) error
	WriteTomlToFile(string, interface{}) error
	WriteJsonToFile(string, interface{}) error
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
	for _, filename := range []string{"apppack.toml", "build.log", "metadata.toml", "test.log"} {
		exists, err := f.FileExists(filename)
		if err != nil {
			return err
		}
		if !exists {
			f.Log().Debug().Str("filename", filename).Msg("touching file")
			err = f.fs.WriteFile(filename, []byte{}, 0o644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *FileState) FileExists(filename string) (bool, error) {
	_, err := f.fs.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func skipBuildFilename(id string) string {
	return fmt.Sprintf(".apppack-skip-build-%s", id)
}

func (f *FileState) WriteSkipBuild(id string) error {
	return f.fs.WriteFile(skipBuildFilename((id)), []byte{}, 0o644)
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
	return f.WriteJsonToFile(name, env)
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

func (f *FileState) UnpackTarArchive(reader io.ReadCloser) error {
	tr := tar.NewReader(reader)

	// Iterate over the entries in the tar archive
	for {
		// Read the next entry
		hdr, err := tr.Next()
		if err == io.EOF {
			// End of tar archive
			break
		}
		if err != nil {
			return err
		}

		// Write the entry to disk
		f, err := f.fs.Create(hdr.Name)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(f, tr); err != nil {
			return err
		}
	}
	return nil
}

// EndLogging closes the file and copies it to the destination
func (f *FileState) EndLogging(src *os.File, dst string) error {
	src.Close()
	return f.copier(src.Name(), dst)
}

func (f *FileState) WriteTomlToFile(filename string, v interface{}) error {
	f.Log().Debug().Str("filename", filename).Msg("writing toml to file")
	file, err := f.fs.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := toml.NewEncoder(file).Encode(v); err != nil {
		return err
	}
	return nil
}

func (f *FileState) WriteJsonToFile(filename string, v interface{}) error {
	f.Log().Debug().Str("filename", filename).Msg("writing json to file")
	file, err := f.fs.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(v); err != nil {
		return err
	}
	return nil
}
