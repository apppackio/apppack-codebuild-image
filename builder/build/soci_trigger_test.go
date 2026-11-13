package build

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/apppackio/codebuild-image/builder/containers"
	"github.com/stretchr/testify/mock"
)

func TestSociPayload(t *testing.T) {
	payload, err := sociPayload("deadbeef")
	if err != nil {
		t.Fatalf("sociPayload returned an error: %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if body["tag"] != "deadbeef" {
		t.Errorf("tag = %q, want deadbeef", body["tag"])
	}
	if len(body) != 1 {
		t.Errorf("payload has %d fields, want only tag", len(body))
	}
}

func TestImageTag(t *testing.T) {
	if got := imageTag("123456789012.dkr.ecr.us-east-1.amazonaws.com/repo:deadbeef"); got != "deadbeef" {
		t.Errorf("imageTag = %q, want deadbeef", got)
	}
	if got := imageTag("repo-without-tag"); got != "" {
		t.Errorf("imageTag = %q, want empty", got)
	}
}

func TestNotifySociGeneratorSkipsWithoutARN(t *testing.T) {
	t.Setenv(sociGeneratorARNEnv, "")
	mockedAWS := new(MockAWS)
	b := Build{aws: mockedAWS, logger: &testLogger}

	b.notifySociGenerator(context.Background(), &containers.BuildConfig{
		Image: "123456789012.dkr.ecr.us-east-1.amazonaws.com/repo:deadbeef",
	})

	mockedAWS.AssertNotCalled(t, "InvokeAsync", mock.Anything, mock.Anything)
}
