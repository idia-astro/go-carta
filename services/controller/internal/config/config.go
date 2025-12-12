// internal/config/config.go
package config

type AuthMode string

const (
    AuthNone AuthMode = "none"
    AuthPAM  AuthMode = "pam"
    AuthOIDC AuthMode = "oidc"
    AuthBoth AuthMode = "both" // optional
)

type OIDCConfig struct {
    IssuerURL      string
    ClientID       string
    ClientSecret   string
    RedirectURL    string
    AllowedAud     []string
    AllowedGroups  []string
}

type PAMConfig struct {
    ServiceName string // e.g. "login" or "carta"
}

type Config struct {
    Port           int
    Hostname       string
    SpawnerAddress string
    BaseFolder     string
    FrontendDir    string
    AuthMode       AuthMode
    OIDC           OIDCConfig
    PAM            PAMConfig
}
