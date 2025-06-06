module github.com/trimble-oss/tierceron

go 1.24.3

require (
	github.com/denisenkom/go-mssqldb v0.12.0
	github.com/dolthub/go-mysql-server v0.19.0
	github.com/go-sql-driver/mysql v1.7.2-0.20231213112541-0004702b931d
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.7.0
	github.com/hashicorp/vault/api v1.1.0
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267 // indirect; IMPORTANT! This must match vault sdk used by vault for plugin to be stable!
	github.com/julienschmidt/httprouter v1.3.0
	github.com/rs/cors v1.7.0
	github.com/sergi/go-diff v1.2.0
	github.com/twitchtv/twirp v5.12.1+incompatible
	github.com/xo/dburl v0.9.0
	golang.org/x/crypto v0.38.0
	golang.org/x/sys v0.33.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/pavlo-v-chernykh/keystore-go/v4 v4.4.1
	github.com/sendgrid/rest v2.6.9+incompatible
	github.com/sendgrid/sendgrid-go v3.12.0+incompatible
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.8.2
	github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry v0.2.3
	github.com/docker/docker v26.1.5+incompatible
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/graphql-go/graphql v0.8.1
	github.com/trimble-oss/tierceron-core/v2 v2.5.12
	github.com/trimble-oss/tierceron-hat v1.2.9
	github.com/trimble-oss/tierceron/atrium v0.0.0-20250527165913-d4b4a62b8377
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdb v0.0.0-20250520001105-4ddb7c61ec12
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra v0.0.0-20250418225747-d9d4ce87f4c0
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck v0.0.0-20241220234051-2d8c369c5b69
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea v0.0.0-20250418225747-d9d4ce87f4c0
	github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcspiralis v0.0.0-20250418225747-d9d4ce87f4c0
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78
	gopkg.in/yaml.v3 v3.0.1
	kernel.org/pub/linux/libs/security/libcap/cap v1.2.70
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.1 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/klauspost/reedsolomon v1.12.4 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lafriks/go-shamir v1.2.0 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/templexxx/cpu v0.1.1 // indirect
	github.com/templexxx/xorsimd v0.4.3 // indirect
	github.com/xtaci/kcp-go/v5 v5.6.16 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	fyne.io/fyne/v2 v2.5.2 // indirect
	fyne.io/systray v1.11.0 // indirect
	gioui.org v0.8.0 // indirect
	gioui.org/shader v1.0.8 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2 v2.1.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/daviddengcn/go-colortext v1.0.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dolthub/vitess v0.0.0-20241211024425-b00987f7ba54 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/faiface/mainthread v0.0.0-20171120011319-8b78f0a41ae3 // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fredbi/uri v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/ftbe/dawg v0.0.0-20131228112149-aadae8139481 // indirect
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/fyne-io/gl-js v0.0.0-20220119005834-d2da28d9ccfe // indirect
	github.com/fyne-io/glfw-js v0.0.0-20240101223322-6e1efdc71b7a // indirect
	github.com/fyne-io/image v0.0.0-20220602074514-4956b0afb3d2 // indirect
	github.com/g3n/engine v0.2.0 // indirect
	github.com/getkin/kin-openapi v0.131.0 // indirect
	github.com/glycerine/bchan v0.0.0-20170210221909-ad30cd867e1c // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-git/go-billy/v5 v5.6.1 // indirect
	github.com/go-gl/gl v0.0.0-20211210172815-726fda9656d6 // indirect
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20240506104042-037f3cc74f2a // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-text/render v0.2.0 // indirect
	github.com/go-text/typesetting v0.2.1 // indirect
	github.com/gocraft/dbr/v2 v2.7.2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/flatbuffers v2.0.6+incompatible // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jeandeaual/go-locale v0.0.0-20240223122105-ce5225dcaa49 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jsummers/gobmp v0.0.0-20151104160322-e2ba15ffa76e // indirect
	github.com/lestrrat-go/strftime v1.0.4 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/newrelic/go-agent/v3 v3.35.0 // indirect
	github.com/newrelic/go-agent/v3/integrations/logcontext-v2/logWriter v1.0.1 // indirect
	github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrwriter v1.0.0 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.4.0 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/rymdport/portal v0.2.6 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/cobra v1.6.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/trimble-oss/tierceron-nute v1.0.10 // indirect
	github.com/trimble-oss/tierceron-nute-core v1.0.3 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	github.com/yuin/goldmark v1.7.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	golang.org/x/exp v0.0.0-20250215185904-eff6e970281f // indirect
	golang.org/x/exp/shiny v0.0.0-20240707233637-46b078467d37 // indirect
	golang.org/x/image v0.21.0 // indirect
	golang.org/x/mobile v0.0.0-20231127183840-76ac6878050a // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/oauth2 v0.26.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
	google.golang.org/genproto v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250528174236-200df99c418a // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/src-d/go-errors.v1 v1.0.0 // indirect
	gotest.tools/v3 v3.5.1 // indirect
	k8s.io/api v0.26.1 // indirect
	k8s.io/apimachinery v0.26.1 // indirect
	k8s.io/cli-runtime v0.26.1 // indirect
	k8s.io/client-go v0.26.1 // indirect
	k8s.io/component-base v0.26.1 // indirect
	k8s.io/component-helpers v0.26.1 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230123231816-1cb3ae25d79a // indirect
	k8s.io/kubectl v0.26.1 // indirect
	k8s.io/metrics v0.26.1 // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.70 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kustomize/v4 v4.5.7 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

require (
	github.com/aws/aws-sdk-go v1.43.30
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang-sql/sqlexp v0.0.0-20170517235910-f1bb20e5a188 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/trimble-oss/tierceron-succinctly v0.0.0-20231202151147-a0fc3a0ba103
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/term v0.32.0
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	google.golang.org/grpc v1.72.1
	google.golang.org/protobuf v1.36.6
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace github.com/dolthub/vitess => github.com/dolthub/vitess v0.0.0-20221121184553-8d519d0bbb91

replace github.com/dolthub/go-mysql-server => github.com/trimble-oss/go-mysql-server v0.12.0-1.26

//replace github.com/square/go-jose.v2 => ../go-jose

replace github.com/trimble-oss/tierceron/atrium => ./atrium

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck@v0.0.0-20241220234051-2d8c369c5b69 ./atrium/vestibulum/hive/plugins/trchealthcheck

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra => ./atrium/vestibulum/hive/plugins/trcfenestra

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra/hcore => ./atrium/vestibulum/hive/plugins/trcfenestra/hcore

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck => ./atrium/vestibulum/hive/plugins/trchealthcheck

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore => ./atrium/vestibulum/hive/plugins/trchealthcheck/hcore

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcspiralis => ./atrium/vestibulum/hive/plugins/trcspiralis

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcspiralis/hcore => ./atrium/vestibulum/hive/plugins/trcspiralis/hcore

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea => ./atrium/vestibulum/hive/plugins/trcrosea

replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdb => ./atrium/vestibulum/hive/plugins/trcdb

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore => ./atrium/vestibulum/hive/plugins/trcrosea/hcore

//replace github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis => ./atrium/vestibulum/hive/plugins/atrium/vestibulum/hive/plugins/trcmutabilis

//replace github.com/trimble-oss/tierceron-hat => ../tierceron-hat

//replace github.com/trimble-oss/tierceron-core/v2 => ../tierceron-core
