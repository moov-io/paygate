// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transform

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/gpgx"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
	"golang.org/x/crypto/openpgp"
)

type GPGEncryption struct {
	pubKey     openpgp.EntityList
	signingKey openpgp.EntityList
}

func NewGPGEncryptor(logger log.Logger, cfg *config.GPG) (*GPGEncryption, error) {
	if cfg == nil {
		return nil, errors.New("missing GPG config")
	}

	out := &GPGEncryption{}

	pubKey, err := gpgx.ReadArmoredKeyFile(cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	out.pubKey = pubKey

	// Print the public key's fingerprint
	if fp := fingerprint(pubKey); fp != "" {
		logger.Log("gpg", fmt.Sprintf("using GPG key %s for pre-upload encryption", fp))
	}

	// Read a signing key if it exists
	if cfg.Signer != nil {
		privKey, err := gpgx.ReadPrivateKeyFile(cfg.Signer.KeyFile, []byte(cfg.Signer.Password()))
		if err != nil {
			return nil, err
		}
		out.signingKey = privKey

		// Print the private key's fingerprint
		if fp := fingerprint(privKey); fp != "" {
			logger.Log("gpg", fmt.Sprintf("using GPG signing key %s for pre-upload encryption", fp))
		}
	}

	return out, nil
}

func fingerprint(key openpgp.EntityList) string {
	if len(key) > 0 {
		if key := key[0].PrimaryKey; key != nil {
			var buf bytes.Buffer
			for i := range key.Fingerprint {
				buf.WriteString(fmt.Sprintf("%s:", strings.ToUpper(hex.EncodeToString(key.Fingerprint[i:i+1]))))
			}
			return strings.TrimSuffix(buf.String(), ":")
		}
	}
	return ""
}

func (morph *GPGEncryption) Transform(res *Result) (*Result, error) {
	var buf bytes.Buffer
	if err := ach.NewWriter(&buf).Write(res.File); err != nil {
		return res, err
	}

	bs, err := gpgx.Encrypt(buf.Bytes(), morph.pubKey)
	if err != nil {
		return res, err
	}

	// Sign the file after encrypting it
	if len(morph.signingKey) > 0 {
		bs, err = gpgx.Sign(bs, morph.signingKey)
		if err != nil {
			return res, err
		}
	}

	res.Encrypted = bs
	return res, nil
}

func (morph *GPGEncryption) String() string {
	return fmt.Sprintf("GPG{pubKey:%v}", len(morph.pubKey) > 0)
}
