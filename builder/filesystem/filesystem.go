package filesystem

import (
	"archive/tar"
	"encoding/json"
	"errors"
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

const (
	envFileFilename            = "env.json"
	DefaultAppPackTomlFilename = "apppack.toml"
)

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
	logger *zerolog.Logger
}

func New(logger *zerolog.Logger) *FileState {
	return &FileState{
		fs:     afero.Afero{Fs: afero.NewOsFs()},
		execer: exec.Command,
		copier: cp.Copy,
		logger: logger,
	}
}

func (f *FileState) Log() *zerolog.Logger {
	return f.logger
}

func (f *FileState) CreateIfNotExists() error {
	// touch files codebuild expects to exist
	apppackToml := GetAppPackTomlFilename()

	for _, filename := range []string{apppackToml, "build.log", "metadata.toml", "test.log"} {
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
	return ".apppack-skip-build-" + id
}

func (f *FileState) WriteSkipBuild(id string) error {
	return f.fs.WriteFile(skipBuildFilename(id), []byte{}, 0o644)
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
	defer func() { _ = file.Close() }()
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
		if errors.Is(err, io.EOF) {
			// End of tar archive
			break
		}
		if err != nil {
			return err
		}

		// Write the entry to disk
		if err := f.writeTarEntry(hdr.Name, tr); err != nil {
			return err
		}
	}
	return nil
}

// writeTarEntry copies one tar entry to disk. It exists so the file is
// closed before the next entry is read: deferring the close inside the
// loop held every handle open until the whole archive had been unpacked.
func (f *FileState) writeTarEntry(name string, r io.Reader) (err error) {
	file, err := f.fs.Create(name)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(file, r)
	return err
}

// EndLogging closes the file and copies it to the destination
func (f *FileState) EndLogging(src *os.File, dst string) error {
	if err := src.Close(); err != nil {
		return err
	}
	return f.copier(src.Name(), dst)
}

func (f *FileState) WriteTomlToFile(filename string, v interface{}) (err error) {
	f.Log().Debug().Str("filename", filename).Msg("writing toml to file")
	file, err := f.fs.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return toml.NewEncoder(file).Encode(v)
}

func (f *FileState) WriteJsonToFile(filename string, v interface{}) (err error) {
	f.Log().Debug().Str("filename", filename).Msg("writing json to file")
	file, err := f.fs.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return json.NewEncoder(file).Encode(v)
}

func GetAppPackTomlFilename() string {
	filename := DefaultAppPackTomlFilename
	if envFile := os.Getenv("APPPACK_TOML"); envFile != "" {
		filename = envFile
	}
	return filename
}

// CopyAppPackTomlToDefault copies the apppack.toml file from a custom location to the default location
// This is needed for artifact archival when APPPACK_TOML env var is used to specify a custom location
// Returns nil if the file is already at the default location or if copy succeeds
func CopyAppPackTomlToDefault() error {
	apppackTomlPath := GetAppPackTomlFilename()
	if apppackTomlPath == DefaultAppPackTomlFilename {
		// File is already at the default location, nothing to do
		return nil
	}

	log.Debug().Msgf("Copying %s to %s for artifact archival", apppackTomlPath, DefaultAppPackTomlFilename)
	if err := cp.Copy(apppackTomlPath, DefaultAppPackTomlFilename); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", apppackTomlPath, DefaultAppPackTomlFilename, err)
	}
	return nil
}
