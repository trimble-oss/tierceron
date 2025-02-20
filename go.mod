module github.com/trimble-oss/tierceron

go 1.23.3

require (
	github.com/denisenkom/go-mssqldb v0.12.0
	github.com/dolthub/go-mysql-server v0.12.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.6.0
	github.com/hashicorp/vault/api v1.1.0
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267 // indirect; IMPORTANT! This must match vault sdk used by vault for plugin to be stable!
	github.com/julienschmidt/httprouter v1.3.0
	github.com/rs/cors v1.7.0
	github.com/sergi/go-diff v1.2.0
	github.com/twitchtv/twirp v5.12.1+incompatible
	github.com/xo/dburl v0.9.0
	golang.org/x/crypto v0.32.0
	golang.org/x/sys v0.29.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/pavlo-v-chernykh/keystore-go/v4 v4.4.1
	github.com/sendgrid/rest v2.6.9+incompatible
	github.com/sendgrid/sendgrid-go v3.12.0+incompatible
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.16.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.8.0
	github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry v0.2.2
	github.com/docker/docker v26.1.5+incompatible
	github.com/graphql-go/graphql v0.8.1-0.20220614210743-09272f350067
	github.com/trimble-oss/tierceron-core/v2 v2.1.7
	github.com/trimble-oss/tierceron-hat v1.2.9
	github.com/trimble-oss/tierceron/atrium v0.0.0-20241231000200-edfd1fe078b0
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck v0.0.0-20241220234051-2d8c369c5b69
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/klauspost/reedsolomon v1.12.4 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lafriks/go-shamir v1.2.0 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/templexxx/cpu v0.1.1 // indirect
	github.com/templexxx/xorsimd v0.4.3 // indirect
	github.com/xtaci/kcp-go/v5 v5.6.16 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250124145028-65684f501c47 // indirect
)

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dolthub/vitess v0.0.0-20221121184553-8d519d0bbb91 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/glycerine/bchan v0.0.0-20170210221909-ad30cd867e1c // indirect
	github.com/go-git/go-billy/v5 v5.6.1 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gocraft/dbr/v2 v2.7.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/flatbuffers v2.0.6+incompatible // indirect
	github.com/hanwen/go-fuse v1.0.0 // indirect
	github.com/hanwen/go-fuse/v2 v2.7.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/lestrrat-go/strftime v1.0.4 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/trimble-oss/tierceron-nute v1.0.6 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.31.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/tools v0.23.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250124145028-65684f501c47 // indirect
	gopkg.in/src-d/go-errors.v1 v1.0.0 // indirect
	gotest.tools/v3 v3.5.1 // indirect
)

require (
	github.com/aws/aws-sdk-go v1.43.30
	github.com/frankban/quicktest v1.14.4 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang-sql/sqlexp v0.0.0-20170517235910-f1bb20e5a188 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/trimble-oss/tierceron-succinctly v0.0.0-20231202151147-a0fc3a0ba103
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/term v0.28.0
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	google.golang.org/grpc v1.69.4
	google.golang.org/protobuf v1.36.4
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace github.com/dolthub/go-mysql-server => github.com/trimble-oss/go-mysql-server v0.12.0-1.26

replace github.com/trimble-oss/tierceron/atrium => ./atrium

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck@v0.0.0-20241220234051-2d8c369c5b69 ./atrium/vestibulum/hive/plugins/trchealthcheck

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck => ./atrium/vestibulum/hive/plugins/trchealthcheck

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore => ./atrium/vestibulum/hive/plugins/trchealthcheck/hcore

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis => ./atrium/vestibulum/hive/plugins/atrium/vestibulum/hive/plugins/trcmutabilis

//replace github.com/trimble-oss/tierceron-hat => ../tierceron-hat

// replace github.com/trimble-oss/tierceron-core/v2 => ../tierceron-core
