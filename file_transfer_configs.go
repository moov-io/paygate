// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type fileTransferRepository interface {
	getCutoffTimes() ([]*cutoffTime, error)
	getFTPConfigs() ([]*ftpConfig, error)
	getFileTransferConfigs() ([]*fileTransferConfig, error)

	close() error
}

func newFileTransferRepository(db *sql.DB, dbType string) fileTransferRepository {
	if db == nil {
		return &localFileTransferRepository{}
	}

	sqliteRepo := &sqliteFileTransferRepository{db}
	if strings.EqualFold(dbType, "mysql") {
		// On 'mysql' database setups return that over the local (hardcoded) values.
		return sqliteRepo
	}

	cutoffCount, ftpCount, fileTransferCount := sqliteRepo.getCounts()
	if (cutoffCount + ftpCount + fileTransferCount) == 0 {
		return &localFileTransferRepository{}
	}

	return sqliteRepo
}

type sqliteFileTransferRepository struct {
	// TODO(adam): admin endpoints to read and write these configs? (w/ masked passwords)
	db *sql.DB
}

func (r *sqliteFileTransferRepository) close() error {
	return r.db.Close()
}

// getCounts returns the count of cutoffTime's, ftpConfig's, and fileTransferConfig's in the sqlite database.
//
// This is used to return localFileTransferRepository if the counts are empty (so local dev "just works").
func (r *sqliteFileTransferRepository) getCounts() (int, int, int) {
	count := func(table string) int {
		query := fmt.Sprintf(`select count(*) from %s`, table)
		stmt, err := r.db.Prepare(query)
		if err != nil {
			return 0
		}
		defer stmt.Close()

		row := stmt.QueryRow()
		var n int
		row.Scan(&n)
		return n
	}
	return count("cutoff_times"), count("ftp_configs"), count("file_transfer_configs")
}

func (r *sqliteFileTransferRepository) getCutoffTimes() ([]*cutoffTime, error) {
	query := `select routing_number, cutoff, location from cutoff_times;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var times []*cutoffTime
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cutoff cutoffTime
		var loc string
		if err := rows.Scan(&cutoff.routingNumber, &cutoff.cutoff, &loc); err != nil {
			return nil, fmt.Errorf("getCutoffTimes: scan: %v", err)
		}
		if l, err := time.LoadLocation(loc); err != nil {
			return nil, fmt.Errorf("getCutoffTimes: parsing %q failed: %v", loc, err)
		} else {
			cutoff.loc = l
		}
		times = append(times, &cutoff)
	}
	return times, rows.Err()
}

func (r *sqliteFileTransferRepository) getFTPConfigs() ([]*ftpConfig, error) {
	query := `select routing_number, hostname, username, password from ftp_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*ftpConfig
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg ftpConfig
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.Hostname, &cfg.Username, &cfg.Password); err != nil {
			return nil, fmt.Errorf("getFTPConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (r *sqliteFileTransferRepository) getFileTransferConfigs() ([]*fileTransferConfig, error) {
	query := `select routing_number, inbound_path, outbound_path, return_path from file_transfer_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*fileTransferConfig
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg fileTransferConfig
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.InboundPath, &cfg.OutboundPath, &cfg.ReturnPath); err != nil {
			return nil, fmt.Errorf("getFileTransferConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

// localFileTransferRepository is a fileTransferRepository for local dev with values that match
// apitest, testdata/ftp-server/ paths, files and FTP (fsftp) defaults.
type localFileTransferRepository struct{}

func (r *localFileTransferRepository) close() error { return nil }

func (r *localFileTransferRepository) getCutoffTimes() ([]*cutoffTime, error) {
	nyc, _ := time.LoadLocation("America/New_York")
	return []*cutoffTime{
		{
			routingNumber: "121042882",
			cutoff:        1700,
			loc:           nyc,
		},
	}, nil
}

func (r *localFileTransferRepository) getFTPConfigs() ([]*ftpConfig, error) {
	return []*ftpConfig{
		{
			RoutingNumber: "121042882",      // from 'go run ./cmd/server' in Accounts
			Hostname:      "localhost:2121", // below configs for moov/fftp:v0.1.0
			Username:      "admin",
			Password:      "123456",
		},
	}, nil
}

func (r *localFileTransferRepository) getFileTransferConfigs() ([]*fileTransferConfig, error) {
	return []*fileTransferConfig{
		{
			RoutingNumber: "121042882",
			InboundPath:   "inbound/", // below configs match paygate's testdata/ftp-server/
			OutboundPath:  "outbound/",
			ReturnPath:    "returned/",
		},
	}, nil
}

// addFileTransferConfigRoutes registers the admin HTTP routes for modifying file-transfer (uploading) configs.
func addFileTransferConfigRoutes(logger log.Logger, svc *admin.Server, repo fileTransferRepository) {
	svc.AddHandler("/configs/uploads", getFileTransferConfigs(logger, repo))

	svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", upsertCutoffTimeConfig(logger, repo))
	svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", deleteCutoffTimeConfig(logger, repo))

	svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", upsertFileTransferConfig(logger, repo))
	svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", deleteFileTransferConfig(logger, repo))

	svc.AddHandler("/configs/uploads/ftp/{routingNumber}", upsertFTPConfig(logger, repo))
	svc.AddHandler("/configs/uploads/ftp/{routingNumber}", deleteFTPConfig(logger, repo))
}

func getRoutingNumber(r *http.Request) string {
	rtn, ok := mux.Vars(r)["routingNumber"]
	if !ok {
		return ""
	}
	return rtn
}

type adminConfigResponse struct {
	CutoffTimes         []*cutoffTime         `json:"cutoffTimes"`
	FTPConfigs          []*ftpConfig          `json:"ftpConfigs"`
	FileTransferConfigs []*fileTransferConfig `json:"fileTransferConfigs"`
}

// getFileTransferConfigs returns all configurations (i.e. FTP, cutoff times, file-transfer configs with passwords masked. (e.g. 'p******d')
func getFileTransferConfigs(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		resp := &adminConfigResponse{}
		if v, err := repo.getCutoffTimes(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.CutoffTimes = v
		}
		if v, err := repo.getFTPConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.FTPConfigs = maskPasswords(v)
		}
		if v, err := repo.getFileTransferConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.FileTransferConfigs = v
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func maskPassword(s string) string {
	if utf8.RuneCountInString(s) < 3 {
		return "**" // too short, we can't mask anything
	} else {
		// turn 'password' into 'p******d'
		first, last := s[0:1], s[len(s)-1:]
		return fmt.Sprintf("%s%s%s", first, strings.Repeat("*", len(s)-2), last)
	}
}

func maskPasswords(cfgs []*ftpConfig) []*ftpConfig {
	for i := range cfgs {
		cfgs[i].Password = maskPassword(cfgs[i].Password)
	}
	return cfgs
}

func upsertCutoffTimeConfig(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))

		w.WriteHeader(http.StatusOK)
	}
}

func deleteCutoffTimeConfig(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))

		w.WriteHeader(http.StatusOK)
	}
}

func upsertFileTransferConfig(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))

		w.WriteHeader(http.StatusOK)
	}
}

func deleteFileTransferConfig(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))

		w.WriteHeader(http.StatusOK)
	}
}

func upsertFTPConfig(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))

		w.WriteHeader(http.StatusOK)
	}
}

func deleteFTPConfig(logger log.Logger, repo fileTransferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))

		w.WriteHeader(http.StatusOK)
	}
}
