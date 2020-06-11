// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package transform

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/gpgx"
	"github.com/moov-io/paygate/internal/sshx"
	"github.com/moov-io/paygate/pkg/config"

	"github.com/go-kit/kit/log"
	"golang.org/x/crypto/openpgp"
)

type GPGEncryption struct {
	entityList openpgp.EntityList
}

func NewGPGEncryptor(logger log.Logger, cfg *config.GPG) (*GPGEncryption, error) {
	if cfg == nil {
		return nil, errors.New("missing GPG config")
	}

	entityList, err := gpgx.ReadArmoredKeyFile(cfg.KeyFile)
	if err != nil {
		pubKey, _ := sshx.ReadPubKeyFile(cfg.KeyFile)
		if pubKey == nil {
			return nil, err // return previous error
		}
		entityList = gpgx.FromSSHPublicKey(pubKey)
		if entityList == nil {
			return nil, errors.New("no GPG entites/keys found")
		}
	}

	return &GPGEncryption{
		entityList: entityList,
	}, nil
}

func (morph *GPGEncryption) Transform(res *Result) (*Result, error) {
	var buf bytes.Buffer
	if err := ach.NewWriter(&buf).Write(res.File); err != nil {
		return res, err
	}

	bs, err := gpgx.Encrypt(buf.Bytes(), morph.entityList)
	if err != nil {
		return res, err
	}
	res.Encrypted = bs

	return res, nil
}

func (morph *GPGEncryption) String() string {
	return fmt.Sprintf("GPG{entityList:%v}", len(morph.entityList) > 0)
}
