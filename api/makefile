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
ifeq ($(OS),Windows_NT)
	@echo "Please generate ../pkg/client/ on macOS or Linux, currently unsupported on windows."
else
# Versions from https://github.com/OpenAPITools/openapi-generator/releases
	@chmod +x ./openapi-generator
	@rm -rf ../pkg/client/
	OPENAPI_GENERATOR_VERSION=4.3.0 ./openapi-generator generate --package-name client -i openapi.yaml -g go -o ../pkg/client/
	rm -rf ../pkg/client/go.mod ../pkg/client/go.sum ../pkg/client/api/
	go fmt ../...
	go test ../pkg/client/
endif
