## Configuration

PayGate uses a file based config for modifying the default way it operates. Various settings can be changed with this file which is written in [YAML format](https://en.wikipedia.org/wiki/YAML).

Use the command-line flag `-config <filename>` for specifying where to read this file from.

Generic placeholders are defined as follows, but typically real-world examples are used. Brackets indicate that a parameter is optional. For non-list parameters the value is set to the specified default.

* `<address>`: a with scheme, host and port that parses as a URL
* `<base64-string>`: a Base64 encoded string
* `<boolean>`: a boolean that can take the values `true` or `false`
* `<duration>`: a duration matching the regular expression `[0-9]+(ms|[smhdwy])`
* `<filename>`: a valid path in the current working directory
* `<host>`: a valid string consisting of hostname or IP followed by an optional port number
* `<number>`: a 32-bit number, but typically only positive values
* `<scheme>`: a string that can take the values `http` or `https`
* `<string>`: a regular string
* `<secret>`: a regular string that is a secret, such as a password
* `<tmpl-string>`: a string which is template-expanded before usage

### Logging

```yaml
logging:
  # Which format to print logs as.
  # Options: plain or json
  [ format: <string> | default = plain ]
```

### HTTP

```yaml
http:
  # Address for paygate to bind its HTTP server on.
  [ bind_address: ":8082" ]
```

### Admin

```yaml
admin:
  # Address for paygate to bind its admin HTTP server on.
  [ bind_address: ":9092" ]
  [ disable_config_endpoint: false ]
```

### ODFI

```yaml
# ODFI holds all the configuration for sending and retrieving ACH files with
# a financial institution to originate files.
odfi:
  routing_number: "987654320"

  # Gateway holds FileHeader information which the ODFI requires is set
  # on all files uploaded.
  gateway:
    [ origin: <string> ]
    [ origin_name: <string> ]
    [ destination: <string> ]
    [ destination_name: <string> ]

  cutoffs:
    timezone: "America/New_York"
    windows:
      - "16:15" # 4:15pm Eastern

  # These paths point to directories on the remote FTP/SFTP server.
  inbound_path: <filename>
  outbound_path: <filename>
  return_path: <filename>

  # Comma separated list of IP addresses and CIDR ranges where connections
  # are allowed. If this value is non-empty remote servers not within these
  # ranges will not be connected to.
  [ allowed_ips: <string> ]

  # Go template string of filenames for the remote server.
  [ outbound_filename_template: <tmpl-string> ]

  # Configuration for using a remote File Transfer Protocol server
  # for ACH file uploads.
  ftp:
    hostname: <host>
    username: <string>
    [ password: <secret> ]
    [ ca_file: <filename> ]
    [ dial_timeout: <duration> | default = 10s ]
    # Offer EPSV to be used if the FTP server supports it.
    [ disabled_epsv: <boolean> | default = false ]

  # Configuration for using a remote SSH File Transfer Protocol server
  # for ACH file uploads
  sftp:
    hostname: <host>
    username: <string>
    [ password: <secret> ]
    [ client_private_key: <filename> ]
    [ host_public_key: <filename> ]
    [ dial_timeout: <duration> | default = 10s ]
    [ max_connections_per_file: <number> | default = 8 ]
    # Sets the maximum size of the payload, measured in bytes.
    # Try lowering this on "failed to send packet header: EOF" errors.
    [ max_packet_size: <number> | default = 20480 ]

  transfers:
    [ balance_entries: <boolean> | default = false ]
    addendum:
      [ create05: <boolean> | default = false ]

  storage:
    # Should we delete the local directory after processing is finished.
    # Leaving these files around helps debugging, but also exposes customer information.
    [ cleanup_local_directory: <boolean> | default = false ]

    # Should we delete the remote file on an ODFI's server after downloading and processing of each file.
    [ keep_remote_files: <boolean> | default = false ]

    local:
      [ directory: <filename> ]
```

### Pipeline

```yaml
pipeline:
  pre_upload:
    gpg:
      [ key_file: <filename> ]
  output:
    # Which encoding to use when writing ACH files to the remote.
    # Options: base64, encrypted-bytes, nacha
    [ format: <string> | default = nacha]
  merging:
    [ directory: <filename> ]
  stream:
    inmem:
      [ url: <address> ]
    kafka:
      brokers:
        - [ <address> ]
      group: [ <string> ]
      topic: [ <string> ]
  notifications:
    email:
      from: <string>
      to:
        - <string>
      # ConnectionURI is a URI used to connect with a remote SFTP server.
      # This config typically needs to contain enough values to successfully
      # authenticate with the server.
      #  - insecure_skip_verify is an optional parameter for disabling certificate verification
      #
      # Example: smtps://user:pass@localhost:1025/?insecure_skip_verify=true
      connection_uri: <string>
      [ template: <tmpl-string> ]
	  company_name: <string>
    pagerduty:
      [ api_key: <secret> ]
    slack:
      [ api_key: <secret> ]
```

### Validation

```yaml
# In order to validate Accounts and Customers to transfer money PayGate must ensure the accounts
# are valid, customers have access to them and are legally allowed in the US to transfer funds.
#
# Currently micro-deposits (two small credits and a debit of their sum) is the only allowed method
# of account validation.
validation:
  micro_deposits:
    source:
      # ID from the Customers service for the source of micro-deposit funds
      customerID: <string>
      accountID: <string>
    # Description is the default for what appears in the Online Banking
	# system for end-users of PayGate. Per NACHA limits this is restricted
	# to 10 characters.
    [ description: <string> ]
```

### Customers

```yaml
customers:
  # A DNS record responsible for routing us to a Moov Customers instance.
  endpoint: <address>
  accounts:
    decryptor:
      symmetric:
        keyURI: "base64key://<base64-string>"
```
