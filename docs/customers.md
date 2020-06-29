[Moov Customers](/customers) is used by PayGate as a service to ensure created `Customer` and `Account` objects pass Anit-Money Laundering (AML), Know Your Customer (KYC), and other checks required by the US Government.

### Configuration

PayGate has a [config object](../config.md#customers) for connecting to Customers. This also registers a [liveness check](../admin.md#liveness-and-readiness-checks) for continued monitoring as the programs are running.

### OFAC Checks

As required by United States law and NACHA guidelines all transfers are checked against the Office of Foreign Asset Control (OFAC) lists for sanctioned individuals and entities to combat fraud, terrorism and unlawful monetary transfers outside of the United States. PayGate defers to Moov's [Customers](https://github.com/moov-io/customers) service for performing these checks.

### OFAC Searches

Customers performs searches against the OFAC list of entities which US businesses are blocked from doing business with. This list changes frequently with world politics and policies.

PayGate requires customers be in `OFAC` or greater status from Customers in order for `Transfers` to be accepted.

### Disclaimers

Before `Transfer` objects can be created the user needs to accept various legal agreements. Having unaccepted disclaimers will result in `Transfer` creation failing with an error message.
