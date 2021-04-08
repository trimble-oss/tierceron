module Vault.Whoville

go 1.15

require (
	VaultConfig.Bootstrap v0.0.0-00010101000000-000000000000
	github.com/denisenkom/go-mssqldb v0.0.0-20200620013148-b91950f658ec
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-sql-driver/mysql v1.5.0
	github.com/golang/protobuf v1.4.2
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/graphql-go/graphql v0.7.9
	github.com/hashicorp/vault/api v1.0.4
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kr/pretty v0.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/cors v1.7.0
	github.com/sendgrid/rest v2.6.0+incompatible // indirect
	github.com/sendgrid/sendgrid-go v3.6.1+incompatible
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/twitchtv/twirp v5.12.1+incompatible
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/sys v0.0.0-20210119212857-b64e53b001e4
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
)

replace VaultConfig.Bootstrap => ../VaultConfig.Bootstrap
