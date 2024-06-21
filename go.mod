module github.com/trimble-oss/tierceron

go 1.21.6

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
	golang.org/x/crypto v0.24.0
	golang.org/x/sys v0.21.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/pavlo-v-chernykh/keystore-go/v4 v4.4.1
	github.com/sendgrid/rest v2.6.9+incompatible
	github.com/sendgrid/sendgrid-go v3.12.0+incompatible
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a

)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.11.1
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.6.0
	github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry v0.2.0
	github.com/docker/docker v26.1.4+incompatible
	github.com/graphql-go/graphql v0.8.1-0.20220614210743-09272f350067
	github.com/trimble-oss/tierceron-hat v1.1.1
	github.com/trimble-oss/tierceron/atrium v0.0.0-20240326213127-e85d6193e1c6
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.8.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/klauspost/reedsolomon v1.11.8 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lafriks/go-shamir v1.1.0 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/templexxx/cpu v0.1.0 // indirect
	github.com/templexxx/xorsimd v0.4.2 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/xtaci/kcp-go/v5 v5.6.3 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240610135401-a8a62080eff3 // indirect
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
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gocraft/dbr/v2 v2.7.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/flatbuffers v2.0.6+incompatible // indirect
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
	github.com/trimble-oss/tierceron-nute v1.0.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel v1.27.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.27.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/genproto v0.0.0-20240617180043-68d350f18fd4 // indirect
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
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.6 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/trimble-oss/tierceron-succinctly v0.0.0-20231202151147-a0fc3a0ba103
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/term v0.21.0
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace github.com/dolthub/go-mysql-server => github.com/trimble-oss/go-mysql-server v0.12.0-1.24

replace github.com/trimble-oss/tierceron/atrium => ./atrium
