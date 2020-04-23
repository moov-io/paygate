// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"database/sql"
	"fmt"
	"time"
)

type SQLRepository struct {
	db *sql.DB
}

func (r *SQLRepository) Close() error {
	return r.db.Close()
}

// GetCounts returns the count of CutoffTime's, FTPConfig's, and Config's in the sqlite database.
//
// This is used to return defaults if the counts are empty (so local dev "just works").
func (r *SQLRepository) GetCounts() (int, int, int) {
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

func (r *SQLRepository) GetConfigs() ([]*Config, error) {
	query := `select routing_number, inbound_path, outbound_path, return_path, outbound_filename_template, allowed_ips from file_transfer_configs;`
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
		if err := rows.Scan(&cfg.RoutingNumber, &cfg.InboundPath, &cfg.OutboundPath, &cfg.ReturnPath, &cfg.OutboundFilenameTemplate, &cfg.AllowedIPs); err != nil {
			return nil, fmt.Errorf("GetConfigs: scan: %v", err)
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (r *SQLRepository) GetCutoffTimes() ([]*CutoffTime, error) {
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

func (r *SQLRepository) getOutboundFilenameTemplates() ([]string, error) {
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

func (r *SQLRepository) upsertConfig(cfg *Config) error {
	query := `replace into file_transfer_configs (routing_number, inbound_path, outbound_path, return_path, outbound_filename_template, allowed_ips) values (?, ?, ?, ?, ?, ?);`
	return exec(r.db, query, cfg.RoutingNumber, cfg.InboundPath, cfg.OutboundPath, cfg.ReturnPath, cfg.OutboundFilenameTemplate, cfg.AllowedIPs)
}

func (r *SQLRepository) deleteConfig(routingNumber string) error {
	query := `delete from file_transfer_configs where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

func (r *SQLRepository) upsertCutoffTime(routingNumber string, cutoff int, loc *time.Location) error {
	query := `replace into cutoff_times (routing_number, cutoff, location) values (?, ?, ?);`
	return exec(r.db, query, routingNumber, cutoff, loc.String())
}

func (r *SQLRepository) deleteCutoffTime(routingNumber string) error {
	query := `delete from cutoff_times where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

func (r *SQLRepository) GetFTPConfigs() ([]*FTPConfig, error) {
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

func (r *SQLRepository) upsertFTPConfigs(routingNumber, host, user, pass string) error {
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

func (r *SQLRepository) deleteFTPConfig(routingNumber string) error {
	query := `delete from ftp_configs where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}

func (r *SQLRepository) GetSFTPConfigs() ([]*SFTPConfig, error) {
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

func (r *SQLRepository) upsertSFTPConfigs(routingNumber, host, user, pass, privateKey, publicKey string) error {
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

func (r *SQLRepository) deleteSFTPConfig(routingNumber string) error {
	query := `delete from sftp_configs where routing_number = ?;`
	return exec(r.db, query, routingNumber)
}
