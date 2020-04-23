// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"
	"time"
)

func TestStaticRepository(t *testing.T) {
	repo := NewRepository("", nil, "")
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

	// switch to SFTP
	if r, ok := repo.(*StaticRepository); ok {
		r.FTPConfigs = nil
		r.Protocol = "sftp"
		r.Populate()
	}

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
