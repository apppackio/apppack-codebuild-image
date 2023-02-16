package filesystem

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"
)

var testContext = zerolog.New(os.Stdout).With().Timestamp().Logger().WithContext(context.Background())

func TestCreateIfNotExists(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	if _, err := fs.Stat("apppack.toml"); !os.IsNotExist(err) {
		t.Error("apppack.toml should not exist")
	}
	s := &FileState{
		fs:  fs,
		ctx: testContext,
	}
	err := s.CreateIfNotExists()
	if err != nil {
		t.Error(err)
	}
	if _, err := fs.Stat("apppack.toml"); os.IsNotExist(err) {
		t.Error("apppack.toml should exist")
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
