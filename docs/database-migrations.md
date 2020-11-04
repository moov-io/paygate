## Database Migrations

Migrations allow us to evolve application database schema over time.  When the application starts, it will check for database migrations and run them if needed. Information about the current schema version (the version of latest applied migration) is stored in the `schema_migrations` table.

### Creating a Migration

Migrations are stored as files in the `./migrations` directory.
Content of each file is passed to a database driver for execution. Migration
file should consist of valid SQL queries. 

Migration file name have to follow the format: `{version}_{title}.up.sql`

- `verision` of the migration should be represented as integer with 4 digits (with
leading zeros: e.g., 0007). All migrations are applied upward in order of
increasing version number. You can find examples of different migrations in
[./migrations](./migrations).
- `title` should describe action of the migration, e.g.,
  `create_accounts_table`, `add_name_to_accounts`.

### Embedding Migrations

We use [pkger](https://github.com/markbates/pkger) to embed migration files
into our application. Please, [install
it](https://github.com/markbates/pkger#installation) before you proceed.

Running `make embed-migrations` will generate `cmd/server/pkged.go` file with
encoded content of `/migrations` directory that will be included into
application build. Please, commit generated file to the git repository.
