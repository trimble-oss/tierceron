package factory

type EnvConfig struct {
	Env      string `yaml:"-"`
	Dbuser   string `yaml:"dbuser,omitempty"`
	Dbpasswd string `yaml:"dbpassword,omitempty"`
	Maxconn  string `yaml:"maxconn,omitempty"` // Maximum allowed db connections
}
