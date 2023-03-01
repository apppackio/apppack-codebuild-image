package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/apppackio/codebuild-image/builder/aws"
	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/apppackio/codebuild-image/builder/filesystem"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/docker/docker/api/types/container"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// define a struct named Build
type Build struct {
	Appname                string
	ArtifactBucket         string
	Branch                 string
	CodebuildBuildId       string
	CodebuildWebhookEvent  string
	CodebuildBuildNumber   string
	CodebuildSourceVersion string
	DockerHubUsername      string
	DockerHubAccessToken   string
	ECRRepo                string
	Pipeline               bool
	CreateReviewApp        bool
	AppJSON                *AppJSON
	AppPackToml            *AppPackToml
	Ctx                    context.Context
	aws                    aws.AWSInterface
	state                  filesystem.State
	containers             containers.ContainersI
}

type PRStatus struct {
	PullRequest string `json:"pull_request"`
	Status      string `json:"status"`
}

func GetenvFallback(envVars []string) string {
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			return os.Getenv(envVar)
		}
	}
	return ""
}

func (b *Build) FinishBuild() error {
	err := b.state.WriteCommitTxt()
	if err != nil {
		return err
	}
	return b.state.CreateIfNotExists()
}

func (b *Build) SkipBuild() error {
	err := b.FinishBuild()
	if err != nil {
		return err
	}
	return b.state.WriteSkipBuild(b.CodebuildBuildId)
}

func New(ctx context.Context) (*Build, error) {
	build := Build{
		Appname:                os.Getenv("APPNAME"),
		ArtifactBucket:         os.Getenv("ARTIFACT_BUCKET"),
		Branch:                 GetenvFallback([]string{"BRANCH", "CODEBUILD_WEBHOOK_HEAD_REF", "CODEBUILD_SOURCE_VERSION"}),
		CodebuildBuildId:       os.Getenv("CODEBUILD_BUILD_ID"),
		CodebuildBuildNumber:   os.Getenv("CODEBUILD_BUILD_NUMBER"),
		CodebuildWebhookEvent:  GetenvFallback([]string{"CODEBUILD_WEBHOOK_EVENT", "PULL_REQUEST_UPDATED"}),
		CodebuildSourceVersion: os.Getenv("CODEBUILD_SOURCE_VERSION"),
		DockerHubUsername:      os.Getenv("DOCKERHUB_USERNAME"),
		DockerHubAccessToken:   os.Getenv("DOCKERHUB_ACCESS_TOKEN"),
		ECRRepo:                os.Getenv("DOCKER_REPO"),
		Pipeline:               os.Getenv("PIPELINE") == "1",
		// REVIEW_APP_STATUS is set by the CLI when a review app is created
		CreateReviewApp: os.Getenv("REVIEW_APP_STATUS") == "created",
		Ctx:             ctx,
		state:           filesystem.New(ctx),
	}
	// the errors below return the build object so that the caller
	// can run the SkipBuild method
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return &build, err
	}
	build.aws = aws.New(&awsCfg, ctx)
	ctainers, err := containers.New(ctx)
	if err != nil {
		return &build, err
	}
	build.containers = ctainers
	appJSON, err := ParseAppJson(ctx)
	if err != nil {
		return &build, err
	}
	build.AppJSON = appJSON
	apppackToml, err := ParseAppPackToml(ctx)
	if err != nil {
		return &build, err
	}
	err = apppackToml.Validate()
	if err != nil {
		return &build, err
	}
	build.AppPackToml = apppackToml
	return &build, nil
}

func (b *Build) Log() *zerolog.Logger {
	return log.Ctx(b.Ctx)
}

func (b *Build) System() string {
	if b.AppPackToml.UseDockerfile() {
		return DockerBuildSystemKeyword
	}
	return BuildpackBuildSystemKeyword
}

func (b *Build) prParameterName() string {
	return fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", b.Appname, b.CodebuildSourceVersion)
}

// ConvertAppJson checks if an app.json file exists, but an apppack.toml file does not.
// If so, it converts the app.json file to an apppack.toml file.
func (b *Build) ConvertAppJson() error {
	// check if app.json file exists
	appJsonExists, err := b.state.FileExists("app.json")
	if err != nil {
		return err
	}
	// check if apppack.toml file exists
	apppackTomlExists, err := b.state.FileExists("apppack.toml")
	if err != nil {
		return err
	}
	if appJsonExists && !apppackTomlExists {
		// convert app.json to apppack.toml
		b.Log().Info().Msg("Converting app.json to apppack.toml")
		t := b.AppJSON.ToApppackToml()
		return b.state.WriteTomlToFile("apppack.toml", t)
	}
	return nil
}

