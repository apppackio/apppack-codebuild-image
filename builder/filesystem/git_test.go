package filesystem

import (
	"context"
	"os/exec"
	"testing"

	cp "github.com/otiai10/copy"
	"github.com/spf13/afero"
)

func TestGitSha(t *testing.T) {
	testText := "hello world"
	fs := &FileState{
		fs:  afero.Afero{Fs: afero.NewMemMapFs()},
		ctx: testContext,
		execer: func(name string, arg ...string) *exec.Cmd {
			return exec.Command("echo", testText)
		},
	}
	sha, err := fs.GitSha()
	if err != nil {
		t.Error(err)
	}
	if sha != testText {
		t.Error("wrong sha")
	}
}

func TestWriteCommitTxt(t *testing.T) {
	testText := "dummy commit log"
	f := &FileState{
		fs:  afero.Afero{Fs: afero.NewMemMapFs()},
		ctx: testContext,
		execer: func(name string, arg ...string) *exec.Cmd {
			return exec.Command("echo", testText)
		},
	}
	err := f.WriteCommitTxt()
	if err != nil {
		t.Errorf("WriteCommitTxt returned error: %s", err)
	}

	// check that the commit.txt file was created and has the expected content
	exists, err := f.fs.Exists("commit.txt")
	if err != nil {
		t.Errorf("Error checking for commit.txt file: %s", err)
	}
	if !exists {
		t.Error("commit.txt file was not created")
	}
	content, err := f.fs.ReadFile("commit.txt")
	if err != nil {
		t.Errorf("Error reading commit.txt file: %s", err)
	}
	if string(content) != testText+"\n" {
		t.Errorf("Unexpected content in commit.txt file. Expected: %s, got: '%s'", testText, string(content))
	}
}
func TestMvGitDirNoOp(t *testing.T) {
	// setup mock filesystem
	mockFs := afero.Afero{
		Fs: afero.NewMemMapFs(),
	}

	// create FileState with mock filesystem
	f := &FileState{
		fs:  mockFs,
		ctx: testContext,
	}

	// test if function returns error when .git file is not found
	err := f.MvGitDir()
	if err == nil {
		t.Error("Expected error when .git file is not found")
	}

	// create .git file as directory
	err = mockFs.Mkdir(".git", 0755)
	if err != nil {
		t.Error("Failed to create .git directory")
	}

	// test if function returns nil when .git is a directory
	err = f.MvGitDir()
	if err != nil {
		t.Error("Expected nil when .git is a directory")
	}
	// check if .git directory still exists
	stat, err := mockFs.Stat(".git")
	if err != nil {
		t.Error("Expected .git directory to exist")
	}
	// check if .git is a directory
	if !stat.IsDir() {
		t.Error("Expected git directory to be moved to root of project")
	}
}

func TestMvGitDir(t *testing.T) {
	// setup mock filesystem
	mockFs := afero.Afero{
		Fs: afero.NewMemMapFs(),
	}

	var mockedSrc string
	var mockedDst string

	// create FileState with mock filesystem
	f := &FileState{
		fs:  mockFs,
		ctx: context.Background(),
		copier: func(src, dst string, opts ...cp.Options) error {
			mockedSrc = src
			mockedDst = dst
			return nil
		},
	}

	gitDir := "/path/to/git/dir"
	err := mockFs.Mkdir(gitDir, 0755)
	if err != nil {
		t.Error("Failed to create git directory")
	}

	// create .git file with correct format
	err = mockFs.WriteFile(".git", []byte("gitdir: "+gitDir), 0644)
	if err != nil {
		t.Error("Failed to create .git file with correct format")
	}

	// test if function successfully parses and renames git directory
	err = f.MvGitDir()
	if err != nil {
		t.Error("Expected nil when git directory is successfully parsed and renamed")
	}

	expectedDst := ".git"
	// check if .git directory exists
	if mockedSrc != gitDir {
		t.Errorf("Expected source to be %s", gitDir)
	}
	if mockedDst != expectedDst {
		t.Errorf("Expected destination to be %s", expectedDst)
	}
}

func TestMvGitDirFileInvalid(t *testing.T) {
	// setup mock filesystem
	mockFs := afero.Afero{
		Fs: afero.NewMemMapFs(),
	}

	// create FileState with mock filesystem
	f := &FileState{
		fs:  mockFs,
		ctx: testContext,
	}

	// create .git file with incorrect format
	err := mockFs.WriteFile(".git", []byte("incorrect format"), 0644)
	if err != nil {
		t.Error("Failed to create .git file with incorrect format")
	}

	// test if function returns error when .git file is not in correct format
	err = f.MvGitDir()
	if err == nil {
		t.Error("Expected error when .git file is not in correct format")
	}
}
