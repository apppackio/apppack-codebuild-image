package filesystem

import (
	"fmt"
	"regexp"
	"strings"
)

// GitSha returns the git hash of the current commit
func (f *FileState) GitSha() (string, error) {
	f.Log().Debug().Msg("fetching git sha")
	cmd, err := f.execer("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(cmd)), nil
}

// WriteCommitTxt shells out to `git log -n1 --decorate=no` and writes stdout to commit.txt
func (f *FileState) WriteCommitTxt() error {
	f.Log().Debug().Msg("fetching git log")
	cmd, err := f.execer("git", "log", "-n1", "--decorate=no").Output()
	if err != nil {
		return err
	}
	// write the output of the command to commit.txt
	f.Log().Debug().Msg("writing commit.txt")
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
	// can't use os.Rename because they are on different filesystems
	return f.copier(string(matches[1]), ".git")
}
