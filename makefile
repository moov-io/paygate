VERSION := $(shell grep -Eo '(v[0-9]+[\.][0-9]+[\.][0-9]+(-dev)?)' version.go)

.PHONY: build docker release

build:
	go fmt ./...
	CGO_ENABLED=1 go build -o bin/paygate .

docker:
	docker build -t moov/paygate:$(VERSION) -f Dockerfile .
	docker tag moov/paygate:$(VERSION) moov/paygate:latest

release: docker
	go vet ./...
	go test ./...
	git tag -f $(VERSION)

release-push:
#	echo "$DOCKER_PASSWORD" | docker login -u wadearnold --password-stdin
#	git push origin $(VERSION)
	docker push moov/paygate:$(VERSION)
