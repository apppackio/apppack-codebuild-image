package build

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// LoadTestEnv uses the environment defined in app.json
// with overrides for any in-dyno services
func (b *Build) LoadTestEnv() (map[string]string, error) {
	env := b.AppJSON.GetTestEnv()
	envOverride, err := b.state.ReadEnvFile()
	if err != nil {
		return nil, err
	}
	for k, v := range *envOverride {
		env[k] = v
	}
	return env, nil
}

// generateDockerEnvStrings converts a map of env vars to a slice of k=v strings
func generateDockerEnvStrings(env map[string]string) []string {
	envStrings := []string{}
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("%s=%s", k, v))
	}
	return envStrings
}

// testLogWriters returns a writer that writes to stdout/err and a file
func testLogWriters(file *os.File) (io.Writer, io.Writer) {
	return io.MultiWriter(os.Stdout, file), io.MultiWriter(os.Stderr, file)
}

func (b *Build) RunPostbuild() error {
	skipBuild, _ := b.state.ShouldSkipBuild(b.CodebuildBuildId)
	if skipBuild {
		b.Log().Info().Msg("skipping test")
		return nil
	}
	testLogFile, err := b.state.CreateLogFile("test.log")
	if err != nil {
		return err
	}
	defer testLogFile.Close()
	writer, errWriter := testLogWriters(testLogFile)
	testScript := b.AppJSON.TestScript()
	PrintStartMarker("test")
	defer PrintEndMarker("test")
	if testScript == "" {
		_, err := writer.Write([]byte("no tests defined in app.json\n"))
		return err
	}
	_, err = writer.Write([]byte(fmt.Sprintf("+ %s\n", testScript)))
	if err != nil {
		return err
	}
	imageName, err := b.ImageName()
	if err != nil {
		return err
	}
	env, err := b.LoadTestEnv()
	if err != nil {
		return err
	}
	envStrings := generateDockerEnvStrings(env)
	containerID := strings.ReplaceAll(b.CodebuildBuildId, ":", "-")
	defer b.containers.Close()
	err = b.containers.RunContainer(containerID, b.CodebuildBuildId, &container.Config{
		Image:      imageName,
		Cmd:        []string{"/bin/sh", "-c", testScript},
		Entrypoint: []string{"/cnb/lifecycle/launcher"},
		Env:        envStrings,
	})
	if err != nil {
		return err
	}
	defer b.containers.DeleteContainer(containerID)
	err = b.containers.AttachLogs(containerID, writer, errWriter)
	if err != nil {
		return err
	}
	// wait for container to finish
	exitCode, err := b.containers.WaitForExit(containerID)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("test failed with exit code %d", exitCode)
	}
	return nil

}
