module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdstream/seed_setup

go 1.26.3

require (
	github.com/google/uuid v1.6.0
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdstream v0.0.0-00010101000000-000000000000
	github.com/vbauerster/mpb/v8 v8.10.2
)

require (
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/linkedin/goavro/v2 v2.14.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	github.com/trimble-oss/tierceron-core/v2 v2.10.9 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.4 // indirect
	github.com/twmb/franz-go v1.20.7 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.12.0 // indirect
	github.com/wildbeavers/schema-registry v0.3.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/linkedin/goavro => ../goavro

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdstream => ../

replace (
	go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.43.0
	go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.43.0
	go.opentelemetry.io/otel/sdk => go.opentelemetry.io/otel/sdk v1.43.0
	go.opentelemetry.io/otel/sdk/metric => go.opentelemetry.io/otel/sdk/metric v1.43.0
	go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.43.0
)
