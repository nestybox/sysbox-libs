module github.com/nestybox/sysbox-libs/shiftfs

go 1.21

require (
	github.com/nestybox/sysbox-libs/linuxUtils v0.0.0-00010101000000-000000000000
	github.com/nestybox/sysbox-libs/mount v0.0.0-00010101000000-000000000000
	github.com/nestybox/sysbox-libs/utils v0.0.0-00010101000000-000000000000
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/sirupsen/logrus v1.9.0
	golang.org/x/sys v0.20.0
	gopkg.in/hlandau/service.v1 v1.0.7
)

require (
	github.com/spf13/afero v1.4.1 // indirect
	golang.org/x/text v0.3.8 // indirect
)

replace (
	github.com/nestybox/sysbox-libs/linuxUtils => ../linuxUtils
	github.com/nestybox/sysbox-libs/mount => ../mount
	github.com/nestybox/sysbox-libs/utils => ../utils
)
