# go-carta

[![Go Reference](https://pkg.go.dev/badge/github.com/CARTAvis/go-carta.svg)](https://pkg.go.dev/github.com/CARTAvis/go-carta)

## Required tools and packages

### Golang

### Protocol buffer compiler and grpc plugin

The protocol buffer compiler and grpc plugin can be installed in a number of ways:

golang:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
source ~/.bashrc
# verify:
which protoc-gen-go-grpc
which protoc-gen-go
```

homebrew (macOS):
```bash
brew install protoc-gen-go protoc-gen-go-grpc
```

apt-get (ubuntu):
```bash
sudo apt install protoc-gen-go protoc-gen-go-grpc
```

Tested with: 
- `protoc-gen-go v1.36.6` and `protoc-gen-go-grpc v1.5.1` (macOS)
- `protoc-gen-go v1.32.0` and `protoc-gen-go-grpc v1.0` (ubuntu 24.04.2 LTS)

### cfitsio

The worker service uses cgo to compile some simple functions that utilise cfitsio. Install `libcfitsio-dev` (ubuntu) or `cfitsio` (macOS homebrew).

## Build
Ensure the `carta-protobuf` submodule is initialised:
```shell
git submodule update --init
```
gRPC files need to be built first, followed by the services. Use the provided Makefile:

```shell
# From the project root
make proto     # Build gRPC protobuf files
make services  # Build all services
```

For other development tasks, see available commands:
```bash
make help
```

## Run
Compiled services are found in the `build` folder.

```bash
# From the project root
./build/carta-spawn --worker_process build/worker --port 8080
./build/carta-ctl --spawner_address http://localhost:8080 --port 8081
```