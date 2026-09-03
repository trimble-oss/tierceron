module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck

go 1.27.0

require (
	github.com/trimble-oss/tierceron-core/v2 v2.11.9
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.9 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace (
	go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.43.0
	go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.43.0
	go.opentelemetry.io/otel/sdk => go.opentelemetry.io/otel/sdk v1.43.0
	go.opentelemetry.io/otel/sdk/metric => go.opentelemetry.io/otel/sdk/metric v1.43.0
	go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.43.0
)
