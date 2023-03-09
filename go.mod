module github.com/trimble-oss/tierceron

go 1.20

require (
	VaultConfig.Bootstrap v0.0.0-00010101000000-000000000000
	VaultConfig.TenantConfig v0.0.0-00010101000000-000000000000
	github.com/denisenkom/go-mssqldb v0.12.0
	github.com/dolthub/go-mysql-server v0.12.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.9
	github.com/hashicorp/vault-plugin-secrets-kv v0.9.0
	github.com/hashicorp/vault/api v1.1.0
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267 // IMPORTANT! This must match vault sdk used by vault for plugin to be stable!
	github.com/julienschmidt/httprouter v1.3.0
	github.com/rs/cors v1.7.0
	github.com/sergi/go-diff v1.2.0
	github.com/twitchtv/twirp v5.12.1+incompatible
	github.com/xo/dburl v0.9.0
	golang.org/x/crypto v0.5.0
	golang.org/x/sys v0.5.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	VaultConfig.Test v0.0.0-00010101000000-000000000000
	github.com/g3n/engine v0.2.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	//github.com/mrjrieke/nute v0.0.0-20221220150022-370bea61eb44
	github.com/pavlo-v-chernykh/keystore-go/v4 v4.4.0
	github.com/sendgrid/rest v2.6.9+incompatible
	github.com/sendgrid/sendgrid-go v3.12.0+incompatible
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a

)

require github.com/mrjrieke/nute v0.0.0-20230128181737-65043c9e434b

require (
	github.com/dsnet/golib/memfile v1.0.0
	github.com/graphql-go/graphql v0.8.1-0.20220614210743-09272f350067
	github.com/trimble-oss/tierceron-hat v0.0.0-00010101000000-000000000000
	k8s.io/api v0.26.1
	k8s.io/apimachinery v0.26.1
	k8s.io/cli-runtime v0.26.1
	k8s.io/client-go v0.26.1
	k8s.io/component-base v0.26.1
	k8s.io/klog/v2 v2.80.1
	k8s.io/kubectl v0.26.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/daviddengcn/go-colortext v1.0.0 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spf13/cobra v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	golang.org/x/oauth2 v0.0.0-20221014153046-6fdb5e3db783 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/component-helpers v0.0.0-20230215120307-78f7b9c080f0 // indirect
	k8s.io/kube-openapi v0.0.0-20230123231816-1cb3ae25d79a // indirect
	k8s.io/metrics v0.0.0-20230215135002-90fa97165491 // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kustomize/v4 v4.5.7 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

require (
	github.com/fyne-io/image v0.0.0-20220602074514-4956b0afb3d2 // indirect
	github.com/gocraft/dbr/v2 v2.7.2 // indirect
	github.com/google/flatbuffers v2.0.6+incompatible // indirect
	//	github.com/graphql-go/graphql     v0.0.0- // indirect
	github.com/jsummers/gobmp v0.0.0-20151104160322-e2ba15ffa76e // indirect
	github.com/tevino/abool v1.2.0 // indirect
	go.opentelemetry.io/otel v1.10.0 // indirect
	go.opentelemetry.io/otel/trace v1.10.0 // indirect
	golang.org/x/exp/shiny v0.0.0-20230202163644-54bba9f4231b // indirect
)

require (
	fyne.io/fyne/v2 v2.1.3
	fyne.io/systray v1.10.1-0.20220621085403-9a2652634e93 // indirect
	gioui.org v0.0.0-20220318070519-8833a6738a3b // indirect
	gioui.org/cpu v0.0.0-20210817075930-8d6a761490d2 // indirect
	gioui.org/shader v1.0.6 // indirect
	github.com/benoitkugler/textlayout v0.0.10 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fredbi/uri v0.0.0-20181227131451-3dcfdacbaaf3 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/fyne-io/gl-js v0.0.0-20220119005834-d2da28d9ccfe // indirect
	github.com/fyne-io/glfw-js v0.0.0-20220120001248-ee7290d23504 // indirect
	github.com/gioui/uax v0.2.1-0.20220325163150-e3d987515a12 // indirect
	github.com/go-gl/gl v0.0.0-20211210172815-726fda9656d6 // indirect
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20211213063430-748e38ca8aec // indirect
	github.com/go-text/typesetting v0.0.0-20220112121102-58fe93c84506 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/goki/freetype v0.0.0-20181231101311-fa8a33aabaff // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/srwiley/oksvg v0.0.0-20200311192757-870daf9aa564 // indirect
	github.com/srwiley/rasterx v0.0.0-20200120212402-85cb7272f5e9 // indirect
	github.com/stretchr/testify v1.8.1 // indirect
	github.com/yuin/goldmark v1.4.13 // indirect
	golang.org/x/image v0.5.0 // indirect
	golang.org/x/mobile v0.0.0-20220307220422-55113b94f09c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/js/dom v0.0.0-20210725211120-f030747120f2 // indirect
)

require (
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/aws/aws-sdk-go v1.43.30
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/dolthub/vitess v0.0.0-20220930181015-759b1acd9188
	github.com/fatih/color v1.7.0 // indirect
	github.com/frankban/quicktest v1.14.4 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang-sql/sqlexp v0.0.0-20170517235910-f1bb20e5a188 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-hclog v1.0.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-kms-wrapping/entropy v0.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-plugin v1.4.3 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.6 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/yamux v0.0.0-20190923154419-df201c70410d // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/lestrrat-go/strftime v1.0.4 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/term v0.5.0 // indirect
	golang.org/x/text v0.7.0
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	golang.org/x/tools v0.4.0 // indirect
	google.golang.org/genproto v0.0.0-20221118155620-16455021b5e6 // indirect
	google.golang.org/grpc v1.52.3
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/src-d/go-errors.v1 v1.0.0 // indirect
)

replace VaultConfig.Bootstrap => ../VaultConfig.Bootstrap

replace VaultConfig.TenantConfig => ../VaultConfig.TenantConfig

replace VaultConfig.Test => ../VaultConfig.Test

replace github.com/trimble-oss/tierceron-hat => ../tierceron-hat

replace github.com/dolthub/go-mysql-server => github.com/trimble-oss/go-mysql-server v0.12.0-1.6

// replace github.com/mrjrieke/nute => github.com/trimble-oss/tierceron-nute

replace github.com/g3n/engine v0.2.0 => github.com/mrjrieke/engine v0.2.1-0.20220803142437-5cc7bcf0b99d

replace fyne.io/fyne/v2 v2.1.3 => github.com/mrjrieke/fyne/v2 v2.1.3-6

replace gioui.org v0.0.0-20220318070519-8833a6738a3b => github.com/mrjrieke/gio v0.0.0-20220406132257-ec1380c11ef0

replace github.com/fyne-io/glfw-js v0.0.0-20220120001248-ee7290d23504 => github.com/mrjrieke/glfw-js v0.0.0-20220409154018-95a896685cdb

// replace k8s.io/client-go v0.26.1 => github.com/trimble-oss/client-go v0.0.3

// replace k8s.io/cli-runtime v0.26.1 => github.com/trimble-oss/cli-runtime v0.0.6

// replace k8s.io/kubectl v0.26.1 => github.com/trimble-oss/kubectl v0.0.5

//Don't forget to update pipelines with the right version.

replace k8s.io/client-go v0.26.1 => ../client-go

replace k8s.io/cli-runtime v0.26.1 => ../cli-runtime

replace k8s.io/kubectl v0.26.1 => ../kubectl

replace k8s.io/api v0.26.1 => k8s.io/api v0.0.0-20230228090259-b5b22ca1babf
