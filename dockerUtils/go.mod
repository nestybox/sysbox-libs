module github.com/nestybox/sysbox-libs/dockerUtils

go 1.21

toolchain go1.21.3

require (
	github.com/docker/docker v26.0.0+incompatible
	github.com/nestybox/sysbox-libs/utils v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.9.0
)

require (
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.51.0 // indirect
	go.opentelemetry.io/otel v1.26.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.26.0 // indirect
	go.opentelemetry.io/otel/metric v1.26.0 // indirect
	go.opentelemetry.io/otel/sdk v1.26.0 // indirect
	go.opentelemetry.io/otel/trace v1.26.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.0.3 // indirect
)

replace github.com/nestybox/sysbox-libs/utils => ../utils
