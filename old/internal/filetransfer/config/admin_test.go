// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	moovadmin "github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/admin"
	"github.com/moov-io/paygate/internal/database"
)

func TestFileTransferConfigsHTTP__GetConfigs(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")

	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, err := http.NewRequest("GET", "http://"+svc.BindAddr()+"/configs/filetransfers", nil)
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

func TestConfigsHTTP_UpsertCutoff(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"cutoff": 1700, "location": "America/New_York"}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/cutoff-times/987654320", body)
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
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/cutoff-times/987654320", body)

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
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/cutoff-times/987654320", body)

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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"cutoff": 1700, "location": "America/New_York"}`)
	req, _ := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/cutoff-times/987654320", body)

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
	req, _ = http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/filetransfers/cutoff-times/987654320", nil)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, _ := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/filetransfers/cutoff-times/987654320", nil)

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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"inboundPath": "incoming/", "outboundPath": "outgoing/", "returnPath": "returns/", "outboundFilenameTemplate": ""}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/121042882", body)
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
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/121042882", nil)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	body := strings.NewReader(`{"inboundPath": "in/", "outboundPath": "out/", "returnPath": "return/", "outboundFilenameTemplate": "{{ date \"20060102\" }}"}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/987654320", body)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := createTestSQLiteRepository(t)
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, err := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/filetransfers/121042882", nil)
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

func TestConfigsHTTP_DeleteFileTransferConfig(t *testing.T) {
	svc := moovadmin.NewServer(":0")
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

	req, err := http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/filetransfers/121042882", nil)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// Update the hostname and username
	body := strings.NewReader(`{"hostname": "ftp-sbx.bank.com", "username": "moovtest"}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/ftp/987654320", body)
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
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/ftp/987654320", body)

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
	req, _ = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/ftp/987654320", body)

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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// write
	body := strings.NewReader(`{"hostname": "ftp-sbx.bank.com", "username": "moovtest"}`)
	req, _ := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/ftp/987654320", body)

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
	req, err = http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/filetransfers/ftp/987654320", nil)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, _ := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/filetransfers/ftp/987654320", nil)

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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// Update the hostname and username
	body := strings.NewReader(`{"hostname": "sftp-sbx.bank.com", "username": "moovtest", "clientPrivateKey": ".."}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/sftp/987654320", body)
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
	req, err = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/sftp/987654320", body)
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
	req, err = http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/sftp/987654320", body)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// write record
	body := strings.NewReader(`{"hostname": "sftp-sbx.bank.com", "username": "moovtest", "clientPrivateKey": ".."}`)
	req, err := http.NewRequest("PUT", "http://"+svc.BindAddr()+"/configs/filetransfers/sftp/987654320", body)
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
	req, err = http.NewRequest("DELETE", "http://"+svc.BindAddr()+"/configs/filetransfers/sftp/987654320", nil)
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
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	// write record
	req, err := http.NewRequest("POST", "http://"+svc.BindAddr()+"/configs/filetransfers/sftp/987654320", nil)
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

func TestConfig__readConfigFile(t *testing.T) {
	repo, err := readConfigFile(filepath.Join("..", "..", "..", "testdata", "configs", "routing-good.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if xs, _ := repo.GetConfigs(); len(xs) != 1 {
		t.Errorf("got %#v", xs)
	}
	if xs, _ := repo.GetCutoffTimes(); len(xs) != 1 {
		t.Errorf("got %#v", xs)
	}
	if xs, _ := repo.GetFTPConfigs(); len(xs) != 1 {
		t.Errorf("got %#v", xs)
	}
	if xs, _ := repo.GetSFTPConfigs(); len(xs) != 1 {
		t.Errorf("got %#v", xs)
	}
}

func TestConfigHTTP__adminRead(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := NewRepository("", nil, "")
	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	conf := admin.NewConfiguration()
	conf.Host = svc.BindAddr()
	client := admin.NewAPIClient(conf)

	cfg, resp, err := client.AdminApi.GetConfigs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(cfg.CutoffTimes) != 1 || len(cfg.FileTransferConfigs) != 0 {
		t.Errorf("CutoffTimes=%#v FileTransferConfigs=%#v", cfg.CutoffTimes, cfg.FileTransferConfigs)
	}
	if len(cfg.FTPConfigs) != 1 || len(cfg.SFTPConfigs) != 0 {
		t.Errorf("FTPConfigs=%#v SFTPConfigs=%#v", cfg.FTPConfigs, cfg.SFTPConfigs)
	}
}

func TestConfigHTTP_adminUpdateFileTransferConfig(t *testing.T) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	var repo Repository
	if testing.Short() {
		db := database.CreateTestSqliteDB(t)
		defer db.Close()
		repo = NewRepository("", db.DB, "sqlite")
	} else {
		db := database.CreateTestMySQLDB(t)
		defer db.Close()
		repo = NewRepository("", db.DB, "mysql")
	}

	if err := repo.upsertConfig(&Config{
		RoutingNumber: "121042882",
		InboundPath:   "/ach/inbound/",
		OutboundPath:  "/ach/outbound/",
		ReturnPath:    "/ach/return/",
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("before cfg=%#v", readFileTransferConfig(repo, "121042882"))

	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	conf := admin.NewConfiguration()
	conf.Host = svc.BindAddr()
	client := admin.NewAPIClient(conf)

	resp, err := client.AdminApi.UpdateFileTransferConfig(context.Background(), "121042882", admin.FileTransferConfig{
		InboundPath:              "/new/directory/inbound",
		OutboundFilenameTemplate: "file.ach",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %v", resp.Status)
	}

	cfg := readFileTransferConfig(repo, "121042882")
	t.Logf("after cfg=%#v", cfg)
	if cfg.InboundPath != "/new/directory/inbound" {
		t.Errorf("cfg.InboundPath=%#v", cfg.InboundPath)
	}
	if cfg.OutboundPath != "/ach/outbound/" {
		t.Errorf("cfg.OutboundPath=%#v", cfg.OutboundPath)
	}
	if cfg.OutboundFilenameTemplate != "file.ach" {
		t.Errorf("cfg.OutboundFilenameTemplate=%#v", cfg.OutboundFilenameTemplate)
	}
}
