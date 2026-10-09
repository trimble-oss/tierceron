module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcvico

go 1.27.1

require (
	github.com/townsendmerino/goinfer v0.22.0
	github.com/townsendmerino/goinfer/cuda v0.22.0
	github.com/trimble-oss/tierceron-core/v2 v2.12.3
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk v0.0.0-20260918175252-00c9428a705b
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/eitamring/gocudrv v0.3.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/townsendmerino/aikit v1.57.0 // indirect
	github.com/townsendmerino/aikit/gpu v0.33.5 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.9 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
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
