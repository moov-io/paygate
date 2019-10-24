// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"
	"gopkg.in/yaml.v2"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type Repository interface {
	GetConfigs() ([]*Config, error)
	upsertConfig(cfg *Config) error
	deleteConfig(routingNumber string) error

	GetCutoffTimes() ([]*CutoffTime, error)
	upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error
	deleteCutoffTime(routingNumber string) error

	GetFTPConfigs() ([]*FTPConfig, error)
	upsertFTPConfigs(routingNumber, host, user, pass string) error
	deleteFTPConfig(routingNumber string) error

	GetSFTPConfigs() ([]*SFTPConfig, error)
	upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error
	deleteSFTPConfig(routingNumber string) error

	Close() error
}

var (
	devFileTransferType = os.Getenv("DEV_FILE_TRANSFER_TYPE")
)

func NewRepository(logger log.Logger, path string, db *sql.DB, dbType string) Repository {
	if path != "" {
		repo, err := newRepositoryFromConfig(path)
		if err != nil {
			panic(fmt.Sprintf("ERROR: problem reading config file: %v", err))
		}
		logger.Log("file-transfer-configs", fmt.Sprintf("config loaded from=%s", path))
		return repo
	}

	if db == nil {
		logger.Log("file-transfer-configs", "local dev config used")
		repo := &staticRepository{
			configs:     make(map[string]*Config),
			cutoffTimes: make(map[string]*CutoffTime),
			ftpConfigs:  make(map[string]*FTPConfig),
			sftpConfigs: make(map[string]*SFTPConfig),
			protocol:    devFileTransferType,
		}
		repo.populate()
		return repo
	}

	sqliteRepo := &sqlRepository{db}
	if strings.EqualFold(dbType, "mysql") {
		// On 'mysql' database setups return that over the local (hardcoded) values.
		logger.Log("file-transfer-configs", "config loaded from mysql")
		return sqliteRepo
	}

	cutoffCount, ftpCount, fileTransferCount := sqliteRepo.GetCounts()
	if (cutoffCount + ftpCount + fileTransferCount) == 0 {
		logger.Log("file-transfer-configs", "local dev config used")
		repo := &staticRepository{
			configs:     make(map[string]*Config),
			cutoffTimes: make(map[string]*CutoffTime),
			ftpConfigs:  make(map[string]*FTPConfig),
			sftpConfigs: make(map[string]*SFTPConfig),
			protocol:    devFileTransferType,
		}
		repo.populate()
		return repo
	}

	logger.Log("file-transfer-configs", "config loaded from sqlite")
	return sqliteRepo
}

type sqlRepository struct {
	db *sql.DB
}

func (r *sqlRepository) Close() error {
	return r.db.Close()
}

