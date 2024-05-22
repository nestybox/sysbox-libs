module github.com/nestybox/sysbox-libs/idShiftUtils

go 1.21

toolchain go1.21.0

require (
	github.com/deckarep/golang-set v1.7.1
	github.com/joshlf/go-acl v0.0.0-20200411065538-eae00ae38531
	github.com/karrick/godirwalk v1.16.1
	github.com/sirupsen/logrus v1.7.0
	golang.org/x/sys v0.19.0
)

require github.com/stretchr/testify v1.4.0 // indirect

require github.com/joshlf/testutil v0.0.0-20170608050642-b5d8aa79d93d // indirect

replace github.com/nestybox/sysbox-libs/utils => ../utils
