package build

import (
	"fmt"

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

func (b *Build) RunPostbuild() error {
	skipBuild, _ := b.state.ShouldSkipBuild(b.CodebuildBuildId)
	if skipBuild {
		b.Log.Info("Skipping test")
		return nil
	}
	testScript := b.AppJSON.TestScript()
	PrintStartMarker("test")
	defer PrintEndMarker("test")
	if testScript == "" {
		b.Log.Info("No tests defined in app.json")
		return nil
	}
	imageName, err := b.ImageName()
	if err != nil {
		return err
	}
	env, err := b.LoadTestEnv()
	if err != nil {
		return err
	}
	envStrings := []string{}
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("%s=%s", k, v))
	}
	containerID := fmt.Sprintf("test-%s", b.CodebuildBuildId)
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
	err = b.containers.AttachLogs(containerID)
	if err != nil {
		return err
	}
	// wait for container to finish
	exitCode, err := b.containers.WaitForExit(fmt.Sprintf("test-%s", b.CodebuildBuildId))
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("test failed with exit code %d", exitCode)
	}
	return nil

}
