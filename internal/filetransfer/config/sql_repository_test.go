// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"database/sql"
	"testing"

	"github.com/moov-io/paygate/internal/database"
)

type testSQLRepository struct {
	*SQLRepository

	testDB *database.TestSQLiteDB
}

func (r *testSQLRepository) Close() error {
	r.SQLRepository.Close()
	return r.testDB.Close()
}

func createTestSQLiteRepository(t *testing.T) *testSQLRepository {
	t.Helper()

	db := database.CreateTestSqliteDB(t)
	repo := &SQLRepository{db: db.DB}
	return &testSQLRepository{repo, db}
}

func writeCutoffTime(t *testing.T, repo *testSQLRepository) {
	t.Helper()

	query := `insert into cutoff_times (routing_number, cutoff, location) values ('123456789', 1700, 'America/New_York');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func writeFTPConfig(t *testing.T, repo *testSQLRepository) {
	t.Helper()

	query := `insert into ftp_configs (routing_number, hostname, username, password) values ('123456789', 'ftp.moov.io', 'moov', 'secret');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func writeSFTPConfig(t *testing.T, repo *testSQLRepository) {
	t.Helper()

	query := `insert into sftp_configs (routing_number, hostname, username, password, client_private_key, host_public_key) values ('123456789', 'ftp.moov.io', 'moov', '', '==secret==', '==public==');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func writeFileTransferConfig(t *testing.T, db *sql.DB) {
	t.Helper()

	query := `insert into file_transfer_configs (routing_number, inbound_path, outbound_path, return_path, allowed_ips) values ('123456789', 'inbound/', 'outbound/', 'return/', '127.0.0.0/8');`
	stmt, err := db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteRepository__getCounts(t *testing.T) {
	repo := createTestSQLiteRepository(t)
	defer repo.Close()

	cutoffs, ftps, filexfers := repo.GetCounts()
	if cutoffs != 0 || ftps != 0 || filexfers != 0 {
		t.Errorf("cutoffs=%d ftps=%d filexfers=%d", cutoffs, ftps, filexfers)
	}

	writeCutoffTime(t, repo)
	writeFileTransferConfig(t, repo.db)
	writeFTPConfig(t, repo)

	cutoffs, ftps, filexfers = repo.GetCounts()
	if cutoffs != 1 || ftps != 1 || filexfers != 1 {
		t.Errorf("cutoffs=%d ftps=%d filexfers=%d", cutoffs, ftps, filexfers)
	}

	// If we read at least one row from each config table we need to make sure NewRepository
	// returns SQLRepository
	r := NewRepository("", repo.db, "")
	if _, ok := r.(*SQLRepository); !ok {
		t.Errorf("got %T", r)
	}
}

func TestSQLiteRepository__GetCutoffTimes(t *testing.T) {
	repo := createTestSQLiteRepository(t)
	defer repo.Close()

	writeCutoffTime(t, repo)

	cutoffTimes, err := repo.GetCutoffTimes()
	if err != nil {
		t.Fatal(err)
	}
	if len(cutoffTimes) != 1 {
		t.Errorf("len(cutoffTimes)=%d", len(cutoffTimes))
	}
	if cutoffTimes[0].RoutingNumber != "123456789" {
		t.Errorf("cutoffTimes[0].RoutingNumber=%s", cutoffTimes[0].RoutingNumber)
	}
	if cutoffTimes[0].Cutoff != 1700 {
		t.Errorf("cutoffTimes[0].Cutoff=%d", cutoffTimes[0].Cutoff)
	}
	if v := cutoffTimes[0].Loc.String(); v != "America/New_York" {
		t.Errorf("cutoffTimes[0].Loc=%v", v)
	}
}

func TestSQLiteRepository__GetFTPConfigs(t *testing.T) {
	repo := createTestSQLiteRepository(t)
	defer repo.Close()

	writeFTPConfig(t, repo)

	// now read
	configs, err := repo.GetFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].Hostname != "ftp.moov.io" {
		t.Errorf("got %q", configs[0].Hostname)
	}
	if configs[0].Username != "moov" {
		t.Errorf("got %q", configs[0].Username)
	}
	if configs[0].Password != "secret" {
		t.Errorf("got %q", configs[0].Password)
	}
}

func TestSQLiteRepository__GetConfigs(t *testing.T) {
	repo := createTestSQLiteRepository(t)
	defer repo.Close()

	writeFileTransferConfig(t, repo.db)

	// now read
	configs, err := repo.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].InboundPath != "inbound/" {
		t.Errorf("got %q", configs[0].InboundPath)
	}
	if configs[0].OutboundPath != "outbound/" {
		t.Errorf("got %q", configs[0].OutboundPath)
	}
	if configs[0].ReturnPath != "return/" {
		t.Errorf("got %q", configs[0].ReturnPath)
	}
	if configs[0].AllowedIPs != "127.0.0.0/8" {
		t.Errorf("got %q", configs[0].AllowedIPs)
	}
}

func TestMySQLConfigRepository(t *testing.T) {
	testdb := database.CreateTestMySQLDB(t)

	repo := NewRepository("", testdb.DB, "mysql")
	if _, ok := repo.(*SQLRepository); !ok {
		t.Fatalf("got %T", repo)
	}
	writeFileTransferConfig(t, testdb.DB)

	configs, err := repo.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].InboundPath != "inbound/" {
		t.Errorf("got %q", configs[0].InboundPath)
	}
	if configs[0].OutboundPath != "outbound/" {
		t.Errorf("got %q", configs[0].OutboundPath)
	}
	if configs[0].ReturnPath != "return/" {
		t.Errorf("got %q", configs[0].ReturnPath)
	}
	if configs[0].AllowedIPs != "127.0.0.0/8" {
		t.Errorf("got %q", configs[0].AllowedIPs)
	}

	testdb.Close()
}

func TestConfigs__GetSFTPConfigs(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *testSQLRepository) {
		writeSFTPConfig(t, repo)

		configs, err := repo.GetSFTPConfigs()
		if err != nil {
			t.Fatal(err)
		}
		if len(configs) != 1 {
			t.Errorf("got %d SFTP configs: %#v", len(configs), configs)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &testSQLRepository{&SQLRepository{sqliteDB.DB}, sqliteDB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &testSQLRepository{SQLRepository: &SQLRepository{mysqlDB.DB}})
}

func TestConfigs__UpsertDeleteCutoffTime(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *SQLRepository) {
		writeCutoffTime(t, testifySqlRepo(repo))

		cutoffTimes, err := repo.GetCutoffTimes()
		if err != nil || len(cutoffTimes) != 1 {
			t.Fatalf("got cutoff times: %#v error=%v", cutoffTimes, err)
		}

		// upsert (update or insert)
		ct := cutoffTimes[0]
		if err := repo.upsertCutoffTime(ct.RoutingNumber, ct.Cutoff+100, ct.Loc); err != nil {
			t.Fatal(err)
		}
		cutoffTimes, err = repo.GetCutoffTimes()
		if err != nil || len(cutoffTimes) != 1 {
			t.Fatalf("got cutoff times: %#v error=%v", cutoffTimes, err)
		}

		ct2 := cutoffTimes[0]
		if ct.Cutoff == ct2.Cutoff {
			t.Errorf("ct.Cutoff=%d ct2.Cutoff=%d", ct.Cutoff, ct2.Cutoff)
		}

		// delete
		if err := repo.deleteCutoffTime(ct.RoutingNumber); err != nil {
			t.Fatal(err)
		}
		cutoffTimes, err = repo.GetCutoffTimes()
		if err != nil || len(cutoffTimes) != 0 {
			t.Fatalf("got cutoff times: %#v error=%v", cutoffTimes, err)
		}

		// delete without a row existing
		if err := repo.deleteCutoffTime("987654320"); err != nil {
			t.Errorf("expected no error: %v", err)
		}
		if err := repo.deleteCutoffTime(""); err != nil {
			t.Errorf("expected no error: %v", err)
		}
		if err := repo.deleteCutoffTime("invalid"); err != nil {
			t.Errorf("expected no error: %v", err)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLRepository{sqliteDB.DB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLRepository{mysqlDB.DB})
}

func TestConfigs__UpsertDeleteFTPConfigs(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *SQLRepository) {
		writeFTPConfig(t, testifySqlRepo(repo))

		ftpConfigs, err := repo.GetFTPConfigs()
		if err != nil || len(ftpConfigs) != 1 {
			t.Fatalf("got ftp configs: %#v error=%v", ftpConfigs, err)
		}

		// upsert (update or insert)
		f1 := ftpConfigs[0]
		if err := repo.upsertFTPConfigs(f1.RoutingNumber, "ftp-sbx.bank.com", f1.Username, ""); err != nil {
			t.Fatal(err)
		}
		ftpConfigs, err = repo.GetFTPConfigs()
		if err != nil || len(ftpConfigs) != 1 {
			t.Fatalf("got ftp configs: %v error=%v", ftpConfigs, err)
		}

		f2 := ftpConfigs[0]
		if f1.Hostname == f2.Hostname {
			t.Errorf("f1.Hostname=%s f2.Hostname=%s", f1.Hostname, f2.Hostname)
		}
		if f1.Password != f2.Password {
			t.Errorf("didn't expect password to change: f2.Password=%s", f2.Password)
		}

		// upsert password
		if err := repo.upsertFTPConfigs(f1.RoutingNumber, f2.Hostname, f1.Username, "updated-password"); err != nil {
			t.Fatal(err)
		}
		ftpConfigs, err = repo.GetFTPConfigs()
		if err != nil || len(ftpConfigs) != 1 {
			t.Fatalf("got ftp configs: %v error=%v", ftpConfigs, err)
		}
		f3 := ftpConfigs[0]
		if f2.Password == f3.Password {
			t.Errorf("f2.Password=%s f3.Password=%s", f2.Password, f3.Password)
		}

		// delete
		if err := repo.deleteFTPConfig(f1.RoutingNumber); err != nil {
			t.Fatal(err)
		}
		ftpConfigs, err = repo.GetFTPConfigs()
		if err != nil || len(ftpConfigs) != 0 {
			t.Fatalf("got ftp configs: %v error=%v", ftpConfigs, err)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLRepository{sqliteDB.DB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLRepository{mysqlDB.DB})
}

func TestConfigs__UpsertDeleteSFTPConfigs(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *SQLRepository) {
		writeSFTPConfig(t, testifySqlRepo(repo))

		sftpConfigs, err := repo.GetSFTPConfigs()
		if err != nil || len(sftpConfigs) != 1 {
			t.Fatalf("got sftp configs: %#v error=%v", sftpConfigs, err)
		}

		// upsert (update or insert)
		sf1 := sftpConfigs[0]
		if err := repo.upsertSFTPConfigs(sf1.RoutingNumber, "sftp-sbx.bank.com", sf1.Username, "", "", ""); err != nil {
			t.Fatal(err)
		}
		sftpConfigs, err = repo.GetSFTPConfigs()
		if err != nil || len(sftpConfigs) != 1 {
			t.Fatalf("got sftp configs: %v error=%v", sftpConfigs, err)
		}

		sf2 := sftpConfigs[0]
		if sf1.Hostname == sf2.Hostname {
			t.Errorf("sf1.Hostname=%s sf2.Hostname=%s", sf1.Hostname, sf2.Hostname)
		}

		// upsert Password and ClientPrivateKey and HostPublicKey
		if err := repo.upsertSFTPConfigs(sf1.RoutingNumber, sf2.Hostname, sf2.Username, "new-password", "client-private-key", "host-public-key"); err != nil {
			t.Fatal(err)
		}
		sftpConfigs, err = repo.GetSFTPConfigs()
		if err != nil || len(sftpConfigs) != 1 {
			t.Fatalf("got sftp configs: %v error=%v", sftpConfigs, err)
		}

		sf3 := sftpConfigs[0]
		if sf2.Password == sf3.Password {
			t.Errorf("sf2.Password=%s sf3.Password=%s", sf2.Password, sf3.Password)
		}
		if sf2.ClientPrivateKey == sf3.ClientPrivateKey {
			t.Errorf("sf2.ClientPrivateKey=%s sf3.ClientPrivateKey=%s", sf2.ClientPrivateKey, sf3.ClientPrivateKey)
		}
		if sf2.HostPublicKey == sf3.HostPublicKey {
			t.Errorf("sf2.HostPublicKey=%s sf3.HostPublicKey=%s", sf2.HostPublicKey, sf3.HostPublicKey)
		}

		// delete
		if err := repo.deleteSFTPConfig(sf1.RoutingNumber); err != nil {
			t.Fatal(err)
		}
		sftpConfigs, err = repo.GetSFTPConfigs()
		if err != nil || len(sftpConfigs) != 0 {
			t.Fatalf("got sftp configs: %v error=%v", sftpConfigs, err)
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &SQLRepository{sqliteDB.DB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &SQLRepository{mysqlDB.DB})
}

func testifySqlRepo(repo *SQLRepository) *testSQLRepository {
	return &testSQLRepository{repo, &database.TestSQLiteDB{
		DB: repo.db,
	}}
}
