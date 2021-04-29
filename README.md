# AppPack Codebuild Image

A minimal image for used in AWS Codebuild by AppPack

## Why?

The [standard images](https://github.com/aws/aws-codebuild-docker-images) have all sorts of tools installed and can act unexpectedly.

For example, if the code has a `.python-version` file in it with a Python version not installed in the image, the `aws` CLI tool won't run.

```
pyenv: version `x.x.x' is not installed (set by /codebuild/output/.../.python-version)
```
