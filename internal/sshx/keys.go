// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package sshx

import (
	"encoding/base64"

	"golang.org/x/crypto/ssh"
)

// ReadPubKey attempts to parse data and return an ssh PublicKey.
//
// It attempts formats such as:
//   - an authorized_keys file used with OpenSSH according to the sshd(8) manual page
//   - used in the SSH wire protocol according to RFC 4253, section 6.6
func ReadPubKey(data []byte) (ssh.PublicKey, error) {
	readAuthd := func(data []byte) (ssh.PublicKey, error) {
		pub, _, _, _, err := ssh.ParseAuthorizedKey(data)
		return pub, err
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if len(decoded) > 0 && err == nil {
		if pub, err := readAuthd(decoded); pub != nil && err == nil {
			return pub, nil
		}
		return ssh.ParsePublicKey(decoded)
	}

	if pub, err := readAuthd(data); pub != nil && err == nil {
		return pub, nil
	}
	return ssh.ParsePublicKey(data)
}
