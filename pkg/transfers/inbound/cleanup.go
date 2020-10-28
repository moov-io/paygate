// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/moov-io/base"

	"github.com/moov-io/paygate/pkg/upload"

	"github.com/moov-io/base/log"
)

func Cleanup(logger log.Logger, agent upload.Agent, dl *downloadedFiles) error {
	var el base.ErrorList

	if err := deleteFilesOnRemote(logger, agent, dl.dir, agent.InboundPath()); err != nil {
		el.Add(err)
	}
	if err := deleteFilesOnRemote(logger, agent, dl.dir, agent.ReturnPath()); err != nil {
		el.Add(err)
	}

	if el.Empty() {
		return nil
	}
	return el
}

func deleteFilesOnRemote(logger log.Logger, agent upload.Agent, localDir, suffix string) error {
	baseDir := filepath.Join(localDir, suffix)
	infos, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("reading %s: %v", baseDir, err)
	}

	var el base.ErrorList
	for i := range infos {
		path := filepath.Join(suffix, filepath.Base(infos[i].Name()))
		if err := agent.Delete(path); err != nil {
			el.Add(err)
		} else {
			logger.Logf("cleanup: deleted remote file %s", path)
		}
	}

	if el.Empty() {
		return nil
	}
	return el
}
