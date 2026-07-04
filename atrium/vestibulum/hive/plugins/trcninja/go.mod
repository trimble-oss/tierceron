module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja

go 1.26.4

require (
	github.com/denisenkom/go-mssqldb v0.12.3
	github.com/go-sql-driver/mysql v1.8.1
	github.com/google/uuid v1.6.0
	github.com/linkedin/goavro/v2 v2.14.0
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/trimble-oss/tierceron-core/v2 v2.11.5
	github.com/twmb/franz-go v1.20.5
	github.com/vbauerster/mpb/v8 v8.10.2
	github.com/wildbeavers/schema-registry v0.3.0
	github.com/xo/dburl v0.9.0
)

require (
	filippo.io/edwards25519 v1.1.1 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.6 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.12.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.43.0
	go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.43.0
	go.opentelemetry.io/otel/sdk => go.opentelemetry.io/otel/sdk v1.43.0
	go.opentelemetry.io/otel/sdk/metric => go.opentelemetry.io/otel/sdk/metric v1.43.0
	go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.43.0
)
