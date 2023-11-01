package capauth

type TrcShConfig struct {
	Env          string
	EnvContext   string // Current env context...
	VaultAddress *string
	CToken       *string
	ConfigRole   *string
	PubRole      *string
	KubeConfig   *string
}
