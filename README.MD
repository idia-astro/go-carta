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

## Configure

Common configuration options can be provided as optional parameters to service commands. Invoke the service with `--help` to see a list of available commands. The recommended way to configure persistent options is through a configuration file in [TOML format](https://toml.io). An annotated [example configuration file](config.toml.example) is provided. By default, services look for configuration in `/etc/carta/config.toml`.

You can specify a different file location with a command line parameter:

```bash
./build/carta-spawn --config=./config.toml
./build/carta-ctl --config=./config.toml
```

Configuration options with string, integer or boolean values can also be overridden on the commandline with the `--override` parameter.

### Configuring the controller

#### Authentication mode

The controller can use PAM for authentication, or communicate with an OIDC service such as Keycloak.

### Configuring the spawner

#### Worker executable

A common initial use case for `go-carta` is as a replacement for the old CARTA controller: in this scenario, the spawner must be configured to use the CARTA backend executable to launch worker processes. This option may be set on the command line or in the configuration file (as in the example file).

```bash
./build/carta-spawn --worker_process=carta_backend
```

### Complete example

