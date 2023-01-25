DOCKER_REPO := public.ecr.aws/d9q4v8a4/apppack-build

.PHONY: build
build:
#	docker build --platform linux/amd64 -t $(DOCKER_REPO):latest .
	docker build --platform linux/amd64 -t $(DOCKER_REPO):builder .

.PHONY: ecr-login
ecr-login:
	aws ecr-public get-login-password | docker login --username AWS --password-stdin $(DOCKER_REPO)

.PHONY: push
push: ecr-login
	docker push $(DOCKER_REPO):latest
