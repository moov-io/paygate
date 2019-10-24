### Local FTP server

Paygate supports merging and uploading tranfers into ACH files and defaults to a local File Transport Protocol (FTP) server. The defaults paygate assumes are defined in `staticRepository` and you can run an FTP server by running Moov's `fsftp` Docker image with `make start-ftp-server`. This image is [located in moov-io/infra](https://github.com/moov-io/infra/tree/master/images/fsftp).

Running `moov/fsftp` can be done with (from paygate's `testdata/ftp-server/` directory):

```
$ make start-ftp-server
Using ACH files in testdata/ftp-server for FTP server
2019/04/29 09:07:29 Starting ftp server on 0.0.0.0:2121
2019/04/29 09:07:29 Username admin, Password 123456
2019/04/29 09:07:29   Go FTP Server listening on 2121
```

Note: After processing paygate will delete files in `testdata/ftp-server/inbound/` and `testdata/ftp-server/returned/`. Use `git checkout testdata/ftp-server` to restore those files.

### Local SFTP server

Paygate supports merging and uploading tranfers into ACH files and defaults to a local SSH File Transport Protocol (SFTP) server. The defaults paygate assumes are defined in `staticRepository` and you can run an SFTP server by running the [`atmoz/sftp`](https://hub.docker.com/r/atmoz/sftp) Docker image with `make start-sftp-server`.

Running `atmoz/sftp` can be done with (from paygate's `testdata/sftp-server/` directory):

```
$ make start-sftp-server
$ make start-sftp-server
Using ACH files in testdata/sftp-server for SFTP server
[/usr/local/bin/create-sftp-user] Parsing user data: "demo:password:::upload"
[/usr/local/bin/create-sftp-user] Directory already exists: /home/demo/upload
...
Server listening on 0.0.0.0 port 22.
Server listening on :: port 22.
```

Note: After processing paygate will delete files in `testdata/sftp-server/inbound/` and `testdata/sftp-server/returned/`. Use `git checkout testdata/sftp-server` to restore those files.
