module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdb

go 1.24.4

require (
	github.com/trimble-oss/tierceron-core/v2 v2.7.5
	github.com/trimble-oss/tierceron/atrium v0.0.0-20250609162306-04fdcac49140
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.3 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.72.1 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
)

replace github.com/cespare/xxhash => github.com/joel-rieke/xxhash v1.1.0-patch

replace github.com/trimble-oss/tierceron-core/v2 => ../../../../../../tierceron-core
