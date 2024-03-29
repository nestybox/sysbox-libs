module github.com/nestybox/sysbox-libs/idShiftUtils

go 1.18

require (
	github.com/deckarep/golang-set v1.7.1
	github.com/joshlf/go-acl v0.0.0-20200411065538-eae00ae38531
	github.com/karrick/godirwalk v1.16.1
	github.com/sirupsen/logrus v1.7.0
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
)

require github.com/stretchr/testify v1.4.0 // indirect

require github.com/joshlf/testutil v0.0.0-20170608050642-b5d8aa79d93d // indirect

replace github.com/nestybox/sysbox-libs/utils => ../utils
