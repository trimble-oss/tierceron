package factory

type EnvConfig struct {
	Env string `yaml:"-"`
	// Mysql access.
	MysqlDburl    string `yaml:"mysqldburl,omitempty"`
	MysqlDbuser   string `yaml:"mysqldbuser,omitempty"`
	MysqlDbpasswd string `yaml:"mysqldbpassword,omitempty"`
	// Local db access.
	Dbuser   string `yaml:"dbuser,omitempty"`
	Dbpasswd string `yaml:"dbpassword,omitempty"`
	Maxconn  string `yaml:"maxconn,omitempty"` // Maximum allowed db connections
}
