package factory

type EnvConfig struct {
	env      string `json:"env,omitempty"`
	dbuser   string `json:"dbuser,omitempty"`
	dbpasswd string `json:"dbpassword,omitempty"`
	maxconn  string `json:"maxconn,omitempty"` // Maximum allowed db connections
}
