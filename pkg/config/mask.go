// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"net/url"

	"github.com/moov-io/paygate/x/mask"
)

// Masked returns a deep copy of cfg with credentials and other secrets replaced
// by masked values. The original Config is left unchanged.
func (cfg *Config) Masked() *Config {
	if cfg == nil {
		return nil
	}

	bs, err := json.Marshal(cfg)
	if err != nil {
		return nil
	}
	var out Config
	if err := json.Unmarshal(bs, &out); err != nil {
		return nil
	}
	out.maskSecrets()
	return &out
}

func (cfg *Config) maskSecrets() {
	if cfg.Database.MySQL != nil {
		cfg.Database.MySQL.Password = mask.Password(cfg.Database.MySQL.Password)
	}
	if cfg.ODFI.FTP != nil {
		cfg.ODFI.FTP.Password = mask.Password(cfg.ODFI.FTP.Password)
	}
	if cfg.ODFI.SFTP != nil {
		cfg.ODFI.SFTP.Password = mask.Password(cfg.ODFI.SFTP.Password)
		cfg.ODFI.SFTP.ClientPrivateKey = mask.Password(cfg.ODFI.SFTP.ClientPrivateKey)
		cfg.ODFI.SFTP.HostPublicKey = mask.Password(cfg.ODFI.SFTP.HostPublicKey)
	}
	if cfg.Customers.Accounts.Decryptor.Symmetric != nil {
		cfg.Customers.Accounts.Decryptor.Symmetric.KeyURI = mask.Password(cfg.Customers.Accounts.Decryptor.Symmetric.KeyURI)
	}
	if n := cfg.Pipeline.Notifications; n != nil {
		if n.Email != nil {
			n.Email.ConnectionURI = maskURI(n.Email.ConnectionURI)
		}
		if n.PagerDuty != nil {
			n.PagerDuty.ApiKey = mask.Password(n.PagerDuty.ApiKey)
			n.PagerDuty.ServiceKey = mask.Password(n.PagerDuty.ServiceKey)
		}
		if n.Slack != nil {
			n.Slack.WebhookURL = mask.Password(n.Slack.WebhookURL)
		}
	}
	if audit := cfg.Pipeline.AuditTrail; audit != nil && audit.GPG != nil && audit.GPG.Signer != nil {
		audit.GPG.Signer.KeyPassword = mask.Password(audit.GPG.Signer.KeyPassword)
	}
}

func maskURI(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return mask.Password(raw)
	}
	username := u.User.Username()
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(username, "******")
	}
	return u.String()
}
