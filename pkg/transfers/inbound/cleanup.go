// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/moov-io/base"

	"github.com/moov-io/paygate/pkg/upload"

	"github.com/moov-io/base/log"
)

// Cleanup deletes files if enabled via config
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

// CleanupEmptyFiles deletes empty ACH files if file is older than value in config
func CleanupEmptyFiles(logger log.Logger, agent upload.Agent, dl *downloadedFiles, sourceTime time.Time, after time.Duration) error {
	var el base.ErrorList

	if after <= 0*time.Second {
		logger.Log(fmt.Sprintf("deleting empty file requires after > 0. currently: %s", after))
		return nil
	}

	if err := deleteEmptyFiles(logger, agent, dl.dir, agent.InboundPath(), sourceTime, after); err != nil {
		el.Add(err)
	}
	if err := deleteEmptyFiles(logger, agent, dl.dir, agent.ReturnPath(), sourceTime, after); err != nil {
		el.Add(err)
	}
	if el.Empty() {
		return nil
	}
	return el
}

// deleteFilesOnRemote deletes all files for a given directory
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

// deleteEmptyFiles deletes all empty files that are older than after (time.Duration)
func deleteEmptyFiles(logger log.Logger, agent upload.Agent, localDir, suffix string, sourceTime time.Time, after time.Duration) error {
	baseDir := filepath.Join(localDir, suffix)
	infos, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("reading %s: %v", baseDir, err)
	}

	var el base.ErrorList
	for i := range infos {
		fileInfo := infos[i]
		path := filepath.Join(suffix, filepath.Base(infos[i].Name()))
		if shouldDeleteEmptyFile(fileInfo, sourceTime, after) {
			err := agent.Delete(path)
			if err != nil {
				el.Add(err)
			} else {
				logger.Log(fmt.Sprintf("deleted zero byte file %s", path))
			}
		} else {
			logger.Log(fmt.Sprintf("zero byte file not deleted %s", path))
		}
	}

	if el.Empty() {
		return nil
	}
	return el
}

// shouldDeleteEmptyFile determines if a file is empty and if it should be deleted
// per the config setting RemoveEmptyFileAfter
func shouldDeleteEmptyFile(info os.FileInfo, sourceTime time.Time, removeEmptyFileAfter time.Duration) bool {
	if info.Size() != 0 {
		return false
	}
	diff := sourceTime.Sub(info.ModTime())
	return diff.Minutes() >= removeEmptyFileAfter.Minutes()
}
