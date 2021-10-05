module github.com/moov-io/paygate

go 1.13

require (
	github.com/Azure/azure-pipeline-go v0.2.3 // indirect
	github.com/PagerDuty/go-pagerduty v1.4.2
	github.com/PuerkitoBio/goquery v1.5.1 // indirect
	github.com/Shopify/sarama v1.30.0
	github.com/antihax/optional v1.0.0
	github.com/go-kit/kit v0.12.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/jaegertracing/jaeger-lib v2.2.0+incompatible // indirect
	github.com/jlaffaye/ftp v0.0.0-20210307004419-5d4190119067
	github.com/lopezator/migrator v0.3.0
	github.com/mattn/go-sqlite3 v2.0.6+incompatible
	github.com/moov-io/ach v1.12.2
	github.com/moov-io/base v0.25.0
	github.com/moov-io/customers v0.5.2
	github.com/moov-io/identity v0.2.7 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0
	github.com/ory/dockertest/v3 v3.8.0
	github.com/ory/mail/v3 v3.0.0
	github.com/pkg/sftp v1.13.4
	github.com/prometheus/client_golang v1.11.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.29.1+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible
	gocloud.dev v0.24.0
	gocloud.dev/pubsub/kafkapubsub v0.24.0
	gocloud.dev/secrets/hashivault v0.20.0 // indirect
	goftp.io/server v0.4.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/oauth2 v0.0.0-20211005180243-6b3c2da341f1
	golang.org/x/text v0.3.7
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
)

replace goftp.io/server v0.4.0 => github.com/adamdecaf/goftp-server v0.4.0
