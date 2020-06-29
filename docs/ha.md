There has been a decision to not implement High Availability (HA) leadership or clustering for PayGate. Resiliency and availability requirements are recommended on the underlying database and deployment of Moov's stack. This is done for a few reasons:

1. The work involved in proper HA leadership, gossip, and coordination is estimated to be too much currently.
1. PayGate can be configured to collect `Transfer` objects for set of ABA routing numbers.
1. Pulling transfers from the DB in batches to process is easier checkpoint than pushing messages around PayGate instances with stashing/replay during downtime.

Given these assumptions we've chosen to focus PayGate's vertical scaling (add CPUs and memory) instead of clustering. Currently two instances of PayGate might be able to build and upload files for the same destination ABA routing numbers, but that setup is not officially supported. That coordination would rely on the database across multiple readers.

PayGate has two flavors of dependencies "CPU based in-memory" and "REST and database" servers along with a database (SQLite or MySQL).

### Database

The underling database PayGate is using will need to be deployed in an acceptable manor for replication, failure recovery, and backups. SQLite replication (possibly implemented via [rqlite](https://github.com/rqlite/rqlite)) has not been tested, but looks promising.

### Clustering concerns

If we implemented HA for multiple PayGate instances we would likely elect a leader for each routing number configured. This would mean elections and heartbeats for each routing number along with stashing knowledge in PayGate when a leader was unresponsive. The implementation details of that seem a lot higher than adding CPU's at the time of writing.