func (b *Build) ConfigParameterPaths() []string {
	if b.Pipeline {
		return []string{
			fmt.Sprintf("/apppack/pipelines/%s/config/", b.Appname),
			fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s/config/", b.Appname, b.CodebuildSourceVersion),
		}
	}
	return []string{fmt.Sprintf("/apppack/apps/%s/config/", b.Appname)}
}

func (b *Build) SetPRStatus(status string) (*PRStatus, error) {
	parameterName := b.prParameterName()
	prStatus := PRStatus{
		PullRequest: b.CodebuildSourceVersion,
		Status:      status,
	}
	// convert the PRStatus struct to JSON
	prStatusJSON, err := json.Marshal(prStatus)
	if err != nil {
		return nil, err
	}
	err = b.aws.SetParameter(parameterName, string(prStatusJSON))
	if err != nil {
		return nil, err
	}
	return &prStatus, nil
}

func (b *Build) GetPRStatus() (*PRStatus, error) {
	parameterName := b.prParameterName()
	b.Log().Debug().Str("pr", b.CodebuildSourceVersion).Msg("getting PR status")
	prStatusJSON, err := b.aws.GetParameter(parameterName)
	if err != nil {
		return nil, err
	}
	var prStatus PRStatus
	err = json.Unmarshal([]byte(prStatusJSON), &prStatus)
	if err != nil {
		return nil, err
	}
	return &prStatus, nil
}

func (b *Build) reviewAppStackName() string {
	prNumber := strings.TrimPrefix(b.CodebuildSourceVersion, "pr/")
	return fmt.Sprintf("apppack-reviewapp-%s%s", b.Appname, prNumber)
}

func (b *Build) ReviewAppStackExists() (bool, error) {
	_, err := b.aws.DescribeStack(b.reviewAppStackName())
	if err != nil {
		// TODO check for specific error that means stack doesn't exist
		return false, err
	}
	return true, nil
}

func (b *Build) DestroyReviewAppStack() error {
	return b.aws.DestroyStack(b.reviewAppStackName())
}

func (b *Build) DockerLogin() error {
	if b.DockerHubUsername == "" || b.DockerHubAccessToken == "" {
		b.Log().Debug().Msg("no Docker Hub credentials provided, skipping login")
		return nil
	}
	b.Log().Debug().Str("username", b.DockerHubUsername).Msg("logging in to Docker Hub")
	return containers.Login("", b.DockerHubUsername, b.DockerHubAccessToken)
}

func (b *Build) ECRLogin() error {
	b.Log().Debug().Str("repo", b.ECRRepo).Msg("logging in to ECR")
	username, password, err := b.aws.GetECRLogin()
	if err != nil {
		return err
	}
	return containers.Login(fmt.Sprintf("https://%s", b.ECRRepo), username, password)
}

func (b *Build) ImageName() (string, error) {
	gitsha, err := b.state.GitSha()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", b.ECRRepo, gitsha), nil
}

func (b *Build) NewPRStatus() string {
	if b.CreateReviewApp {
		return "created"
	}
	if b.CodebuildWebhookEvent == "PULL_REQUEST_CREATED" || b.CodebuildWebhookEvent == "PULL_REQUEST_REOPENED" {
		return "open"
	}
	if b.CodebuildWebhookEvent == "PULL_REQUEST_MERGED" {
		return "merged"
	}
	_, err := b.GetPRStatus()
	if err != nil {
		b.Log().Debug().Err(err).Msg("failed to get PR status")
		return "open"
	}
	b.Log().Warn().Msg("next PR status is unknown")
	return ""
}

