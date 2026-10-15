## Architecture

![Specmatic gRPC Support Architecture](/assets/SpecmaticGRPCSupport.gif)

## Setup

### Cloning with submodules

1. Clone the repository

```shell
git clone https://github.com/specmatic/specmatic-order-bff-grpc-go
```

2. Initialize and update the `proto_files` submodule

```shell
git submodule update --init --recursive --remote
```

3. Enable automatic submodule updating when executing `git pull`

```shell
git config submodule.recurse true
```

### Contract Testing BFF using specmatic/enterprise docker image (preferred) with test containers

The BFF exposes the standard `grpc.health.v1.Health/Check` service. The Testcontainers setup waits for its listening port, startup log, and a `SERVING` health response before starting Specmatic.

```shell
go mod tidy
go test contract_test.go -v -count=1
```
