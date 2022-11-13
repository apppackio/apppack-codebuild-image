package state

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"
)

type State interface {
	CreateIfNotExists() error
	WriteCommitTxt() error
	WriteSkipBuild(string) error
	ShouldSkipBuild(string) (bool, error)
	WriteMetadataToml(io.ReadCloser) error
}

// State is a struct that holds the state of the build
type FileState struct {
	// writer is a function that writes a file
	writer func(string, []byte, fs.FileMode) error
	// execer is a function that executes a command
	execer func(string, ...string) *exec.Cmd
	// stater is a function that stats a file
	stater func(string) (fs.FileInfo, error)
	// creator is a function that creates a file
	creator func(string) (*os.File, error)
}

func New() *FileState {
	return &FileState{
		writer: os.WriteFile,
		execer: exec.Command,
		stater: os.Stat,
		creator: os.Create,
	}
}

func (f *FileState) CreateIfNotExists() error {
	// touch files codebuild expects to exist
	for _, filename := range []string{"app.json", "build.log", "metadata.toml", "test.log"} {
		_, err := f.stater(filename)
		if !os.IsNotExist(err) {
			return err
		}
		log.Debug().Msg(fmt.Sprintf("touching %s", filename))
		err = f.writer(filename, []byte{}, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteCommitTxt shells out to `git log -n1 --decorate=no` and writes stdout to commit.txt
func (f *FileState) WriteCommitTxt() error {
	log.Debug().Msg("fetching git log")
	cmd, err := f.execer("git", "log", "-n1", "--decorate=no").Output()
	if err != nil {
		return err
	}
	// write the output of the command to commit.txt
	log.Debug().Msg("writing commit.txt")
	return f.writer("commit.txt", cmd, 0644)
}

func skipBuildFilename(id string) string {
	return fmt.Sprintf(".apppack-skip-build-%s", id)
}

func (f *FileState) WriteSkipBuild(id string) error {
	return f.writer(skipBuildFilename((id)), []byte{}, 0644)
}

func (f *FileState) ShouldSkipBuild(id string) (bool, error) {
	_, err := f.stater(skipBuildFilename(id))
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (f *FileState) WriteMetadataToml(reader io.ReadCloser) error {
	defer reader.Close()
	dest := "metadata.toml"
	file, err := f.creator(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return nil
}
