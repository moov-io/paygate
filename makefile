PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-[a-zA-Z0-9]*)?)' version.go)

.PHONY: build docker release

build:
	go fmt ./...
	@mkdir -p ./bin/
	CGO_ENABLED=1 go build -o ./bin/paygate github.com/moov-io/paygate/cmd/server/

.PHONY: check
check:
ifeq ($(OS),Windows_NT)
	@echo "Skipping checks on Windows, currently unsupported."
else
	@wget -O lint-project.sh https://raw.githubusercontent.com/moov-io/infra/master/go/lint-project.sh
	@chmod +x ./lint-project.sh
	./lint-project.sh
endif

docker: clean
# Docker image
	docker build --pull -t moov/paygate:$(VERSION) -f Dockerfile .
	docker tag moov/paygate:$(VERSION) moov/paygate:latest
# OpenShift Docker image
	docker build --pull -t quay.io/moov/paygate:$(VERSION) -f Dockerfile-openshift --build-arg VERSION=$(VERSION) .
	docker tag quay.io/moov/paygate:$(VERSION) quay.io/moov/paygate:latest

.PHONY: clean
clean:
ifeq ($(OS),Windows_NT)
	@echo "Skipping cleanup on Windows, currently unsupported."
else
	@rm -rf coverage.txt misspell* staticcheck
	@rm -rf ./bin/ openapi-generator-cli-*.jar paygate.db ./storage/ lint-project.sh
endif

dist: clean build
ifeq ($(OS),Windows_NT)
	CGO_ENABLED=1 GOOS=windows go build -o bin/paygate.exe github.com/moov-io/paygate/cmd/server/
else
	CGO_ENABLED=1 GOOS=$(PLATFORM) go build -o bin/paygate-$(PLATFORM)-amd64 github.com/moov-io/paygate/cmd/server/
endif

release: docker AUTHORS
	go vet ./...
	go test -coverprofile=cover-$(VERSION).out ./...
	git tag -f $(VERSION)

release-push:
	docker push moov/paygate:$(VERSION)
	docker push moov/paygate:latest

quay-push:
	docker push quay.io/moov/paygate:$(VERSION)
	docker push quay.io/moov/paygate:latest

.PHONY: cover-test cover-web
cover-test:
	go test -coverprofile=cover.out ./...
cover-web:
	go tool cover -html=cover.out

start-ftp-server:
	@echo Using ACH files in testdata/ftp-server for FTP server
	docker-compose run ftp

start-sftp-server:
	@echo Using ACH files in testdata/sftp-server for SFTP server
	docker-compose run sftp

# From https://github.com/genuinetools/img
.PHONY: AUTHORS
AUTHORS:
	@$(file >$@,# This file lists all individuals having contributed content to the repository.)
	@$(file >>$@,# For how it is generated, see `make AUTHORS`.)
	@echo "$(shell git log --format='\n%aN <%aE>' | LC_ALL=C.UTF-8 sort -uf)" >> $@