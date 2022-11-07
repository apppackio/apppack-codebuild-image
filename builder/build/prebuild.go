package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/apppackio/codebuild-image/builder/awshelpers"
	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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
	ReviewAppStatus        string
	AWSConfig              *aws.Config
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

// WriteCommitTxt shells out to `git log -n1 --decorate=no` and writes stdout to commit.txt
func WriteCommitTxt() error {
	log.Debug().Msg("fetching git log")
	cmd, err := exec.Command("git", "log", "-n1", "--decorate=no").Output()
	if err != nil {
		return err
	}
	// write the output of the command to commit.txt
	log.Debug().Msg("writing commit.txt")
	return os.WriteFile("commit.txt", cmd, 0644)
}

func SkipBuild() error {
	err := WriteCommitTxt()
	if err != nil {
		return err
	}
	// touch files codebuild expects to exist
	for _, filename := range []string{"app.json", "build.log", "metadata.toml", "test.log"} {
		log.Debug().Msg(fmt.Sprintf("touching %s", filename))
		err = os.WriteFile(filename, []byte{}, 0644)
		if err != nil {
			return err
		}
	}
	// TODO write state so future steps can skip too
	return nil
}

func New() (*Build, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return &Build{
		Appname:                os.Getenv("APPNAME"),
		AWSConfig:              &awsCfg,
		Branch:                 GetenvFallback([]string{"BRANCH", "CODEBUILD_WEBHOOK_HEAD_REF", "CODEBUILD_SOURCE_VERSION"}),
		CodebuildBuildNumber:   os.Getenv("CODEBUILD_BUILD_NUMBER"),
		CodebuildWebhookEvent:  GetenvFallback([]string{"CODEBUILD_WEBHOOK_EVENT", "PULL_REQUEST_UPDATED"}),
		CodebuildSourceVersion: os.Getenv("CODEBUILD_SOURCE_VERSION"),
		DockerHubUsername:      os.Getenv("DOCKERHUB_USERNAME"),
		DockerHubAccessToken:   os.Getenv("DOCKERHUB_ACCESS_TOKEN"),
		ECRRepo:                os.Getenv("DOCKER_REPO"),
		Pipeline:               os.Getenv("PIPELINE") == "1",
		ReviewAppStatus:        os.Getenv("REVIEW_APP_STATUS"),
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
	err = awshelpers.SetParameter(*b.AWSConfig, parameterName, string(prStatusJSON))
	if err != nil {
		return nil, err
	}
	return &prStatus, nil
}

func (b *Build) GetPRStatus() (*PRStatus, error) {
	parameterName := b.prParameterName()
	log.Debug().Msg("getting PR status")
	prStatusJSON, err := awshelpers.GetParameter(*b.AWSConfig, parameterName)
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
	_, err := awshelpers.DescribeStack(*b.AWSConfig, b.reviewAppStackName())
	if err != nil {
		// TODO check for specific error that means stack doesn't exist
		return false, err
	}
	return true, nil
}

func (b *Build) DestroyReviewAppStack() error {
	return awshelpers.DestroyStack(*b.AWSConfig, b.reviewAppStackName())
}

func (b *Build) DockerLogin() error {
	log.Debug().Msg("logging in to Docker Hub")
	return containers.Login("https://index.docker.io/v1/", b.DockerHubUsername, b.DockerHubAccessToken)
}

func (b *Build) ECRLogin() error {
	log.Debug().Msg("logging in to ECR")
	username, password, err := awshelpers.GetECRLogin()
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
	if b.ReviewAppStatus == "created" {
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
		return SkipBuild()
	} else if b.CodebuildWebhookEvent == "PULL_REQUEST_UPDATED" {
		// if we weren't aware of the PR yet, set the status to open
		status, err := b.GetPRStatus()
		if err != nil {
			log.Debug().Err(err).Msg("failed to get PR status")
			// if the parameter doesn't exist, mark the PR as open
			status, err = b.SetPRStatus("open")
			if err != nil {
				return err
			}
		}
		// if the review app isn't created, mark the PR as open and skip the build
		if status.Status != "created" && status.Status != "creating" {
			log.Info().Msg(fmt.Sprintf("%s not deployed, skipping build", b.CodebuildSourceVersion))
			if status.Status != "open" {
				_, err = b.SetPRStatus("open")
				if err != nil {
					return err
				}
			}
			return SkipBuild()
		}
	} else if b.CodebuildWebhookEvent == "PULL_REQUEST_MERGED" {
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
		return SkipBuild()
	}
	return nil
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
	for _, image := range appJson.GetBuilders() {
		err = containers.PullImage(image)
		if err != nil {
			return err
		}
	}
	err = containers.CreateDockerNetwork(b.CodebuildBuildId)
	if err != nil {
		return err
	}
	if appJson.Environments != nil {
		test, ok := appJson.Environments["test"]
		if ok {
			StartAddons(b.CodebuildBuildId, test.Addons)
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
	gitFile, err := ioutil.ReadFile(".git")
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

func StartAddons(buildId string, addons []string) (map[string]string, error) {
	// if "heroku-redis:in-dyno" in addons start redis:apline
	// if "heroku-postgresql:in-dyno" in addons start postgres:alpine
	hasRedis := false
	hasPostgres := false
	envOverides := map[string]string{}
	var err error
	for _, addon := range addons {
		if addon == "heroku-redis:in-dyno" && !hasRedis {
			err = containers.RunContainer("redis:alpine", "redis", buildId)
			if err != nil {
				return nil, err
			}
			envOverides["REDIS_URL"] = "redis://redis:6379"
			hasRedis = true
		} else if addon == "heroku-postgresql:in-dyno" && !hasPostgres {
			err = containers.RunContainer("postgres:alpine", "db", buildId)
			if err != nil {
				return nil, err
			}
			envOverides["DATABASE_URL"] = "postgres://postgres:postgres@db:5432/postgres"
			hasPostgres = true
		}
	}
	return envOverides, nil
}
