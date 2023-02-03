module github.com/nestybox/sysbox-libs/shiftfs

go 1.18

require (
	github.com/nestybox/sysbox-libs/linuxUtils v0.0.0-00010101000000-000000000000
	github.com/nestybox/sysbox-libs/mount v0.0.0-00010101000000-000000000000
	github.com/nestybox/sysbox-libs/utils v0.0.0-00010101000000-000000000000
	github.com/opencontainers/runtime-spec v1.0.2
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
)

require (
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/afero v1.4.1 // indirect
	golang.org/x/text v0.3.4 // indirect
)

replace (
	github.com/nestybox/sysbox-libs/linuxUtils => ../linuxUtils
	github.com/nestybox/sysbox-libs/mount => ../mount
	github.com/nestybox/sysbox-libs/utils => ../utils
)
