# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

* Removed support for EOL `heroku-20` builds 

### Fixed

* Builds that use an in-dyno test database or Redis add-on (`heroku-postgresql:in-dyno`,
  `heroku-redis:in-dyno`) could intermittently fail during the pre-build phase with an
  error like `The container name "/db" is already in use`. These add-on containers now
  start with unique names, so a build no longer collides with a leftover container from
  a previous run.

## [2.5.0] - 2025-10-21

### Added

* Tear down review apps when a PULL_REQUEST_CLOSED event is received.

## [2.4.0] - 2025-09-22

### Fixed

* Enable Codebuild to archive `apppack.toml` from non-standard locations

### Security

* Updated Go version and underlying dependencies

## [2.3.0] - 2025-09-19

### Added

* apppack.toml file is now accessible by APPPACK_TOML environment variable, allowing different services in different environments. 

## [2.2.0] - 2025-06-16

### Changed

* Updated embedded version of `pack` for all but `heroku/buildpacks:20` builders

### Fixed

* `heroku/builder:24` can now be used as a builder

## [2.1.0] - 2025-02-10

### Fixed

* Release tasks work with latest metadata from Heroku Buildpacks
