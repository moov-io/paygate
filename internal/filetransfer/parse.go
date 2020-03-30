// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
)

func (c *Controller) readFileOrReturn(req *periodicFileOperationsRequest, path string) (*ach.File, error) {
	file, err := parseACHFilepath(path)
	if file == nil || file.Header.ImmediateOrigin == "" {
		// We were unable to read the ACH file, so we can only exit with nothing to return
		return nil, fmt.Errorf("unable to parse %s: %v", path, err)
	}
	if err != nil {
		switch {
		case base.Match(err, &ach.ErrBatchTraceNumberNotODFI{}), base.Match(err, &ach.ErrBatchAddendaTraceNumber{}):
			// {"R27", "Trace number error", "Original entry trace number is not valid for return entry; or addenda trace numbers do not correspond with entry detail record"},
			// TODO(adam): need to return file and send back to ODFI

		case base.Match(err, &ach.FieldError{}):
			// {"R26", "Mandatory field error", "Improper information in one of the mandatory fields"},
			// {"R69", "Field Error(s)", "One or more of the field requirements are incorrect."},
			// TODO(adam): need to return file and send back to ODFI

		case strings.Contains(strings.ToLower(err.Error()), "addenda"),
			base.Match(err, &ach.ErrBatchAddendaCount{}) || base.Match(err, &ach.ErrBatchRequiredAddendaCount{}) || base.Match(err, &ach.ErrBatchExpectedAddendaCount{}):
			// {"R25", "Addenda error", "Improper formatting of the addenda record information"},
			// TODO(adam): need to return file and send back to ODFI

		case strings.Contains(err.Error(), os.ErrPermission.Error()), strings.Contains(err.Error(), os.ErrNotExist.Error()):
			c.logger.Log("readFileOrReturn", fmt.Sprintf("problem reading %s: %v", path, err), "userID", req.userID, "requestID", req.requestID)
			return file, err
		}
	}
	if err2 := file.Validate(); err2 != nil {
		return file, fmt.Errorf("problem validating %s: %v: %v", path, err2, err)
	}
	return file, err
}

func parseACHFilepath(path string) (*ach.File, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	return parseACHFile(fd)
}

func parseACHFile(r io.Reader) (*ach.File, error) {
	file, err := ach.NewReader(r).Read()
	if err != nil {
		return nil, err
	}
	return &file, nil
}
