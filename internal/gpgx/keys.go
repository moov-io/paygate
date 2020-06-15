// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package gpgx

import (
	"bytes"
	"crypto"
	"errors"
	"io"
	"io/ioutil"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
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
	if entity.PrivateKey != nil && len(password) > 0 {
		entity.PrivateKey.Decrypt(password)
	}
	for _, subkey := range entity.Subkeys {
		if subkey.PrivateKey != nil && len(password) > 0 {
			subkey.PrivateKey.Decrypt(password)
		}
	}

	return entityList, nil
}

func Encrypt(msg []byte, pubkeys openpgp.EntityList) ([]byte, error) {
	var encCloser, armorCloser io.WriteCloser
	var err error

	cfg := &packet.Config{
		DefaultHash:            crypto.SHA256,
		DefaultCipher:          packet.CipherAES256,
		DefaultCompressionAlgo: packet.NoCompression,
	}

	encbuf := new(bytes.Buffer)
	encCloser, err = openpgp.Encrypt(encbuf, pubkeys, nil, nil, cfg)
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
	if err != nil {
		return nil, err
	}

	err = armorCloser.Close()
	if err != nil {
		return nil, err
	}

	return armorbuf.Bytes(), nil
}

func Sign(message []byte, pubKey openpgp.EntityList) ([]byte, error) {
	if len(pubKey) == 0 {
		return nil, errors.New("sign: missing Entity")
	}

	var out bytes.Buffer
	r := bytes.NewReader(message)
	if err := openpgp.ArmoredDetachSign(&out, pubKey[0], r, nil); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func Decrypt(cipherArmored []byte, keys openpgp.EntityList) ([]byte, error) {
	if !(len(keys) == 1 && keys[0].PrivateKey != nil) {
		return nil, errors.New("requires a single private key")
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
			return nil, errors.New("verifying public key included, but message is not signed")
		} else if md.SignedBy.PublicKey.Fingerprint != keys[1].PrimaryKey.Fingerprint {
			return nil, errors.New("signature pubkey doesn't match signing pubkey")
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
