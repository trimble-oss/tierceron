module tierceron

go 1.17

require (
	VaultConfig.Bootstrap v0.0.0-00010101000000-000000000000
	VaultConfig.TenantConfig v0.0.0-00010101000000-000000000000
	github.com/denisenkom/go-mssqldb v0.11.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dolthub/go-mysql-server v0.11.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/graphql-go/graphql v0.7.9
	github.com/hashicorp/vault-plugin-secrets-kv v0.9.0
	github.com/hashicorp/vault/api v1.1.0
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267 // IMPORTANT! This must match vault sdk used by vault for plugin to be stable!
	github.com/julienschmidt/httprouter v1.3.0
	github.com/rs/cors v1.7.0
	github.com/sendgrid/sendgrid-go v3.6.1+incompatible
	github.com/sergi/go-diff v1.2.0
	github.com/twitchtv/twirp v5.12.1+incompatible
	github.com/txn2/txeh v1.3.0
	github.com/xo/dburl v0.9.0
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654
	gopkg.in/yaml.v2 v2.4.0
)

require VaultConfig.Test v0.0.0-00010101000000-000000000000

require (
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/dolthub/vitess v0.0.0-20220215181831-b3d3007ceb62 // indirect
	github.com/fatih/color v1.7.0 // indirect
	github.com/frankban/quicktest v1.14.1 // indirect
	github.com/go-co-op/gocron v1.13.0 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang/glog v0.0.0-20210429001901-424d2337a529 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-hclog v1.0.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-kms-wrapping/entropy v0.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-plugin v1.4.3 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.6 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d // indirect
	github.com/lestrrat-go/strftime v1.0.4 // indirect
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sendgrid/rest v2.6.0+incompatible // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/src-d/go-oniguruma v1.1.0 // indirect
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	golang.org/x/tools v0.1.9 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20210506142907-4a47615972c2 // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/src-d/go-errors.v1 v1.0.0 // indirect
)

replace VaultConfig.Bootstrap => ../../Azure/VaultConfig.Bootstrap

replace VaultConfig.TenantConfig => ../../Azure/VaultConfig.TenantConfig

replace VaultConfig.Test => ../../Azure/VaultConfig.Test

replace github.com/dolthub/go-mysql-server => ../../Github/go-mysql-server
