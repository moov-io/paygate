// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gpgx

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

// ReadArmoredKeyFile attempts to read the filepath and parses an armored GPG key
func ReadArmoredKeyFile(path string) (openpgp.EntityList, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return openpgp.ReadArmoredKeyRing(bytes.NewBuffer(bs))
}

// ReadPrivateKeyFile attempts to read the filepath and parses an armored GPG private key
func ReadPrivateKeyFile(path string, password []byte) (openpgp.EntityList, error) {
	// Read the private key
	entityList, err := ReadArmoredKeyFile(path)
	if err != nil {
		return nil, err
	}
	entity := entityList[0]

	// Get the passphrase and read the private key.
	entity.PrivateKey.Decrypt(password)
	for _, subkey := range entity.Subkeys {
		subkey.PrivateKey.Decrypt(password)
	}

	return entityList, nil
}

func Encrypt(msg []byte, pubkeys openpgp.EntityList) ([]byte, error) {
	var encCloser, armorCloser io.WriteCloser
	var err error

	encbuf := new(bytes.Buffer)
	encCloser, err = openpgp.Encrypt(encbuf, pubkeys, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	_, err = encCloser.Write(msg)
	if err != nil {
		return nil, err
	}

	err = encCloser.Close()
	if err != nil {
		return nil, err
	}

	armorbuf := new(bytes.Buffer)
	armorCloser, err = armor.Encode(armorbuf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, err
	}

	_, err = armorCloser.Write(encbuf.Bytes())

	err = armorCloser.Close()
	if err != nil {
		return nil, err
	}

	return armorbuf.Bytes(), nil
}

func Decrypt(cipherArmored []byte, keys openpgp.EntityList) ([]byte, error) {
	if !(len(keys) == 1 && keys[0].PrivateKey != nil) {
		return nil, errors.New("Requires a single private key.")
	}
	return readMessage(cipherArmored, keys)
}

func readMessage(armoredMessage []byte, keys openpgp.EntityList) ([]byte, error) {
	// Decode armored message
	decbuf := bytes.NewBuffer(armoredMessage)
	result, err := armor.Decode(decbuf)
	if err != nil {
		return nil, err
	}

	// Decrypt with private key
	md, err := openpgp.ReadMessage(result.Body, keys, nil, nil)
	if err != nil {
		return nil, err
	}

	// If pubkey included, verify
	if len(keys) == 2 {
		if md.SignedBy == nil || md.SignedBy.PublicKey == nil {
			return nil, errors.New("Verifying public key included, but message is not signed.")
		} else if md.SignedBy.PublicKey.Fingerprint != keys[1].PrimaryKey.Fingerprint {
			return nil, errors.New("Signature pubkey doesn't match signing pubkey.")
		}
	}

	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, err
	}
	if md.SignatureError != nil {
		return nil, md.SignatureError
	}

	return bytes, nil
}
