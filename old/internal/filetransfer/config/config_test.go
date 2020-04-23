// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"testing"
)

func TestConfig__maskPassword(t *testing.T) {
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
