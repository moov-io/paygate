// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

type testSQLRepository struct {
	*sqlRepository

	testDB *database.TestSQLiteDB
}

func (r *testSQLRepository) Close() error {
	r.sqlRepository.Close()
	return r.testDB.Close()
}

func createTestSQLiteRepository(t *testing.T) *testSQLRepository {
	t.Helper()

	db := database.CreateTestSqliteDB(t)
	repo := &sqlRepository{db: db.DB}
	return &testSQLRepository{repo, db}
}

type mockRepository struct {
	configs     []*Config
	cutoffTimes []*CutoffTime
	ftpConfigs  []*FTPConfig
	sftpConfigs []*SFTPConfig

	err error
}

func (r *mockRepository) GetConfigs() ([]*Config, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.configs, nil
}

func (r *mockRepository) upsertConfig(cfg *Config) error {
	return r.err
}

func (r *mockRepository) deleteConfig(routingNumber string) error {
	return r.err
}

func (r *mockRepository) GetCutoffTimes() ([]*CutoffTime, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.cutoffTimes, nil
}

func (r *mockRepository) upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error {
	return r.err
}

func (r *mockRepository) deleteCutoffTime(routingNumber string) error {
	return r.err
}

func (r *mockRepository) GetFTPConfigs() ([]*FTPConfig, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.ftpConfigs, nil
}

func (r *mockRepository) upsertFTPConfigs(routingNumber, host, user, pass string) error {
	return r.err
}

func (r *mockRepository) deleteFTPConfig(routingNumber string) error {
	return r.err
}

func (r *mockRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.sftpConfigs, nil
}

func (r *mockRepository) upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error {
	return r.err
}

func (r *mockRepository) deleteSFTPConfig(routingNumber string) error {
	return r.err
}

func (r *mockRepository) Close() error {
	return r.err
}

