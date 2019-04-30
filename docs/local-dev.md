### Local FTP server

Paygate supports merging and uploading tranfers into ACH files and defaults to a local FTP server. The defaults paygate assumes are defined in `localFileTransferRepository` and you can run an FTP server by installing [`github.com/goftp/server/exampleftpd`](https://github.com/goftp/server/tree/master/exampleftpd) (or similar FTP service).

Running `exampleftpd` can be done with (from paygate's `testdata/ftp-server/` directory):

```
$ cd testdata/

$ exampleftpd -host 0.0.0.0 -root $(pwd) -user admin -pass 123456
2019/04/29 09:07:29 Starting ftp server on 0.0.0.0:2121
2019/04/29 09:07:29 Username admin, Password 123456
2019/04/29 09:07:29   Go FTP Server listening on 2121
```

Note: After processing paygate will delete files in `testdata/ftp-server/inbound/` and `testdata/ftp-server/returned/`. Use `git checkout testdata/ftp-server` to restore those files.
