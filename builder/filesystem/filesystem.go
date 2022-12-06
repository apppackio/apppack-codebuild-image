package filesystem

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
	"github.com/spf13/afero"
)

type State interface {
	CreateIfNotExists() error
	WriteSkipBuild(string) error
	ShouldSkipBuild(string) (bool, error)
	WriteMetadataToml(io.ReadCloser) error
	WriteCommitTxt() error
	MvGitDir() error
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

// WriteCommitTxt shells out to `git log -n1 --decorate=no` and writes stdout to commit.txt
func (f *FileState) WriteCommitTxt() error {
	f.log.Debug("fetching git log")
	cmd, err := f.execer("git", "log", "-n1", "--decorate=no").Output()
	if err != nil {
		return err
	}
	// write the output of the command to commit.txt
	f.log.Debug("writing commit.txt")
	return f.fs.WriteFile("commit.txt", cmd, 0644)
}

// MvGitDir moves the git directory to the root of the project
// Codebuild has a .git file that points to the real git directory
func (f *FileState) MvGitDir() error {
	// test if .git is a file
	fileInfo, err := f.fs.Stat(".git")
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return nil
	}
	// read the contents of .git
	gitFile, err := f.fs.ReadFile(".git")
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`^gitdir:\s*(.*)$`)
	matches := re.FindSubmatch(gitFile)
	if len(matches) != 2 {
		return fmt.Errorf("failed to parse .git file")
	}
	// delete the .git file
	err = f.fs.Remove(".git")
	if err != nil {
		return err
	}
	// move the git directory to the root of the project
	return f.fs.Rename(string(matches[1]), ".git")
}
