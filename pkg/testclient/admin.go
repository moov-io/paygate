// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package testclient

import (
	"testing"

	moovadmin "github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/pkg/admin"
)

func Admin(t *testing.T) (*moovadmin.Server, *admin.APIClient) {
	svc := moovadmin.NewServer(":0")
	go svc.Listen()
	t.Cleanup(func() { svc.Shutdown() })

	conf := admin.NewConfiguration()
	conf.BasePath = "http://" + svc.BindAddr()

	return svc, admin.NewAPIClient(conf)
}
