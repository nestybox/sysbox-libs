module github.com/nestybox/sysbox-libs/idMap

go 1.18

require (
	github.com/nestybox/sysbox-libs/linuxUtils v0.0.0-00010101000000-000000000000
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.8.1
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
)

replace github.com/nestybox/sysbox-libs/linuxUtils => ../linuxUtils
