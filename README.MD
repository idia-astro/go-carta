# go-carta

[![Go Reference](https://pkg.go.dev/badge/github.com/idia-astro/go-carta.svg)](https://pkg.go.dev/github.com/idia-astro/go-carta)

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
brew install brew install protoc-gen-go protoc-gen-go-grpc
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
Ensure the carta-protobuf` submodule is initialised:
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
./build/worker --port 8080
./build/controller --remoteAddress localhost:8080
./build/spawner --workerProcess build/worker --port 8080
```


## Examples
This section shows some command lines for running the carta-go code on an Ubuntu system.


Command line showing starting the Spawner which is used to start the carta_backend processes at a site.

```$ ./build/carta-spawn --port 8085 --worker_process "carta_backend"```

Once a spawner process is started, a CARTA Controller can be started. Below is a command line if PAM is being used for authentication.

``$  ./build/carta-ctl  --spawner_address "http://localhost:8085" --frontend_dir /usr/share/carta/frontend/ --log_level "info" --auth_mode pam```

The CARTA Controller can also talk to an OIDC service such as Keycloak. In that case auth_mode is set to oidc and additional configuration is needed to set the OIDC service URL and the OIDC Secret needed to connect to this service. These values have to be set in the main configuration file rather from the command line.

Below is the contents of a config.toml that works with Keyloak (the the secret removed):

```
environment = "development"
log_level = "info"

[controller]
port = 8081
hostname = ""
frontend_dir = "/usr/share/carta/frontend"
spawner_address = "http://localhost:8085"
base_folder = "/mnt/data"
auth_mode = "oidc"   # or "both" if you support that

[controller.oidc]
issuer_url = "http://localhost:8080/realms/carta"
client_id = "carta-ctl"
client_secret = "IODC_SECRET"
redirect_url = "http://localhost:8081/oidc/callback"

# optional lists
allowed_aud = ["YOUR_CLIENT_ID"]
allowed_groups = ["carta-users", "admins"]

[controller.pam]
service_name = "carta"

[spawner]
port = 8085
hostname = ""
worker_process = "carta_backend"
timeout = "5s"
```

Using the above config file, the CARTA processes can be started with:

```$  ./build/carta-spawn --config ./config.toml```

and

```$ ./build/carta-ctl --config ./config.toml```

