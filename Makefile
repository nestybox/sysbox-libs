#
# sysbox-ipc Makefile
#

.PHONY: lint listpackages

GO := go

lint:
	@for d in $(_allgodirs); do \
	   cd $$d; \
	   $(GO) vet ./...; \
	   $(GO) fmt ./...; \
	   cd ..; \
	done

listpackages:
	@echo $(allpackages)

# memoize allpackages, so that it's executed only once and only if used
_allgodirs = $(shell $(GO) list ./... | xargs basename -s)
_allpackages = $(shell for d in $(_allgodirs); do cd $$d && $(GO) list ./... | grep -v vendor; cd ..; done)
allpackages = $(if $(__allpackages),,$(eval __allpackages := $$(_allpackages)))$(__allpackages)
