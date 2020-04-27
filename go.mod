module github.com/moov-io/paygate

go 1.13

replace go4.org v0.0.0-20190430205326-94abd6928b1d => go4.org v0.0.0-20190313082347-94abd6928b1d

require (
	github.com/antihax/optional v1.0.0
	github.com/go-kit/kit v0.10.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/goftp/file-driver v0.0.0-20180502053751-5d604a0fc0c9
	github.com/goftp/server v0.0.0-20190712054601-1149070ae46b
	github.com/gorilla/mux v1.7.4
	github.com/hashicorp/vault/api v1.0.4
	github.com/jaegertracing/jaeger-lib v2.2.0+incompatible
	github.com/jlaffaye/ftp v0.0.0-20200422224957-b9f3ade29122
	github.com/lopezator/migrator v0.3.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/moov-io/accounts v0.4.1
	github.com/moov-io/ach v1.3.1
	github.com/moov-io/base v0.11.0
	github.com/moov-io/customers v0.3.0
	github.com/moov-io/fed v0.5.0
	github.com/opentracing/opentracing-go v1.1.0
	github.com/ory/dockertest/v3 v3.6.0
	github.com/pkg/sftp v1.11.0
	github.com/prometheus/client_golang v1.5.1
	github.com/uber/jaeger-client-go v2.23.0+incompatible
	github.com/uber/jaeger-lib v2.2.0+incompatible
	go.uber.org/atomic v1.6.0 // indirect
	gocloud.dev v0.19.0
	gocloud.dev/secrets/hashivault v0.19.0
	golang.org/x/crypto v0.0.0-20200117160349-530e935923ad
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/text v0.3.2
	gopkg.in/yaml.v2 v2.2.7
)
