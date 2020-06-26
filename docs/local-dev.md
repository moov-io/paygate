### Local FTP server

PayGate supports merging and uploading transfers into ACH files using the File Transport Protocol (FTP). You can run an FTP server using Moov's `fsftp` Docker image with the command `make start-ftp-server`. This image is [located in moov-io/infra](https://github.com/moov-io/infra/tree/master/images/fsftp).

Running `moov/fsftp` can be done with (from PayGate's `testdata/ftp-server/` directory):

```
$ make start-ftp-server
Using ACH files in testdata/ftp-server for FTP server
2019/04/29 09:07:29 Starting ftp server on 0.0.0.0:2121
2019/04/29 09:07:29 Username admin, Password 123456
2019/04/29 09:07:29   Go FTP Server listening on 2121
```

Note: After processing PayGate will delete files in `testdata/ftp-server/inbound/` and `testdata/ftp-server/returned/`. Use `git checkout testdata/ftp-server` to restore those files.

### Local SFTP server

PayGate supports merging and uploading transfers into ACH files using the SSH File Transport Protocol (SFTP). You can run an SFTP server by running the [`atmoz/sftp`](https://hub.docker.com/r/atmoz/sftp) Docker image with `make start-sftp-server`.

Running `atmoz/sftp` can be done with (from PayGate's `testdata/sftp-server/` directory):

```
$ make start-sftp-server
Using ACH files in testdata/sftp-server for SFTP server
[/usr/local/bin/create-sftp-user] Parsing user data: "demo:password:::upload"
[/usr/local/bin/create-sftp-user] Directory already exists: /home/demo/upload
...
Server listening on 0.0.0.0 port 22.
Server listening on :: port 22.
```

Note: After processing PayGate will delete files in `testdata/sftp-server/inbound/` and `testdata/sftp-server/returned/`. Use `git checkout testdata/sftp-server` to restore those files.

### Running Tests

PayGate has a lot of tests which spin up Docker containers, network calls, and can take a while to run (~30-60s). To skip these tests run `go test ./... -short`.

### Code Coverage

Go offers code coverage reports from testing. You can create one by running `go test ./... -coverprofile=cover.out`. Then `make cover-web` uses the Go tooling to display them in your default browser.
