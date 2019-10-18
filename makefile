PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-[a-zA-Z0-9]*)?)' version.go)
export GO111MODULE=on

.PHONY: build docker release

build:
	go fmt ./...
	@mkdir -p ./bin/
	CGO_ENABLED=1 go build -o ./bin/paygate github.com/moov-io/paygate/cmd/server/

docker:
	docker build --pull -t moov/paygate:$(VERSION) -f Dockerfile .
	docker tag moov/paygate:$(VERSION) moov/paygate:latest

.PHONY: client
client:
# Versions from https://github.com/OpenAPITools/openapi-generator/releases
	@chmod +x ./openapi-generator
	@rm -rf ./client
	OPENAPI_GENERATOR_VERSION=4.1.3 ./openapi-generator generate -i openapi.yaml -g go -o ./client
	rm -f client/go.mod client/go.sum
	go fmt ./...
	go build github.com/moov-io/ofac/client
	go test ./client

.PHONY: clean
clean:
	@rm -rf ./bin/ openapi-generator-cli-*.jar paygate.db ./storage/

dist: clean client build
ifeq ($(OS),Windows_NT)
	CGO_ENABLED=1 GOOS=windows go build -o bin/paygate-windows-amd64.exe github.com/moov-io/paygate/cmd/server/
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

.PHONY: cover-test cover-web
cover-test:
	go test -coverprofile=cover.out ./...
cover-web:
	go tool cover -html=cover.out

clean-integration:
	docker-compose kill
	docker-compose rm -v -f

test-integration: clean-integration
	docker-compose up -d
	sleep 10
	curl -v http://localhost:9094/data/refresh # hangs until download and parsing completes
	./bin/apitest -local -debug # TravisCI downloads this

start-ftp-server:
	@echo Using ACH files in testdata/ftp-server for FTP server
	@docker run -p 2121:2121 -p 30000-30009:30000-30009 -v $(shell pwd)/testdata/ftp-server:/data moov/fsftp:v0.2.0 -host 0.0.0.0 -root /data -user admin -pass 123456 -passive-ports 30000-30009

start-sftp-server:
	@echo Using ACH files in testdata/sftp-server for SFTP server
	@docker run -p 2222:22 -v $(shell pwd)/testdata/sftp-server:/home/demo/upload atmoz/sftp:latest demo:password:::upload

# From https://github.com/genuinetools/img
.PHONY: AUTHORS
AUTHORS:
	@$(file >$@,# This file lists all individuals having contributed content to the repository.)
	@$(file >>$@,# For how it is generated, see `make AUTHORS`.)
	@echo "$(shell git log --format='\n%aN <%aE>' | LC_ALL=C.UTF-8 sort -uf)" >> $@
