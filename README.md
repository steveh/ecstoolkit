# ecstoolkit

[![Go Reference](https://pkg.go.dev/badge/github.com/steveh/ecstoolkit.svg)](https://pkg.go.dev/github.com/steveh/ecstoolkit)
[![CI](https://github.com/steveh/ecstoolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/steveh/ecstoolkit/actions/workflows/ci.yml)

This is a hard fork of the AWS [session-manager-plugin](https://github.com/aws/session-manager-plugin) as of v1.2.707.0. The primary audience is people shaped like platform engineers who want to integrate connecting to ECS in their own Go tooling, such as an in-house CLI.

It represents a significant rewrite and modernization of the original library. The main goals were to make it easy to integrate, improve code quality, remove technical debt, and address concurrency and maintainability issues. 

## Changes Since Upstream 

* Removed packaging tooling and AWS CLI plugin capabalities.
* Removed vendored code.
* Removed Windows support and its dependencies (I have no Windows systems to test on).
* Removed deprecated and unusued code.
* Replaced unmaintained UUID library with `google/uuid`.
* Replaced logging with `log/slog`.
* Upgraded to AWS SDK for Go v2.
* Removed global state and `init()` functions.
* Fixed data races and added mutexes/atomic where required.
* Fixed most `golangci-lint` errors.
* Removed direct printing to stdout and use of `os.Exit()`.
* Refactored and simplified how sessions are established.

## Contributing

I'm very happy to accept changes that would improve `ecstoolskit`'s usefulness as a library.
If someone wants to maintain Windows support that would be great too.

## License

Apache License. Original source copyright 2018 Amazon.com, Inc. or its affiliates.

I've pulled some changes from unmerged pull requests, which hopefully preserved the original authors when cherry-picking.

There is no vendored code in this fork. Third party dependencies are as shown in [go.mod](go.mod).
