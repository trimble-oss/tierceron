module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis

go 1.25.2

require (
	github.com/hanwen/go-fuse/v2 v2.7.2
	github.com/trimble-oss/tierceron v1.45.3
	github.com/trimble-oss/tierceron-core/v2 v2.8.8
	google.golang.org/grpc v1.72.1
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.3 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
)

exclude (
	gopkg.in/square/go-jose.v2 v2.3.1
	gopkg.in/square/go-jose.v2 v2.4.0
	gopkg.in/square/go-jose.v2 v2.4.1
	gopkg.in/square/go-jose.v2 v2.5.1
	gopkg.in/square/go-jose.v2 v2.6.0
)

replace gopkg.in/square/go-jose.v2 => github.com/go-jose/go-jose/v3 v3.0.4