// GetCounts returns the count of CutoffTime's, FTPConfig's, and Config's in the sqlite database.
//
// This is used to return *staticRepository if the counts are empty (so local dev "just works").
func (r *sqlRepository) GetCounts() (int, int, int) {
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

func (r *sqlRepository) GetConfigs() ([]*Config, error) {
	query := `select routing_number, inbound_path, outbound_path, return_path, outbound_filename_template from file_transfer_configs;`
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
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.InboundPath, &cfg.OutboundPath, &cfg.ReturnPath, &cfg.OutboundFilenameTemplate); err != nil {
			return nil, fmt.Errorf("GetConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (r *sqlRepository) GetCutoffTimes() ([]*CutoffTime, error) {
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

func exec(db *sql.DB, rawQuery string, args ...interface{}) error {
	stmt, err := db.Prepare(rawQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(args...)
	return err
}

func (r *sqlRepository) getOutboundFilenameTemplates() ([]string, error) {
	query := `select outbound_filename_template from file_transfer_configs where outbound_filename_template <> '';`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []string
	for rows.Next() {
		var tmpl string
		if err := rows.Scan(&tmpl); err != nil {
			return nil, err
		}
		templates = append(templates, tmpl)
	}
	return templates, rows.Err()
}

func (r *sqlRepository) upsertConfig(cfg *Config) error {
	query := `replace into file_transfer_configs (routing_number, inbound_path, outbound_path, return_path, outbound_filename_template) values (?, ?, ?, ?, ?);`
	return exec(r.db, query, cfg.RoutingNumber, cfg.InboundPath, cfg.OutboundPath, cfg.ReturnPath, cfg.OutboundFilenameTemplate)
}

func (r *sqlRepository) deleteConfig(routingNumber string) error {
	query := `delete from file_transfer_configs where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

func (r *sqlRepository) upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error {
	query := `replace into cutoff_times (routing_number, cutoff, location) values (?, ?, ?);`
	return exec(r.db, query, routingNumber, cutoff, loc.String())
}

func (r *sqlRepository) deleteCutoffTime(routingNumber string) error {
	query := `delete from cutoff_times where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

func (r *sqlRepository) GetFTPConfigs() ([]*FTPConfig, error) {
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

func (r *sqlRepository) upsertFTPConfigs(routingNumber, host, user, pass string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`select password from ftp_configs where routing_number = ? limit 1;`)
	if err != nil {
		return fmt.Errorf("error reading existing password: error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	row := stmt.QueryRow(routingNumber)
	var existingPass string
	if err := row.Scan(&existingPass); err != nil {
		return fmt.Errorf("error scanning existing password: error=%v rollback=%v", err, tx.Rollback())
	}
	if pass == "" {
		pass = existingPass
	}

	query := `replace into ftp_configs (routing_number, hostname, username, password) values (?, ?, ?, ?);`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("error preparing replace: error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()
	if _, err := stmt.Exec(routingNumber, host, user, pass); err != nil {
		return fmt.Errorf("error replacing ftp config error=%v rollback=%v", err, tx.Rollback())
	}

	return tx.Commit()
}

func (r *sqlRepository) deleteFTPConfig(routingNumber string) error {
	query := `delete from ftp_configs where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

func (r *sqlRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
	query := `select routing_number, hostname, username, password, client_private_key, host_public_key from sftp_configs;`
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
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.Hostname, &cfg.Username, &cfg.Password, &cfg.ClientPrivateKey, &cfg.HostPublicKey); err != nil {
			return nil, fmt.Errorf("GetSFTPConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (r *sqlRepository) upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	query := `select password, client_private_key, host_public_key from sftp_configs where routing_number = ? limit 1;`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("error preparing read: error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	// read existing values
	ePass, ePriv, ePub := "", "", ""
	if err := stmt.QueryRow(routingNumber).Scan(&ePass, &ePriv, &ePub); err != nil {
		return fmt.Errorf("error reading existing: error=%v rollback=%v", err, tx.Rollback())
	}

	if pass == "" {
		pass = ePass
	}
	if privateKey == "" {
		privateKey = ePriv
	}
	if publicKey == "" {
		publicKey = ePub
	}

	// update/insert entire row
	query = `replace into sftp_configs (routing_number, hostname, username, password, client_private_key, host_public_key) values (?, ?, ?, ?, ?, ?);`
	stmt, err = tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("error preparing replace: error=%v rollback=%v", err, tx.Rollback())
	}
	defer stmt.Close()

	if _, err := stmt.Exec(routingNumber, host, user, pass, privateKey, publicKey); err != nil {
		return fmt.Errorf("error executing repalce: error=%v rollback=%v", err, tx.Rollback())
	}

	return tx.Commit()
}

func (r *sqlRepository) deleteSFTPConfig(routingNumber string) error {
	query := `delete from sftp_configs where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

type staticRepository struct {
	cutoffTimes map[string]*CutoffTime
	configs     map[string]*Config
	ftpConfigs  map[string]*FTPConfig
	sftpConfigs map[string]*SFTPConfig

	// protocol represents values like ftp or sftp to return back relevant configs
	// to the moov/fsftp or SFTP docker image
	protocol string
}

type cutoff struct {
	RoutingNumber string `json:"routingNumber" yaml:"routingNumber"`
	Cutoff        int    `json:"cutoff" yaml:"cutoff"`
	Location      string `json:"location" yaml:"location"`
}

func newRepositoryFromConfig(path string) (*staticRepository, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var file struct {
		CutoffTimes []*cutoff     `yaml:"cutoffTimes"`
		Configs     []*Config     `yaml:"configs"`
		FTPConfigs  []*FTPConfig  `yaml:"ftpConfigs"`
		SFTPConfigs []*SFTPConfig `yaml:"sftpConfigs"`
	}
	if err := yaml.NewDecoder(fd).Decode(&file); err != nil {
		return nil, err
	}

	repo := &staticRepository{
		configs:     make(map[string]*Config),
		cutoffTimes: make(map[string]*CutoffTime),
		ftpConfigs:  make(map[string]*FTPConfig),
		sftpConfigs: make(map[string]*SFTPConfig),
	}
	for i := range file.CutoffTimes {
		loc, err := time.LoadLocation(file.CutoffTimes[i].Location)
		if err != nil {
			return nil, fmt.Errorf("routingNumber=%s unable to parse %q", file.CutoffTimes[i].RoutingNumber, file.CutoffTimes[i].Location)
		}
		ct := &CutoffTime{
			RoutingNumber: file.CutoffTimes[i].RoutingNumber,
			Cutoff:        file.CutoffTimes[i].Cutoff,
			Loc:           loc,
		}
		repo.cutoffTimes[ct.RoutingNumber] = ct
	}
	for i := range file.Configs {
		repo.configs[file.Configs[i].RoutingNumber] = file.Configs[i]
	}
	for i := range file.FTPConfigs {
		repo.ftpConfigs[file.FTPConfigs[i].RoutingNumber] = file.FTPConfigs[i]
	}
	for i := range file.SFTPConfigs {
		repo.sftpConfigs[file.SFTPConfigs[i].RoutingNumber] = file.SFTPConfigs[i]
	}

	if len(repo.configs) == 0 {
		repo.populate()
	}

	return repo, nil
}

func (r *staticRepository) populate() {
	r.populateConfigs()
	r.populateCutoffTimes()

	switch strings.ToLower(r.protocol) {
	case "", "ftp":
		r.populateFTPConfigs()
	case "sftp":
		r.populateSFTPConfigs()
	}
}

func (r *staticRepository) populateConfigs() {
	cfg := &Config{RoutingNumber: "121042882"} // test value, matches apitest

	switch strings.ToLower(r.protocol) {
	case "", "ftp":
		// For 'make start-ftp-server', configs match paygate's testdata/ftp-server/
		cfg.InboundPath = "inbound/"
		cfg.OutboundPath = "outbound/"
		cfg.ReturnPath = "returned/"
	case "sftp":
		// For 'make start-sftp-server', configs match paygate's testdata/sftp-server/
		cfg.InboundPath = "/upload/inbound/"
		cfg.OutboundPath = "/upload/outbound/"
		cfg.ReturnPath = "/upload/returned/"
	}

	r.configs[cfg.RoutingNumber] = cfg
}

func (r *staticRepository) populateCutoffTimes() {
	nyc, _ := time.LoadLocation("America/New_York")
	ct := &CutoffTime{
		RoutingNumber: "121042882",
		Cutoff:        1700,
		Loc:           nyc,
	}

	r.cutoffTimes[ct.RoutingNumber] = ct
}

func (r *staticRepository) populateFTPConfigs() {
	cfg := &FTPConfig{
		RoutingNumber: "121042882",
		Hostname:      "localhost:2121", // below configs for moov/fsftp:v0.1.0
		Username:      "admin",
		Password:      "123456",
	}
	r.ftpConfigs[cfg.RoutingNumber] = cfg
}

func (r *staticRepository) populateSFTPConfigs() {
	cfg := &SFTPConfig{
		RoutingNumber: "121042882",
		Hostname:      "localhost:2222", // below configs for atmoz/sftp:latest
		Username:      "demo",
		Password:      "password",
		// ClientPrivateKey: "...", // Base64 encoded or PEM format
	}
	r.sftpConfigs[cfg.RoutingNumber] = cfg
}

func (r *staticRepository) GetConfigs() ([]*Config, error) {
	var out []*Config
	for i := range r.configs {
		out = append(out, r.configs[i])
	}
	return out, nil
}

func (r *staticRepository) GetCutoffTimes() ([]*CutoffTime, error) {
	var out []*CutoffTime
	for i := range r.cutoffTimes {
		out = append(out, r.cutoffTimes[i])
	}
	return out, nil
}

func (r *staticRepository) GetFTPConfigs() ([]*FTPConfig, error) {
	var out []*FTPConfig
	for i := range r.ftpConfigs {
		out = append(out, r.ftpConfigs[i])
	}
	return out, nil
}

func (r *staticRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
	var out []*SFTPConfig
	for i := range r.sftpConfigs {
		out = append(out, r.sftpConfigs[i])
	}
	return out, nil
}

func (r *staticRepository) Close() error {
	return nil
}

func (r *staticRepository) upsertConfig(cfg *Config) error {
	return nil
}

func (r *staticRepository) deleteConfig(routingNumber string) error {
	return nil
}

func (r *staticRepository) upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error {
	return nil
}

func (r *staticRepository) deleteCutoffTime(routingNumber string) error {
	return nil
}

func (r *staticRepository) upsertFTPConfigs(routingNumber, host, user, pass string) error {
	return nil
}

func (r *staticRepository) deleteFTPConfig(routingNumber string) error {
	return nil
}

func (r *staticRepository) upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error {
	return nil
}

func (r *staticRepository) deleteSFTPConfig(routingNumber string) error {
	return nil
}

// AddFileTransferConfigRoutes registers the admin HTTP routes for modifying file-transfer (uploading) configs.
func AddFileTransferConfigRoutes(logger log.Logger, svc *admin.Server, repo Repository) {
	svc.AddHandler("/configs/uploads", GetConfigs(logger, repo))
	svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", manageCutoffTimeConfig(logger, repo))
	svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", manageFileTransferConfig(logger, repo))
	svc.AddHandler("/configs/uploads/ftp/{routingNumber}", manageFTPConfig(logger, repo))
	svc.AddHandler("/configs/uploads/sftp/{routingNumber}", manageSFTPConfig(logger, repo))
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

func manageCutoffTimeConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				Cutoff   int    `json:"cutoff"`
				Location string `json:"location"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if req.Cutoff == 0 {
				moovhttp.Problem(w, errors.New("misisng cutoff"))
				return
			}
			loc, err := time.LoadLocation(req.Location)
			if err != nil {
				moovhttp.Problem(w, fmt.Errorf("time: %s: %v", req.Location, err))
				return
			}
			if err := repo.upsertCutoffTime(routingNumber, req.Cutoff, loc); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updating cutoff time config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		case "DELETE":
			if err := repo.deleteCutoffTime(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleting cutoff time config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		default:
			moovhttp.Problem(w, fmt.Errorf("cutoff-times: unsupported HTTP verb %s", r.Method))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func manageFileTransferConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				InboundPath              string `json:"inboundPath,omitempty"`
				OutboundPath             string `json:"outboundPath,omitempty"`
				ReturnPath               string `json:"returnPath,omitempty"`
				OutboundFilenameTemplate string `json:"outboundFilenameTemplate,omitempty"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			// Ensure that a provided template validates before saving it
			if req.OutboundFilenameTemplate != "" {
				if err := validateTemplate(req.OutboundFilenameTemplate); err != nil {
					moovhttp.Problem(w, err)
					return
				}
			}
			err := repo.upsertConfig(&Config{
				RoutingNumber:            routingNumber,
				InboundPath:              req.InboundPath,
				OutboundPath:             req.OutboundPath,
				ReturnPath:               req.ReturnPath,
				OutboundFilenameTemplate: req.OutboundFilenameTemplate,
			})
			if err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updated config for routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))
			w.WriteHeader(http.StatusOK)

		case "DELETE":
			if err := repo.deleteConfig(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleted config for routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))
			w.WriteHeader(http.StatusOK)

		default:
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
	}
}

func manageFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				Hostname string `json:"hostname"`
				Username string `json:"username"`
				Password string `json:"password,omitempty"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if req.Hostname == "" || req.Username == "" {
				moovhttp.Problem(w, errors.New("missing hostname, or username"))
				return
			}
			if err := repo.upsertFTPConfigs(routingNumber, req.Hostname, req.Username, req.Password); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updating FTP configs routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		case "DELETE":
			if err := repo.deleteFTPConfig(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleting FTP config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		default:
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func manageSFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				Hostname         string `json:"hostname"`
				Username         string `json:"username"`
				Password         string `json:"password,omitempty"`
				ClientPrivateKey string `json:"clientPrivateKey,omitempty"`
				HostPublicKey    string `json:"hostPublicKey,omitempty"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if req.Hostname == "" || req.Username == "" {
				moovhttp.Problem(w, errors.New("missing hostname, or username"))
				return
			}
			if err := repo.upsertSFTPConfigs(routingNumber, req.Hostname, req.Username, req.Password, req.ClientPrivateKey, req.HostPublicKey); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updating SFTP config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		case "DELETE":
			if err := repo.deleteSFTPConfig(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleting SFTP cofnig routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		default:
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
