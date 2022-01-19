module tierceron

go 1.15

require (
	VaultConfig.Bootstrap v0.0.0-00010101000000-000000000000
	VaultConfig.TenantConfig v0.0.0-00010101000000-000000000000
	github.com/denisenkom/go-mssqldb v0.11.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dolthub/go-mysql-server v0.11.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/graphql-go/graphql v0.7.9
	github.com/hashicorp/vault-plugin-secrets-kv v0.10.1
	github.com/hashicorp/vault/api v1.3.0
	github.com/hashicorp/vault/sdk v0.3.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/rs/cors v1.7.0
	github.com/sanity-io/litter v1.5.1 // indirect
	github.com/sendgrid/rest v2.6.0+incompatible // indirect
	github.com/sendgrid/sendgrid-go v3.6.1+incompatible
	github.com/sergi/go-diff v1.2.0
	github.com/twitchtv/twirp v5.12.1+incompatible
	github.com/txn2/txeh v1.3.0
	github.com/xo/dburl v0.9.0
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654
	gopkg.in/yaml.v2 v2.4.0
)

replace VaultConfig.Bootstrap => ../VaultConfig.Bootstrap

replace VaultConfig.TenantConfig => ../VaultConfig.TenantConfig

replace github.com/dolthub/go-mysql-server => ../../go-mysql-server