func (b *Build) HandlePR() error {
	if !b.Pipeline {
		return nil
	}
	if !strings.HasPrefix(b.CodebuildSourceVersion, "pr/") {
		return fmt.Errorf("not a pull request: CODEBUILD_SOURCE_VERSION=%s", b.CodebuildSourceVersion)
	}
	newStatus := b.NewPRStatus()
	b.Log().Debug().Str("status", newStatus).Msg("PR status")
	if newStatus == "merged" {
		hasReviewApp, err := b.ReviewAppStackExists()
		// TODO err is ignored because it usually means the stack doesn't exist
		if err != nil {
			b.Log().Debug().Err(err).Msg("unable to access review app stack")
		}
		if hasReviewApp {
			b.Log().Info().Str("pr", b.CodebuildSourceVersion).Msg("deleting review app")
			if err := b.DestroyReviewAppStack(); err != nil {
				return err
			}
		}
		return b.SkipBuild()
	}
	status, err := b.GetPRStatus()
	// if the status needs to change, update it
	noStatus := err != nil
	keepStatus := newStatus == ""
	// if no status exists and no status to set, set it to "open"
	if noStatus {
		status = &PRStatus{Status: ""}
	}
	statusChanged := status.Status != newStatus
	if noStatus || (statusChanged && !keepStatus) {
		status, err = b.SetPRStatus(newStatus)
		if err != nil {
			return err
		}
	}
	if status.Status != "created" {
		return b.SkipBuild()
	}
	return nil
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func (b *Build) StartAddons() (map[string]string, error) {
	// if "heroku-redis:in-dyno" in addons start redis:apline
	// if "heroku-postgresql:in-dyno" in addons start postgres:alpine
	envOverides := map[string]string{}
	var err error
	// dedupe addons
	addons := removeDuplicateStr(b.AppJSON.GetTestAddons())
	// use a switch statement to iterate over addons
	for _, addon := range addons {
		switch addon {
		case "heroku-redis:in-dyno":
			if err = b.containers.RunContainer("redis", b.CodebuildBuildId, &container.Config{Image: "redis:alpine"}); err != nil {
				return nil, err
			}
			envOverides["REDIS_URL"] = "redis://redis:6379"
		case "heroku-postgresql:in-dyno":
			if err = b.containers.RunContainer("db", b.CodebuildBuildId, &container.Config{Image: "postgres:alpine"}); err != nil {
				return nil, err
			}
			envOverides["DATABASE_URL"] = "postgres://postgres:postgres@db:5432/postgres"
		}
	}
	return envOverides, nil
}

func (b *Build) RunPrebuild() error {
	b.Log().Debug().Msg("running prebuild")
	defer b.containers.Close()
	err := b.HandlePR()
	if err != nil {
		return err
	}

	// start downloading cache while we do other work
	var copyError error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Log().Info().Msg("downloading build cache")
		copyError = b.aws.CopyFromS3(b.ArtifactBucket, "cache", CacheDirectory)
	}()

	err = b.state.MvGitDir()
	if err != nil {
		return err
	}
	err = b.DockerLogin()
	if err != nil {
		return err
	}
	err = b.ECRLogin()
	if err != nil {
		return err
	}
	c, err := containers.New(b.Ctx)
	if err != nil {
		return err
	}
	if b.System() == DockerBuildSystemKeyword {
		err = b.DockerPrebuild()
	} else {
		err = b.BuildpackPrebuild(c)
	}
	if err != nil {
		return err
	}

	err = c.CreateNetwork(b.CodebuildBuildId)
	if err != nil {
		return err
	}
	envOverrides, err := b.StartAddons()
	if err != nil {
		return err
	}
	err = b.state.WriteEnvFile(&envOverrides)
	if err != nil {
		return err
	}
	if err = b.ConvertAppJson(); err != nil {
		return err
	}
	// make sure cache has finished downloading
	wg.Wait()
	if copyError != nil {
		b.Log().Warn().Err(copyError).Msg("failed to download build cache")
	}
	return nil
}

func (b *Build) DockerPrebuild() error {
	b.Log().Debug().Msg("running docker prebuild")
	ready, err := usingBuildxBuilder(b.Ctx)
	if err != nil {
		return err
	}
	if ready {
		b.Log().Debug().Msg("docker buildx builder is ready")
		return nil
	}
	b.Log().Info().Msg("setting up docker buildx builder")
	cmd := exec.Command(
		"docker", "buildx", "create",
		"--use",
		"--name", strings.ReplaceAll(b.CodebuildBuildId, ":", "-"),
		"--driver", "docker-container",
		"--bootstrap",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *Build) BuildpackPrebuild(c *containers.Containers) error {
	b.Log().Debug().Msg("running buildpack prebuild")
	b.Log().Info().Msg("pulling buildpack images")
	for _, image := range b.BuildpackBuilders() {

		err := c.PullImage(fmt.Sprintf("%s/%s", DockerHubMirror, image))
		if err != nil {
			return err
		}
	}
	return nil
}

func usingBuildxBuilder(ctx context.Context) (bool, error) {
	cmd := exec.Command("docker", "buildx", "inspect")
	outputBuffer := &bytes.Buffer{}
	cmd.Stdout = outputBuffer
	cmd.Stderr = outputBuffer
	if err := cmd.Run(); err != nil {
		return false, err
	}
	re := regexp.MustCompile(`Driver:\s+docker-container`)
	if !re.Match(outputBuffer.Bytes()) {
		return false, nil
	}
	re = regexp.MustCompile(`Status:\s+running`)
	if !re.Match(outputBuffer.Bytes()) {
		return false, nil
	}
	return true, nil
}
