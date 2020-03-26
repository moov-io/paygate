// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
)

// TODO(adam): handle incoming file as a transfer (reconcile against Depositories, create Transfer row, NSF/return, etc..)

func (c *Controller) handleIncomingTransfer(req *periodicFileOperationsRequest, file *ach.File, filename string) error {
	c.logger.Log("handleIncomingTransfer", fmt.Sprintf("incoming ACH file %s", filename), "userID", req.userID, "requestID", req.requestID)

	for i := range file.Batches {
		entries := file.Batches[i].GetEntries()
		for j := range entries {

			fmt.Printf("rtn=%s acct=%s\n", file.Header.ImmediateDestination, entries[j].DFIAccountNumber)

			dep, err := c.depRepo.LookupDepositoryForIncoming(file.Header.ImmediateDestination, entries[j].DFIAccountNumber, entries[j].IndividualName)
			if err != nil {
				c.logger.Log(
					"handleIncomingTransfer", fmt.Sprintf("unable to find depository: %v", err),
					"userID", req.userID, "requestID", req.requestID)
				continue
			}
			if dep == nil || dep.ID == "" {
				c.logger.Log(
					"handleIncomingTransfer", fmt.Sprintf("depository not found traceNumber=%s", entries[j].TraceNumber),
					"userID", req.userID, "requestID", req.requestID)
				continue
			}

			fmt.Printf("dep=%#v\n", dep)
		}
	}

	return nil
}
