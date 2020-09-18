USERID := $(shell id -u $$USER)
GROUPID:= $(shell id -g $$USER)

.PHONY: generate
generate: admin client

.PHONY: admin
admin:
ifeq ($(OS),Windows_NT)
	@echo "Please generate ../pkg/admin/ on macOS or Linux, currently unsupported on windows."
else
# Versions from https://github.com/OpenAPITools/openapi-generator/releases
	@chmod +x ./openapi-generator
	@rm -rf ../pkg/admin/
	OPENAPI_GENERATOR_VERSION=4.3.0 ./openapi-generator generate --package-name admin -i openapi-admin.yaml -g go -o ../pkg/admin/
	rm -rf ../pkg/admin/go.mod ../pkg/admin/go.sum ../pkg/admin/api/
	go fmt ../...
	go test ../pkg/admin/
endif

.PHONY: client
client:
	@rm -rf ./pkg/client
	docker run --rm \
        -u $(USERID):$(GROUPID) \
        -v ${PWD}:/local openapitools/openapi-generator-cli:v4.3.1 batch -- /local/.openapi-generator/client-generator-config.yml
	rm -f ./pkg/client/go.mod ./pkg/client/go.sum
	gofmt -w ./pkg/client/
	go build github.com/moov-io/customers/pkg/client