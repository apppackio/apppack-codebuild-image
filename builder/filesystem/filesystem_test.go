package filesystem

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"
)

var testContext = zerolog.New(os.Stdout).With().Timestamp().Logger().WithContext(context.Background())

func TestCreateIfNotExists(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}

	filename := GetAppPackTomlFilename()

	if _, err := fs.Stat(filename); !os.IsNotExist(err) {
		t.Error(fmt.Sprintf("%s should not exist", filename))
	}
	s := &FileState{
		fs:  fs,
		ctx: testContext,
	}
	err := s.CreateIfNotExists()
	if err != nil {
		t.Error(err)
	}
	if _, err := fs.Stat(filename); os.IsNotExist(err) {
		t.Error(fmt.Sprintf("%s should exist", filename))
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
		ctx: testContext,
	}
	err := s.WriteSkipBuild(cbId)
	if err != nil {
		t.Error(err)
	}
	if _, err := fs.Stat(filename); os.IsNotExist(err) {
		t.Error("skip file should exist")
	}
}

func TestShouldSkipBuild(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	cbId := "codebuild-123"
	s := &FileState{
		fs:  fs,
		ctx: testContext,
	}
	skip, err := s.ShouldSkipBuild(cbId)
	if err != nil {
		t.Error(err)
	}
	if skip {
		t.Error("should not skip build")
	}
	err = s.WriteSkipBuild(cbId)
	if err != nil {
		t.Error(err)
	}
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
		ctx: testContext,
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

func TestGetFilename(t *testing.T) {
	// Check that there is no env variable set
	if os.Getenv("APPPACK_TOML") != "" {
		t.Error("APPPACK_TOML env variable should not be set")
	}
	// Call GetAppPackTomlFilename and check the default value

	filename := GetAppPackTomlFilename()
	if filename != DefaultAppPackTomlFilename {
		t.Errorf("expected apppack.toml, got %s", filename)
	}

	// Set the env variable and check again
	os.Setenv("APPPACK_TOML", "custom.toml")
	defer os.Unsetenv("APPPACK_TOML") // Clean up after the test
	filename = GetAppPackTomlFilename()
	if filename != "custom.toml" {
		t.Errorf("expected custom.toml, got %s", filename)
	}
}

func TestCopyAppPackTomlToDefault(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		setupFiles    map[string]string
		expectCopy    bool
		expectError   bool
		errorContains string
	}{
		{
			name:       "no copy needed when using default location",
			envValue:   "",
			expectCopy: false,
		},
		{
			name:     "copies from custom location",
			envValue: "config/custom.toml",
			setupFiles: map[string]string{
				"config/custom.toml": "[build]\ntest = true\n",
			},
			expectCopy: true,
		},
		{
			name:          "error when custom file doesn't exist",
			envValue:      "nonexistent.toml",
			expectCopy:    false,
			expectError:   true,
			errorContains: "failed to copy nonexistent.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("APPPACK_TOML", tt.envValue)
				defer os.Unsetenv("APPPACK_TOML")
			}

			// Create temp directory for test
			tempDir := t.TempDir()
			originalDir, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(originalDir)

			// Set up test files
			for path, content := range tt.setupFiles {
				dir := filepath.Dir(path)
				if dir != "." {
					os.MkdirAll(dir, 0o755)
				}
				os.WriteFile(path, []byte(content), 0o644)
			}

			// Run the function
			err := CopyAppPackTomlToDefault()

			// Check error
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check if file was copied
			if tt.expectCopy {
				if _, err := os.Stat(DefaultAppPackTomlFilename); os.IsNotExist(err) {
					t.Errorf("expected %s to exist after copy", DefaultAppPackTomlFilename)
				} else {
					// Verify content matches
					expected := tt.setupFiles[tt.envValue]
					actual, _ := os.ReadFile(DefaultAppPackTomlFilename)
					if string(actual) != expected {
						t.Errorf("copied content mismatch: got %q, want %q", actual, expected)
					}
				}
			}
		})
	}
}

func dummyTarBuffer() (*io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dummyText := "hello world"
	err := tw.WriteHeader(&tar.Header{
		Name: "metadata.toml",
		Size: int64(len(dummyText)),
	})
	if err != nil {
		return nil, err
	}
	_, err = tw.Write([]byte(dummyText))
	if err != nil {
		return nil, err
	}
	tw.Close()
	ir := io.Reader(bytes.NewReader(buf.Bytes()))
	return &ir, nil
}

func TestWriteMetadataToml(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	s := &FileState{
		fs:  fs,
		ctx: testContext,
	}
	reader, err := dummyTarBuffer()
	if err != nil {
		t.Error(err)
	}
	readClose := io.NopCloser(*reader)
	err = s.UnpackTarArchive(readClose)
	if err != nil {
		t.Error(err)
	}
	// read metadata.toml and assert it has the correct contents
	f, err := fs.Open("metadata.toml")
	if err != nil {
		t.Error(err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if string(b) != "hello world" {
		t.Error("metadata.toml does not have the correct contents")
	}
}

func TestWriteTomlToFile(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	s := &FileState{
		fs:  fs,
		ctx: testContext,
	}
	err := s.WriteTomlToFile("test.toml", map[string]string{"foo": "bar"})
	if err != nil {
		t.Error(err)
	}
	f, err := fs.Open("test.toml")
	if err != nil {
		t.Error(err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	actual := string(b)
	expected := "foo = \"bar\"\n"
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}
