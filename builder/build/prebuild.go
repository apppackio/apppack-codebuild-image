package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/apppackio/codebuild-image/builder/aws"
	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/apppackio/codebuild-image/builder/state"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/buildpacks/pack/pkg/logging"
	"github.com/heroku/color"
	"github.com/rs/zerolog/log"
)

// define a struct named Build
type Build struct {
	Appname                string
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
	Context                context.Context
	LogLevel               string
	logger                 logging.Logger
	aws                    aws.AWSInterface
	state                  state.State
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
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	logger := logging.NewLogWithWriters(color.Stdout(), color.Stderr())
	logging.WithVerbose()
	return &Build{
		Appname:                os.Getenv("APPNAME"),
		aws:                    aws.New(&awsCfg, ctx),
		Branch:                 GetenvFallback([]string{"BRANCH", "CODEBUILD_WEBHOOK_HEAD_REF", "CODEBUILD_SOURCE_VERSION"}),
		CodebuildBuildId:       os.Getenv("CODEBUILD_BUILD_ID"),
		CodebuildBuildNumber:   os.Getenv("CODEBUILD_BUILD_NUMBER"),
		CodebuildWebhookEvent:  GetenvFallback([]string{"CODEBUILD_WEBHOOK_EVENT", "PULL_REQUEST_UPDATED"}),
		CodebuildSourceVersion: os.Getenv("CODEBUILD_SOURCE_VERSION"),
		DockerHubUsername:      os.Getenv("DOCKERHUB_USERNAME"),
		DockerHubAccessToken:   os.Getenv("DOCKERHUB_ACCESS_TOKEN"),
		ECRRepo:                os.Getenv("DOCKER_REPO"),
		Pipeline:               os.Getenv("PIPELINE") == "1",
		CreateReviewApp:        os.Getenv("REVIEW_APP_STATUS") == "created",
		LogLevel:               "debug",
		Context:                ctx,
		logger:                 logger,
		state:                  state.New(),
	}, nil
}

func (b *Build) prParameterName() string {
	return fmt.Sprintf("/apppack/pipelines/%s/review-apps/%s", b.Appname, b.CodebuildSourceVersion)
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
	log.Debug().Msg("getting PR status")
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
	log.Debug().Msg("logging in to Docker Hub")
	return containers.Login("https://index.docker.io/v1/", b.DockerHubUsername, b.DockerHubAccessToken)
}

func (b *Build) ECRLogin() error {
	log.Debug().Msg("logging in to ECR")
	username, password, err := b.aws.GetECRLogin()
	if err != nil {
		return err
	}
	return containers.Login(fmt.Sprintf("https://%s", b.ECRRepo), username, password)
}

func (b *Build) HandlePR() error {
	if !b.Pipeline {
		return nil
	}
	if !strings.HasPrefix(b.CodebuildSourceVersion, "pr/") {
		return fmt.Errorf("not a pull request: CODEBUILD_SOURCE_VERSION=%s", b.CodebuildSourceVersion)
	}
	var err error
	// REVIEW_APP_STATUS is set by the CLI when a review app is created
	if b.CreateReviewApp {
		_, err = b.SetPRStatus("created")
		if err != nil {
			return err
		}
	}
	if b.CodebuildWebhookEvent == "PULL_REQUEST_CREATED" || b.CodebuildWebhookEvent == "PULL_REQUEST_REOPENED" {
		_, err = b.SetPRStatus("open")
		if err != nil {
			return err
		}
		return b.SkipBuild()
	}
	if b.CodebuildWebhookEvent == "PULL_REQUEST_UPDATED" {
		// if we weren't aware of the PR yet, set the status to open
		status, err := b.GetPRStatus()
		if err != nil {
			log.Debug().Err(err).Msg("failed to get PR status")
			// if the parameter doesn't exist, mark the PR as open
			_, err = b.SetPRStatus("open")
			if err != nil {
				return err
			}
			return b.SkipBuild()
		}
		if status.Status == "created" || status.Status == "creating" {
			return nil
		}
		// if the review app isn't created, mark the PR as open and skip the build
		log.Info().Msg(fmt.Sprintf("%s not deployed, skipping build", b.CodebuildSourceVersion))
		if status.Status != "open" {
			_, err = b.SetPRStatus("open")
			if err != nil {
				return err
			}
		}
		return b.SkipBuild()
	}
	if b.CodebuildWebhookEvent == "PULL_REQUEST_MERGED" {
		hasReviewApp, _ := b.ReviewAppStackExists()
		// TODO err is ignored because it usually means the stack doesn't exist
		// if err != nil {
		// 	return err
		// }
		if hasReviewApp {
			log.Info().Msg(fmt.Sprintf("deleting review app for %s", b.CodebuildSourceVersion))
			err = b.DestroyReviewAppStack()
			if err != nil {
				return err
			}
		}
		return b.SkipBuild()
	}
	return nil
}

func (b *Build) StartAddons(addons []string) (map[string]string, error) {
	// if "heroku-redis:in-dyno" in addons start redis:apline
	// if "heroku-postgresql:in-dyno" in addons start postgres:alpine
	hasRedis := false
	hasPostgres := false
	envOverides := map[string]string{}
	var err error
	c, err := containers.New(b.Context)
	if err != nil {
		return envOverides, err
	}
	for _, addon := range addons {
		if addon == "heroku-redis:in-dyno" && !hasRedis {
			err = c.RunContainer("redis:alpine", "redis", b.CodebuildBuildId)
			if err != nil {
				return nil, err
			}
			envOverides["REDIS_URL"] = "redis://redis:6379"
			hasRedis = true
		} else if addon == "heroku-postgresql:in-dyno" && !hasPostgres {
			err = c.RunContainer("postgres:alpine", "db", b.CodebuildBuildId)
			if err != nil {
				return nil, err
			}
			envOverides["DATABASE_URL"] = "postgres://postgres:postgres@db:5432/postgres"
			hasPostgres = true
		}
	}
	return envOverides, nil
}

func (b *Build) RunPrebuild() error {
	err := b.HandlePR()
	if err != nil {
		return err
	}
	appJson, err := ParseAppJson()
	if err != nil {
		return err
	}
	err = MvGitDir()
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
	c, err := containers.New(b.Context)
	if err != nil {
		return err
	}
	for _, image := range appJson.GetBuilders() {
		err = c.PullImage(image, b.logger)
		if err != nil {
			return err
		}
	}
	err = c.CreateDockerNetwork(b.CodebuildBuildId)
	if err != nil {
		return err
	}
	if appJson.Environments != nil {
		test, ok := appJson.Environments["test"]
		if ok {
			b.StartAddons(test.Addons)
		}
	}
	return nil
}

// MvGitDir moves the git directory to the root of the project
// Codebuild has a .git file that points to the real git directory
func MvGitDir() error {
	// test if .git is a file
	fileInfo, err := os.Stat(".git")
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return nil
	}
	// read the contents of .git
	gitFile, err := os.ReadFile(".git")
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`^gitdir:\s*(.*)$`)
	matches := re.FindSubmatch(gitFile)
	if len(matches) != 2 {
		return fmt.Errorf("failed to parse .git file")
	}
	// delete the .git file
	err = os.Remove(".git")
	if err != nil {
		return err
	}
	// move the git directory to the root of the project
	return os.Rename(string(matches[1]), ".git")
}
