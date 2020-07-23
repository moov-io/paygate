## Running PayGate in Production

After you've [gotten started](https://github.com/moov-io/paygate#getting-started) with PayGate and are ready to deploy it in an environment there are a few considerations you need to make. The first consideration will be to ensure you have an agreement setup with a Financial Institution to act as your Originating Depository Financial Institution (ODFI) where files are uploaded to and processed with the Federal Reserve. After you're setup with an ODFI there will be some configuration needed along with deployment of PayGate.

PayGate operates as a first-pary ACH originator, which means it assumes you're operating from the perspective of an ODFI and are debiting or crediting accounts between your FI and another financial institution (FI). If you need to move funds between two FI's which are not your ODFI Moov's commercial solution may better fit your needs. You are responsible to ensure all fund transfers comply with United States, state and local laws, relevant financial requirements and any agreements you have entered into. Moov accepts no responsibility for funds that are transfered by other parties using PayGate.

Moov offers commercial support, and hosting with an ODFI to offer payments for your needs. For more information refer to our [product documentation](https://docs.moov.io/paygate/) for more information.

### Suggestions

Prior to running PayGate in a production environment consider how data replication, process monitoring, networking, and general availability factor into your infrastructure. We have makde some explicit decisions around [high availability](./ha.md) that currently drive PayGate's archecture, but your data has requirements to consider.

We recommend you run [MySQL as the datastore](./config.md#database) along with running Kafak as the [pipeline stream](./config.md#pipeline) and [inbound stream](#TODO). The ODFI storage (`odfi.storage.local.directory`) should be a persistent volume PayGate can rely on for consistent and durable storage.

Bringing your own FI typically requires an Origination agreement with them and brings some Gateway (`FileHeader`) configuration, SFTP credentials, and audit trail setup. Consult your financial institution for more details. We are available to assist in setting configuration options according to your requirements.

### Configuration

Please consult the entire [configuration guide](./config.md) for details on all of PayGate's options.

The following are suggestions we recommend changing from their default for a production deployment.

1. `database`: Deploy and configure a MySQL cluster
1. `odfi`
   1. Setup the `gateway` configuration according to your ODFI's requirements
   1. Setup `cutoffs` to accomidate your ODFI's policies around ACH origination
   1. Setup `inboundPath`, `outboundPath`, and `returnPath` according to your ODFI's remote server
   1. Setup `ftp` or `sftp` credentials with industry standard authentication
1. `transfers`
   1. Setup soft and hard limits to review and prevent unexpected large transfers
1. `pipeline`
   1. Setup `audittrail` recording to persist uploaded ACH files
   1. Setup `stream.kafka` with a replicated Kafka cluster
   1. Setup `notifications` for your teams and ODFI.
      1. Configure `email`, `pagerduty`, and/or `slack` for each ACH file uploaded
         1. Note: PagerDuty notifications are not supported in v0.8, but will be the later releases
1. `validation`
   1. Setup a `microDeposits` source account to fund micro-deposit account validation
1. `customers`
   1. Deploy [Moov Customers](https://github.com/moov-io/customers) with a replicated MySQL cluster
   1. Configure [strong encryption keys](https://github.com/moov-io/customers#account-numbers) for account number storage and transit operations

### Deployment

We recommend PayGate is deployed with Terraform modules or Helm Charts. To deploy with either make sure that tool is installed to the latest version and you follow the steps below:

**Terraform Modules**
Hosted in our moov-io/infra repository we have a [Terraform module for PayGate](https://github.com/moov-io/infra/blob/master/modules/paygate/variables.tf). Please see the variables for required values.

<!--
**Helm Charts**
Hosted in our moov-io/charts repository we have a [Helm chart for PayGate](https://github.com/moov-io/charts/tree/master/charts/paygate). Please fill in `values.yaml` with the required values.
-->

### Monitoring

PayGate emits Prometheus metrics on the admin HTTP server at `/metrics`. These should be scraped and monitored. See our [metrics documentation](./metrics.md) for more information. We advise you setup alerting (typically with [Alertmanager](https://github.com/prometheus/alertmanager)) for your teams.

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://github.com/moov-io/paygate/tree/master/docs/) | Our project documentation available online.
 [Hosted Documentation](https://docs.moov.io/paygate/) | Hosted documentation for enterprise solutions.
 Google Group [moov-users](https://groups.google.com/forum/#!forum/moov-users)| The Moov users Google group is for contributors other people contributing to the Moov project. You can join them without a google account by sending an email to [moov-users+subscribe@googlegroups.com](mailto:moov-users+subscribe@googlegroups.com). After receiving the join-request message, you can simply reply to that to confirm the subscription.
Twitter [@moov_io](https://twitter.com/moov_io)	| You can follow Moov.IO's Twitter feed to get updates on our project(s). You can also tweet us questions or just share blogs or stories.
[GitHub Issue](https://github.com/moov-io) | If you are able to reproduce a problem please open a GitHub Issue under the specific project that caused the error.
[moov-io slack](https://slack.moov.io/) | Join our slack channel (`#paygate`) to have an interactive discussion about the development of the project.
