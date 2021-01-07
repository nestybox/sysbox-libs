#
# sysbox-ipc Makefile
#

.PHONY: validate listpackages

GO := go

validate:
	script/validate-gofmt

listpackages:
	@echo $(allpackages)

# memoize allpackages, so that it's executed only once and only if used
_allpackages = $(shell $(GO) list ./... | grep -v vendor)
allpackages = $(if $(__allpackages),,$(eval __allpackages := $$(_allpackages)))$(__allpackages)
