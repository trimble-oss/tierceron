module github.com/trimble-oss/tierceron/atrium/vestibulum/trcchatproxy

go 1.25.1

require (
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.5 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.72.1 // indirect
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/trimble-oss/tierceron v1.45.3
	github.com/trimble-oss/tierceron-core/v2 v2.8.6
	github.com/trimble-oss/tierceron-nute v1.0.10
	github.com/trimble-oss/tierceron-nute-core v1.0.3
	google.golang.org/api v0.226.0
)

require (
	cloud.google.com/go/auth v0.15.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/frankban/quicktest v1.14.6 // indirect
	github.com/go-jose/go-jose/v3 v3.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/vault/api v1.1.0 // indirect
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/pavlo-v-chernykh/keystore-go/v4 v4.4.1 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/term v0.32.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250409194420-de1ac958c67a // indirect
	gopkg.in/square/go-jose.v2 v2.0.0-00010101000000-000000000000 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

// replace github.com/trimble-oss/tierceron-nute => github.com/trimble-oss/tierceron-nute v0.0.0-20231215170009-f41bd163ccda

replace github.com/dolthub/go-mysql-server => github.com/trimble-oss/go-mysql-server v0.12.0-1.30

exclude (
	gopkg.in/square/go-jose.v2 v2.3.1
	gopkg.in/square/go-jose.v2 v2.4.0
	gopkg.in/square/go-jose.v2 v2.4.1
	gopkg.in/square/go-jose.v2 v2.5.1
)

replace gopkg.in/square/go-jose.v2 => github.com/go-jose/go-jose/v3 v3.0.4
