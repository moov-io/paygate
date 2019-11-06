module github.com/moov-io/paygate

require (
	github.com/antihax/optional v1.0.0
	github.com/go-kit/kit v0.9.0
	github.com/go-sql-driver/mysql v1.4.1
	github.com/goftp/file-driver v0.0.0-20180502053751-5d604a0fc0c9
	github.com/goftp/server v0.0.0-20190712054601-1149070ae46b
	github.com/gorilla/mux v1.7.3
	github.com/jlaffaye/ftp v0.0.0-20191025175106-a59fe673c9b2
	github.com/lopezator/migrator v0.2.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/moov-io/accounts v0.4.1
	github.com/moov-io/ach v1.2.2-0.20191101170628-7c57c87800e3
	github.com/moov-io/base v0.11.0-rc1
	github.com/moov-io/customers v0.3.0-rc1.0.20191106030900-8f02e6bd0e15
	github.com/moov-io/fed v0.4.0
	github.com/ory/dockertest/v3 v3.5.2
	github.com/pkg/sftp v1.10.1
	github.com/prometheus/client_golang v1.2.1
	golang.org/x/crypto v0.0.0-20191029031824-8986dd9e96cf
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/text v0.3.2
	gopkg.in/yaml.v2 v2.2.5
)

go 1.13

replace go4.org v0.0.0-20190430205326-94abd6928b1d => go4.org v0.0.0-20190313082347-94abd6928b1d
