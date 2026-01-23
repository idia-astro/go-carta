## Examples
This file shows some command lines for running the carta-go code on an Ubuntu system.


Command line showing starting the Spawner which is used to start the carta_backend processes at a site.

```$ ./build/carta-spawn --port 8085 --worker_exec "carta_backend"```

Once a spawner process is started, a CARTA Controller can be started. Below is a command line if PAM is being used for authentication.

```$  ./build/carta-ctl  --spawner_address "http://localhost:8085" --frontend_dir /usr/share/carta/frontend/ --log_level "info" --auth_mode pam```

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
allowed_aud = ["carta-ctl"]
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

