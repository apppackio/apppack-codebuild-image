# AppPack Codebuild Image

A minimal image for used in AWS Codebuild by AppPack. It includes the code to manage the build process in the `builder` directory.

## Why?

Amazon will tell you that, to limit time spent in provisioning, you should use the latest [Codebuild standard image](https://github.com/aws/aws-codebuild-docker-images) for your build. Unfortunately, the latest standard images are often not [well](https://www.reddit.com/r/aws/comments/lxnq82/codebuild_provisioning_taking_30s_too_long/) [cached](https://github.com/aws/aws-codebuild-docker-images/issues/296#issuecomment-1184748374) on Codebuild leading to long provisioning times (over 30s).

To minimize this issue, we use this image which contains the minimal set of tools we use to perform app builds. It is not cached on the runners, but is small enough that the provisioning time is usually less than 25s.

## Deployment

Codebuild will pull the latest image with the tag `builder`. To build a new image, push a tag to the repo in the form `vX.Y.Z`. In addition to updating the `builder` tag, this will also push an image with the same tag to ECR.
