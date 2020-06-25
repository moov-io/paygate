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
  [ format: <string> | default = "plain" ]
```

### HTTP

```yaml
http:
  # Address for paygate to bind its HTTP server on.
  [ bind_address: <string> | default = ":8082" ]
```

### Admin

```yaml
admin:
  # Address for paygate to bind its admin HTTP server on.
  [ bind_address: <strong> | default = ":9092" ]
  [ disable_config_endpoint: <boolean> | default = false ]
```

### Database

```yaml
database:
  # Setup a SQLite connection for the database. If using this config all fields are required.
  sqlite:
    [ path: <filename> ]
  # Setup a MySQL connection for the database. If using this config all fields are required.
  mysql:
    [ address: <address> ]
    [ username: <string> ]
    [ password: <secret> ]
    [ database: <string> ]
```

### ODFI

```yaml
# ODFI holds all the configuration for sending and retrieving ACH files with
# a financial institution to originate files.
odfi:
  # The ABA routing number to use and run PayGate under.
  # Example: 987654320
  routing_number: <string>

  # Gateway holds FileHeader information which the ODFI requires is set
  # on all files uploaded.
  gateway:
    [ origin: <string> ]
    [ origin_name: <string> ]
    [ destination: <string> ]
    [ destination_name: <string> ]

  cutoffs:
    # An IANA Timezone used to determine when to upload ACH files to the ODFI.
    # Time fields in ACH files are created in this timezone.
    # Example: America/New_York
    timezone: <string>
    # Array of 24-hour and minute timestamps when to initiate cutoff processing.
    # Example: 16:15
    windows:
      - <string>

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
    # Should we delete the local temporary directory after inbound processing is finished.
    # Leaving these files around helps debugging, but also exposes customer information.
    # Empty directories are deleted and if no files are downloaded the entire temporary
    # directory is removed.
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
      signer:
        [ key_file: <filename> ]
        # Optional password to decrypt this private key.
        # It can also be set with PIPELINE_SIGNING_KEY_PASSWORD as an environment variable
        [ key_password: <secret> ]
  output:
    # Which encoding to use when writing ACH files to the remote.
    # Options: base64, encrypted-bytes, nacha
    [ format: <string> | default = "nacha" ]
  merging:
    [ directory: <filename> ]
  audit_trail:
    # BucketURI is a URI used to connect to a remote storage layer for saving
    # ACH files uploaded to the ODFI as part of records retention.
    # See the provider docs for more information: https://gocloud.dev/howto/blob/
    #
    # Example: gs://my-bucket
    bucket_uri: <string>
    gpg:
      # Optional filepath used for encrypting ACH files when they're saved for auditing
      [ key_file: <filename> ]
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
      [ webhook_url: <secret> ]
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
        # Base64 encoded URI for encryption key to use
        # Example: base64key://<base64-string>
        keyURI: <string>
  [ debug: <boolean> | default = false ]
```