func newTestStaticRepository(protocol string) *staticRepository {
	repo := &staticRepository{
		configs:     make(map[string]*Config),
		cutoffTimes: make(map[string]*CutoffTime),
		ftpConfigs:  make(map[string]*FTPConfig),
		sftpConfigs: make(map[string]*SFTPConfig),
		protocol:    protocol,
	}
	repo.populate()
	return repo
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
	// returns sqlRepository (rather than staticRepository)
	r := NewRepository(log.NewNopLogger(), "", repo.db, "")
	if _, ok := r.(*sqlRepository); !ok {
		t.Errorf("got %T", r)
	}
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

func writeFileTransferConfig(t *testing.T, db *sql.DB) {
	t.Helper()

	query := `insert into file_transfer_configs (routing_number, inbound_path, outbound_path, return_path) values ('123456789', 'inbound/', 'outbound/', 'return/');`
	stmt, err := db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
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
}

func TestMySQLFileTransferRepository(t *testing.T) {
	testdb := database.CreateTestMySQLDB(t)

	repo := NewRepository(log.NewNopLogger(), "", testdb.DB, "mysql")
	if _, ok := repo.(*sqlRepository); !ok {
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

	testdb.Close()
}

func TestFileTransferConfigs__maskPassword(t *testing.T) {
	if v := maskPassword(""); v != "**" {
		t.Errorf("got %q", v)
	}
	if v := maskPassword("12"); v != "**" {
		t.Errorf("got %q", v)
	}
	if v := maskPassword("123"); v != "1*3" {
		t.Errorf("got %q", v)
	}
	if v := maskPassword("password"); v != "p******d" {
		t.Errorf("got %q", v)
	}

	out := maskFTPPasswords([]*FTPConfig{{Password: "password"}})
	if len(out) != 1 {
		t.Errorf("got %d ftpConfigs: %v", len(out), out)
	}
	if out[0].Password != "p******d" {
		t.Errorf("got %q", out[0].Password)
	}

	out2 := maskSFTPPasswords([]*SFTPConfig{{Password: "drowssap"}})
	if len(out2) != 1 {
		t.Errorf("got %d sftpConfigs: %v", len(out2), out2)
	}
	if out2[0].Password != "d******p" {
		t.Errorf("got %q", out2[0].Password)
	}
}

func TestFileTransferConfigsHTTP__GetConfigs(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")

	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, err := http.NewRequest("GET", "http://"+svc.BindAddr()+"/configs/uploads", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}
	var response adminConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.CutoffTimes) == 0 || len(response.FileTransferConfigs) == 0 {
		t.Errorf("response.CutoffTimes=%d response.FileTransferConfigs=%d", len(response.CutoffTimes), len(response.FileTransferConfigs))
	}
	if len(response.FTPConfigs) == 0 || len(response.SFTPConfigs) != 0 {
		t.Errorf("response.FTPConfigs=%d response.SFTPConfigs=%d", len(response.FTPConfigs), len(response.SFTPConfigs))
	}
}

func TestLocalFileTransferRepository(t *testing.T) {
	repo := newTestStaticRepository("ftp")
	ftpConfigs, err := repo.GetFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ftpConfigs) != 1 {
		t.Errorf("FTP Configs: %#v", ftpConfigs)
	}
	sftpConfigs, err := repo.GetSFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(sftpConfigs) != 0 {
		t.Errorf("SFTP Configs: %#v", sftpConfigs)
	}

	repo = newTestStaticRepository("sftp")
	ftpConfigs, err = repo.GetFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ftpConfigs) != 0 {
		t.Errorf("FTP Configs: %#v", ftpConfigs)
	}
	sftpConfigs, err = repo.GetSFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(sftpConfigs) != 1 {
		t.Errorf("SFTP Configs: %#v", sftpConfigs)
	}

	// make sure all these return nil
	nyc, _ := time.LoadLocation("America/New_York")
	if err := repo.upsertCutoffTime("", 0, nyc); err != nil {
		t.Error(err)
	}
	if err := repo.deleteCutoffTime(""); err != nil {
		t.Error(err)
	}
	if err := repo.upsertFTPConfigs("", "", "", ""); err != nil {
		t.Error(err)
	}
	if err := repo.deleteFTPConfig(""); err != nil {
		t.Error(err)
	}
	if err := repo.upsertSFTPConfigs("", "", "", "", "", ""); err != nil {
		t.Error(err)
	}
	if err := repo.deleteSFTPConfig(""); err != nil {
		t.Error(err)
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
	check(t, &testSQLRepository{&sqlRepository{sqliteDB.DB}, sqliteDB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &testSQLRepository{sqlRepository: &sqlRepository{mysqlDB.DB}})
}

func testifySqlRepo(repo *sqlRepository) *testSQLRepository {
	return &testSQLRepository{repo, &database.TestSQLiteDB{
		DB: repo.db,
	}}
}

func TestConfigs__UpsertDeleteCutoffTime(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *sqlRepository) {
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
	check(t, &sqlRepository{sqliteDB.DB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &sqlRepository{mysqlDB.DB})
}

func TestConfigs__UpsertDeleteFTPConfigs(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *sqlRepository) {
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
	check(t, &sqlRepository{sqliteDB.DB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &sqlRepository{mysqlDB.DB})
}

func TestConfigs__UpsertDeleteSFTPConfigs(t *testing.T) {
	t.Helper()

	check := func(t *testing.T, repo *sqlRepository) {
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
	check(t, &sqlRepository{sqliteDB.DB})

	// MySQL tests
	mysqlDB := database.CreateTestMySQLDB(t)
	defer mysqlDB.Close()
	check(t, &sqlRepository{mysqlDB.DB})
}

func TestConfigsHTTP_UpsertCutoff(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"cutoff": 1700, "location": "America/New_York"}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/cutoff-times/987654320", body)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	// invalid cutoff
	body = strings.NewReader(`{"cutoff": 0, "location": "America/New_York"}`)
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/cutoff-times/987654320", body)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	// invalid location
	body = strings.NewReader(`{"cutoff": 1700, "location": "invalid"}`)
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/cutoff-times/987654320", body)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}
}

func TestConfigsHTTP_DeleteCutoff(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"cutoff": 1700, "location": "America/New_York"}`)
	req, _ := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/cutoff-times/987654320", body)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	// delete
	req, _ = http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/uploads/cutoff-times/987654320", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP__CutoffErrors(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, _ := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/uploads/cutoff-times/987654320", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// POST is not a valid verb for these routes so expect an error
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP_UpsertFileTransferConfig(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"inboundPath": "incoming/", "outboundPath": "outgoing/", "returnPath": "returns/", "outboundFilenameTemplate": ""}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/file-transfers/121042882", body)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	cfgs, err := repo.GetConfigs()
	if len(cfgs) != 1 || err != nil {
		t.Errorf("cfgs=%#v error=%v", cfgs, err)
	}
	if cfgs[0].RoutingNumber != "121042882" {
		t.Errorf("cfgs[0].RoutingNumber=%s", cfgs[0].RoutingNumber)
	}

	// send no body so expect an error
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/file-transfers/121042882", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP_UpsertOutboundFilenameTemplate(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"inboundPath": "in/", "outboundPath": "out/", "returnPath": "return/", "outboundFilenameTemplate": "{{ date \"20060102\" }}"}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/file-transfers/987654320", body)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	configs, err := repo.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	for i := range configs {
		if configs[i].RoutingNumber == "987654320" {
			if configs[i].OutboundFilenameTemplate != `{{ date "20060102" }}` {
				t.Errorf("template=%v", configs[i].OutboundFilenameTemplate)
			} else {
				return // template matched
			}
		}
	}
	t.Error("never found *Config")
}

func TestConfigsHTTP__FileTransferConfigError(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, err := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/uploads/file-transfers/121042882", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// POST isn't a valid verb for these routes, so expect an error
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigs__deleteFileTransferConfig(t *testing.T) {
	repo := createTestSQLiteRepository(t)

	// nothing, expect no error
	if err := repo.deleteConfig("987654320"); err != nil {
		t.Errorf("expected no error: %v", err)
	}
	if err := repo.deleteConfig(""); err != nil {
		t.Errorf("expected no error: %v", err)
	}
	if err := repo.deleteConfig("invalid"); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestConfigsHTTP_DeleteFileTransferConfig(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	if err := repo.upsertConfig(&Config{
		RoutingNumber:            "121042882",
		InboundPath:              "inbound/",
		OutboundPath:             "outbound/",
		ReturnPath:               "return/",
		OutboundFilenameTemplate: "",
	}); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/uploads/file-transfers/121042882", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	cfgs, err := repo.GetConfigs()
	if len(cfgs) != 0 || err != nil {
		t.Errorf("cfgs=%#v error=%v", cfgs, err)
	}
}

func TestConfigsHTTP_UpsertFTP(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// Update the hostname and username
	body := strings.NewReader(`{"hostname": "ftp-sbx.bank.com", "username": "moovtest"}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/ftp/987654320", body)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	// invalid json body
	body = strings.NewReader(`{"ldkjadaksj": {...}}`)
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/ftp/987654320", body)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	// empty username
	body = strings.NewReader(`{"hostname": "ftp-sbx.bank.com", "username": ""}`)
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/ftp/987654320", body)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP_DeleteFTP(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// write
	body := strings.NewReader(`{"hostname": "ftp-sbx.bank.com", "username": "moovtest"}`)
	req, _ := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/ftp/987654320", body)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	// delete
	req, err = http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/uploads/ftp/987654320", nil)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP__FTPError(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, _ := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/uploads/ftp/987654320", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	// POST is not a valid verb for these endpoints, so expect an error
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP_UpsertSFTP(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// Update the hostname and username
	body := strings.NewReader(`{"hostname": "sftp-sbx.bank.com", "username": "moovtest", "clientPrivateKey": ".."}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/sftp/987654320", body)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	// invalid json body
	body = strings.NewReader(`{"asdkajds": {...}}`)
	req, err = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/sftp/987654320", body)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}

	// empty hostname
	body = strings.NewReader(`{"hostname": "", "username": "moovtest", "clientPrivateKey": ".."}`)
	req, err = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/sftp/987654320", body)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP_DeleteSFTP(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// write record
	body := strings.NewReader(`{"hostname": "sftp-sbx.bank.com", "username": "moovtest", "clientPrivateKey": ".."}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/uploads/sftp/987654320", body)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}

	// delete
	req, err = http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/uploads/sftp/987654320", nil)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfigsHTTP_SFTPError(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := newTestStaticRepository("ftp")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// write record
	req, err := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/uploads/sftp/987654320", nil)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%v: %v", err, time.Now())
	}
	defer resp.Body.Close()

	// POST is not a valid verb for these endpoints, so expect an error
	if resp.StatusCode != http.StatusBadRequest {
		bs, _ := ioutil.ReadAll(resp.Body)
		t.Errorf("bogus HTTP status: %d: %s", resp.StatusCode, string(bs))
	}
}

func TestConfig__newRepositoryFromConfig(t *testing.T) {
	repo, err := newRepositoryFromConfig(filepath.Join("..", "..", "testdata", "configs", "routing-good.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	cfgs, err := repo.GetConfigs()
	if err != nil || len(cfgs) != 1 {
		t.Fatalf("configs=%#v error=%v", cfgs, err)
	}
	if cfgs[0].RoutingNumber != "987654320" || cfgs[0].InboundPath != "ach/inbound/" {
		t.Errorf("cfgs[0]=%#v", cfgs[0])
	}

	cts, err := repo.GetCutoffTimes()
	if err != nil || len(cts) != 1 {
		t.Fatalf("cutoffs=%#v error=%v", cts, err)
	}
	if cts[0].RoutingNumber != "987654320" || cts[0].Loc.String() != "America/New_York" {
		t.Errorf("cts[0]=%#v", cts[0])
	}

	ftpConfigs, err := repo.GetFTPConfigs()
	if err != nil || len(ftpConfigs) != 1 {
		t.Fatalf("ftpConfigs=%#v error=%v", ftpConfigs, err)
	}
	if ftpConfigs[0].Hostname != "ftp.bank.com" {
		t.Errorf("ftpConfigs[0].Hostname=%s", ftpConfigs[0].Hostname)
	}

	sftpConfigs, err := repo.GetSFTPConfigs()
	if err != nil || len(sftpConfigs) != 1 {
		t.Fatalf("sftpConfigs=%#v error=%v", sftpConfigs, err)
	}
	if sftpConfigs[0].Hostname != "sftp.bank.com" {
		t.Errorf("sftpConfigs[0].Hostname=%s", sftpConfigs[0].Hostname)
	}
	if sftpConfigs[0].HostPublicKey != "host-key" {
		t.Errorf("sftpConfigs[0].HostPublicKey=%s", sftpConfigs[0].HostPublicKey)
	}
}
