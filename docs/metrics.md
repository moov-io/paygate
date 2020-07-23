## Prometheus Metrics

PayGate emits Prometheus metrics on the admin HTTP server at `/metrics`. Typically [Alertmanager](https://github.com/prometheus/alertmanager) is setup to aggregate the metrics and alert teams.

### HTTP Server

- `http_response_duration_seconds`: Histogram representing the http response durations

### Database

- `mysql_connections`: How many MySQL connections and what status they're in.
- `sqlite_connections`: How many sqlite connections and what status they're in.

### Inbound Files

- `correction_codes_processed`: Counter of correction (COR/NOC) files processed
- `files_downloaded`: Counter of files downloaded from a remote server
- `missing_return_transfers`: Counter of return EntryDetail records handled without a found transfer
- `prenote_entries_processed`: Counter of prenote EntryDetail records processed
- `return_entries_processed`: Counter of return EntryDetail records processed

### Remote File Servers

- `ftp_agent_up`: Status of FTP agent connection
- `sftp_agent_up`: Status of SFTP agent connection
