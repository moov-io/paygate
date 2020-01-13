// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package secrets

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSecrets(t *testing.T) {
	// We assume CLOUD_PROVIDER is unset
	keeper, err := GetSecretKeeper("foo")
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := keeper.Encrypt(context.Background(), []byte("hello, world"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := keeper.Decrypt(context.Background(), encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if v := string(out); v != "hello, world" {
		t.Errorf("got %q", v)
	}
}

func TestSecrets__OpenLocal(t *testing.T) {
	if _, err := OpenLocal("invalid key"); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "SECRETS_LOCAL_BASE64_KEY") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	keeper, err := testSecretKeeper(testSecretKey)("test-path")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := keeper.Encrypt(context.Background(), []byte("hello, world"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := keeper.Decrypt(context.Background(), enc)
	if err != nil {
		t.Fatal(err)
	}
	if v := string(out); v != "hello, world" {
		t.Errorf("got %q", v)
	}
}

func TestStringKeeper__cycle(t *testing.T) {
	keeper, err := testSecretKeeper(testSecretKey)("string-keeper")
	if err != nil {
		t.Fatal(err)
	}

	str := NewStringKeeper(keeper, 1*time.Second)
	defer str.Close()

	encrypted, err := str.EncryptString("123")
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := str.DecryptString(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "123" {
		t.Errorf("decrypted=%s", decrypted)
	}
}

func TestStringKeeper__nil(t *testing.T) {
	keeper := TestStringKeeper(t)
	keeper.Close()

	keeper = nil

	if _, err := keeper.EncryptString(""); err == nil {
		t.Error("expected error")
	}
	if _, err := keeper.DecryptString(""); err == nil {
		t.Error("expected error")
	}
}

func TestSecrets__TestStringKeeper(t *testing.T) {
	keeper := TestStringKeeper(t)
	if keeper == nil {
		t.Fatal("nil StringKeeper")
	}
	keeper.Close()
}

func TestOpenSecretKeeper(t *testing.T) {
	ctx := context.Background()

	// Just call these and make sure they don't panic.
	//
	// The result depends on env variables, which in TravisCI is different than local.
	OpenSecretKeeper(ctx, "", "gcp")
	OpenSecretKeeper(ctx, "", "vault")
}
