SHELL := /bin/bash

.PHONY: test static-analysis security-scan build package test-install test-release validate clean

test:
	@/bin/bash scripts/test.sh

static-analysis:
	@/bin/bash scripts/static-analysis.sh

security-scan:
	@/bin/bash scripts/security-scan.sh

build:
	@/bin/bash scripts/build.sh

package:
	@/bin/bash scripts/package-release.sh

test-install:
	@/bin/bash scripts/test-install.sh

test-release:
	@/bin/bash scripts/test-release.sh

validate:
	@/bin/bash scripts/validate-examples.sh

clean:
	@go clean
	@rm -f coverage.out coverage.html
