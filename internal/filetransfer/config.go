// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

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

type Repository interface {
	GetConfigs() ([]*Config, error)
	GetCutoffTimes() ([]*CutoffTime, error)

	GetFTPConfigs() ([]*FTPConfig, error)
	GetSFTPConfigs() ([]*SFTPConfig, error)

	Close() error
}

func NewRepository(db *sql.DB, dbType string) Repository {
	if db == nil {
		return &localFileTransferRepository{}
	}

	sqliteRepo := &sqliteRepository{db}
	if strings.EqualFold(dbType, "mysql") {
		// On 'mysql' database setups return that over the local (hardcoded) values.
		return sqliteRepo
	}

	cutoffCount, ftpCount, fileTransferCount := sqliteRepo.GetCounts()
	if (cutoffCount + ftpCount + fileTransferCount) == 0 {
		return &localFileTransferRepository{}
	}

	return sqliteRepo
}

type sqliteRepository struct {
	// TODO(adam): admin endpoints to read and write these configs? (w/ masked passwords)
	db *sql.DB
}

func (r *sqliteRepository) Close() error {
	return r.db.Close()
}

// GetCounts returns the count of CutoffTime's, FTPConfig's, and Config's in the sqlite database.
//
// This is used to return localFileTransferRepository if the counts are empty (so local dev "just works").
func (r *sqliteRepository) GetCounts() (int, int, int) {
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

func (r *sqliteRepository) GetConfigs() ([]*Config, error) {
	query := `select routing_number, inbound_path, outbound_path, return_path from file_transfer_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*Config
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg Config
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.InboundPath, &cfg.OutboundPath, &cfg.ReturnPath); err != nil {
			return nil, fmt.Errorf("GetConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (r *sqliteRepository) GetCutoffTimes() ([]*CutoffTime, error) {
	query := `select routing_number, cutoff, location from cutoff_times;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var times []*CutoffTime
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cutoff CutoffTime
		var loc string
		if err := rows.Scan(&cutoff.RoutingNumber, &cutoff.Cutoff, &loc); err != nil {
			return nil, fmt.Errorf("GetCutoffTimes: scan: %v", err)
		}
		if l, err := time.LoadLocation(loc); err != nil {
			return nil, fmt.Errorf("GetCutoffTimes: parsing %q failed: %v", loc, err)
		} else {
			cutoff.Loc = l
		}
		times = append(times, &cutoff)
	}
	return times, rows.Err()
}

func (r *sqliteRepository) GetFTPConfigs() ([]*FTPConfig, error) {
	query := `select routing_number, hostname, username, password from ftp_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*FTPConfig
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg FTPConfig
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.Hostname, &cfg.Username, &cfg.Password); err != nil {
			return nil, fmt.Errorf("GetFTPConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (r *sqliteRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
	query := `select routing_number, hostname, username, password, client_private_key from sftp_configs;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var configs []*SFTPConfig
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cfg SFTPConfig
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.Hostname, &cfg.Username, &cfg.Password, &cfg.ClientPrivateKey); err != nil {
			return nil, fmt.Errorf("GetSFTPConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

// localFileTransferRepository is a Repository for local dev with values that match
// apitest, testdata/ftp-server/ paths, files and FTP (fsftp) defaults.
type localFileTransferRepository struct{}

func (r *localFileTransferRepository) Close() error { return nil }

func (r *localFileTransferRepository) GetConfigs() ([]*Config, error) {
	return []*Config{
		{
			RoutingNumber: "121042882",
			InboundPath:   "inbound/", // below configs match paygate's testdata/ftp-server/
			OutboundPath:  "outbound/",
			ReturnPath:    "returned/",
		},
	}, nil
}

func (r *localFileTransferRepository) GetCutoffTimes() ([]*CutoffTime, error) {
	nyc, _ := time.LoadLocation("America/New_York")
	return []*CutoffTime{
		{
			RoutingNumber: "121042882",
			Cutoff:        1700,
			Loc:           nyc,
		},
	}, nil
}

func (r *localFileTransferRepository) GetFTPConfigs() ([]*FTPConfig, error) {
	return []*FTPConfig{
		{
			RoutingNumber: "121042882",
			Hostname:      "localhost:2121", // below configs for moov/fftp:v0.1.0
			Username:      "admin",
			Password:      "123456",
		},
	}, nil
}

func (r *localFileTransferRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
	return []*SFTPConfig{
		{
			RoutingNumber: "121042882",
			Hostname:      "localhost:22", // below configs for atmoz/sftp:latest
			Username:      "demo",
			Password:      "password",
			// ClientPrivateKey: "...", // Base64 encoded or PEM format
		},
	}, nil
}

// AddFileTransferConfigRoutes registers the admin HTTP routes for modifying file-transfer (uploading) configs.
func AddFileTransferConfigRoutes(logger log.Logger, svc *admin.Server, repo Repository) {
	svc.AddHandler("/configs/uploads", GetConfigs(logger, repo))

	svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", upsertCutoffTimeConfig(logger, repo))
	svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", deleteCutoffTimeConfig(logger, repo))

	svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", upsertFileTransferConfig(logger, repo))
	svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", deleteFileTransferConfig(logger, repo))

	svc.AddHandler("/configs/uploads/ftp/{routingNumber}", upsertFTPConfig(logger, repo))
	svc.AddHandler("/configs/uploads/ftp/{routingNumber}", deleteFTPConfig(logger, repo))
	// svc.AddHandler("/configs/uploads/sftp/{routingNumber}", upsertSFTPConfig(logger, repo)) // TODO(adam): impl
	// svc.AddHandler("/configs/uploads/sftp/{routingNumber}", deleteSFTPConfig(logger, repo))
}

func getRoutingNumber(r *http.Request) string {
	rtn, ok := mux.Vars(r)["routingNumber"]
	if !ok {
		return ""
	}
	return rtn
}

type adminConfigResponse struct {
	CutoffTimes         []*CutoffTime `json:"CutoffTimes"`
	FileTransferConfigs []*Config     `json:"Configs"`
	FTPConfigs          []*FTPConfig  `json:"FTPConfigs"`
	SFTPConfigs         []*SFTPConfig `json:"SFTPConfigs"`
}

// GetConfigs returns all configurations (i.e. FTP, cutoff times, file-transfer configs with passwords masked. (e.g. 'p******d')
func GetConfigs(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		resp := &adminConfigResponse{}
		if v, err := repo.GetCutoffTimes(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.CutoffTimes = v
		}
		if v, err := repo.GetConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.FileTransferConfigs = v
		}
		if v, err := repo.GetFTPConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.FTPConfigs = maskFTPPasswords(v)
		}
		if v, err := repo.GetSFTPConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.SFTPConfigs = maskSFTPPasswords(v)
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

func maskFTPPasswords(cfgs []*FTPConfig) []*FTPConfig {
	for i := range cfgs {
		cfgs[i].Password = maskPassword(cfgs[i].Password)
	}
	return cfgs
}

func maskSFTPPasswords(cfgs []*SFTPConfig) []*SFTPConfig {
	for i := range cfgs {
		cfgs[i].Password = maskPassword(cfgs[i].Password)
	}
	return cfgs
}

func upsertCutoffTimeConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func deleteCutoffTimeConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func upsertFileTransferConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func deleteFileTransferConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func upsertFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func deleteFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func upsertSFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}

func deleteSFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		logger.Log("file-transfer-configs", "", "requestId", moovhttp.GetRequestId(r))
		w.WriteHeader(http.StatusOK)
	}
}
