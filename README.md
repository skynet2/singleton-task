# singleton-task

![build workflow](https://github.com/skynet2/singleton-task/actions/workflows/build.yaml/badge.svg?branch=master)
[![codecov](https://codecov.io/gh/skynet2/singleton-task/branch/master/graph/badge.svg?token=Y71ZQHTLKC)](https://codecov.io/gh/skynet2/singleton-task)
[![go-report](https://goreportcard.com/badge/github.com/skynet2/singleton-task?nocache=true)](https://goreportcard.com/report/github.com/skynet2/singleton-task)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/skynet2/singleton-task)](https://pkg.go.dev/github.com/skynet2/singleton-task?tab=doc)

## Description
Due to the nature of microservices, k8s etc., we have multiple pods\instances of the same service, but in some cases, there is a
the requirement to run the specific job in a single instance, for example, some parsing jobs with rate limiting, database migrations, etc.
Here come the leader election algorithms.

This implementation is targeted for [redlock](https://redis.io/docs/reference/patterns/distributed-locks/) approach
## Installation
```shell
go get github.com/skynet2/singleton-task
```

## Quickstart
