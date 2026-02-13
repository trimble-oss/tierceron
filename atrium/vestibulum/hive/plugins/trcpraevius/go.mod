module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcpraevius

go 1.25.7

require (
	github.com/trimble-oss/tierceron-core/v2 v2.10.4
	github.com/trimble-oss/tierceron/atrium v0.0.0-20250907153032-8764a0aa515b
	google.golang.org/grpc v1.72.1
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.3 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
)

//replace github.com/trimble-oss/tierceron-core/v2 => ../../../../../../tierceron-core
