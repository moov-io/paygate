// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package secrets

import (
	"context"
	"strings"
	"testing"
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
}
