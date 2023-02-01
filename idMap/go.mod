module github.com/nestybox/sysbox-libs/idMap

go 1.18

require (
	github.com/nestybox/sysbox-libs/utils v0.0.0-00010101000000-000000000000
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.8.1
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
)

require github.com/sirupsen/logrus v1.9.0 // indirect

replace github.com/nestybox/sysbox-libs/utils => ../utils
