package factory

type EnvConfig struct {
	env      string
	dbuser   string `yaml:"dbuser,omitempty"`
	dbpasswd string `yaml:"dbpassword,omitempty"`
	maxconn  string `yaml:"maxconn,omitempty"` // Maximum allowed db connections
}
