module github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdstream

go 1.26.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.0
	github.com/denisenkom/go-mssqldb v0.12.3
	github.com/go-sql-driver/mysql v1.7.2-0.20231213112541-0004702b931d
	github.com/google/uuid v1.6.0
	github.com/katasec/dstream-ingester-mssql v0.0.55
	github.com/linkedin/goavro/v2 v2.14.0
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/segmentio/kafka-go v0.4.49
	github.com/trimble-oss/tierceron-core/v2 v2.9.2
	github.com/vbauerster/mpb/v8 v8.10.2
	github.com/wildbeavers/schema-registry v0.3.0
	github.com/xo/dburl v0.9.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	filippo.io/edwards25519 v1.1.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.3 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.72.1 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdstream => ../trcdstream

replace github.com/katasec/dstream-ingester-mssql => ../../../../../../dstream-ingester-mssql
