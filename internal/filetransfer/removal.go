// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"os"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/depository"
	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *Controller) handleRemoval(req interface{}) {
	switch rr := req.(type) {
	case *transfers.RemoveTransferRequest:
		c.logger.Log("handleRemoval", fmt.Sprintf("removing transfer=%s from uploads", rr.Transfer.ID))
		if err := c.removeTransfer(rr); err != nil {
			c.logger.Log("removeTransfer", fmt.Sprintf("ERROR removing transfer: %v", err), "requestID", rr.XRequestID, "userID", rr.XUserID)
		} else {
			c.logger.Log("removeTransfer", fmt.Sprintf("removed transfer=%s", rr.Transfer.ID), "requestID", rr.XRequestID, "userID", rr.XUserID)
		}
		if rr.Waiter != nil {
			rr.Waiter <- struct{}{}
		}

	case *depository.RemoveMicroDeposits:
		c.logger.Log("StartPeriodicFileOperations", fmt.Sprintf("removing micro-deposits for depository=%s from uploads", rr.DepositoryID))
		if err := c.removeMicroDeposit(rr); err != nil {
			c.logger.Log("removeMicroDeposit", fmt.Sprintf("ERROR removing micro-deposits: %v", err), "requestID", rr.XRequestID, "userID", rr.XUserID)
		} else {
			c.logger.Log("removeMicroDeposit", fmt.Sprintf("removed micro-deposits for depository=%s", rr.DepositoryID), "requestID", rr.XRequestID, "userID", rr.XUserID)
		}
		if rr.Waiter != nil {
			rr.Waiter <- struct{}{}
		}

	default:
		c.logger.Log("handleRemoval", fmt.Sprintf("unknown removal message: %T", req))
	}
}

func (c *Controller) removeMicroDeposit(req *depository.RemoveMicroDeposits) error {
	credits, err := c.microDepositRepo.GetMicroDepositsForUser(req.DepositoryID, req.XUserID)
	if err != nil {
		return fmt.Errorf("problem reading micro-deposits: %v", err)
	}

	// micro-deposits are all in the same file, so grab one value and load it
	var fileID string
	for i := range credits {
		if credits[i].FileID != "" {
			fileID = credits[i].FileID
			break
		}
	}
	if fileID == "" {
		c.logger.Log(
			"removeMicroDeposit", fmt.Sprintf("missing fileID for depository=%s", req.DepositoryID),
			"requestID", req.XRequestID, "userID", req.XUserID,
		)
		return nil
	}

	var file *ach.File

	return c.removeBatch(req.DepositoryID, req.XUserID, collectTraceNumbers(file), file)
}

func (c *Controller) removeTransfer(xfer *transfers.RemoveTransferRequest) error {
	userID := id.User(xfer.XUserID)

	fileID, err := c.transferRepo.GetFileIDForTransfer(xfer.Transfer.ID, userID)
	if fileID == "" || err != nil {
		return fmt.Errorf("missing fileID for transfer=%s: %v", xfer.Transfer.ID, err)
	}

	if fileID == "" {
		c.logger.Log(
			"removeTransfer", fmt.Sprintf("missing fileID for transfer=%s", xfer.Transfer.ID),
			"requestID", xfer.XRequestID, "userID", xfer.XUserID,
		)
		return nil
	}

	var file *ach.File

	traceNumber, err := c.transferRepo.GetTraceNumber(xfer.Transfer.ID)
	if err != nil {
		return fmt.Errorf("problem getting trace number for transfer=%s: %v", xfer.Transfer.ID, err)
	}

	return c.removeBatch(xfer.Transfer.ReceiverDepository, userID, []string{traceNumber}, file)
}

// removeBatch inspects an ACH file and attempts in-place removal of the Batch for a given set of TraceNumbers.
//
// We do this in-place mutation because with larger files it can be costly to regenerate the entire file as well
// as we would wipe the previous merged_filename from database rows used to build the file. This is a tradeoff
// where we have chosen to modify the existing file.
func (c *Controller) removeBatch(depID id.Depository, userID id.User, traceNumbers []string, file *ach.File) error {
	dep, err := c.depRepo.GetUserDepository(depID, userID)
	if err != nil {
		return fmt.Errorf("missing receiver depository: %v", err)
	}
	if dep == nil {
		return fmt.Errorf("depository=%s not found", depID)
	}

	mergableFile, err := c.grabLatestMergedACHFile(dep.RoutingNumber, file)
	if err != nil {
		return fmt.Errorf("problem getting latest mergable file: %v", err)
	}

	// If the mergableFile only contains our transfer just delete it and move on
	if len(mergableFile.File.Batches) == 1 {
		return os.Remove(mergableFile.filepath)
	}

	for i := range traceNumbers {
		if err := removeBatch(mergableFile, traceNumbers[i]); err != nil {
			return fmt.Errorf("unable to remove traceNumber=%s: %v", traceNumbers[i], err)
		}
	}

	return nil
}

// removeBatch will look through an ach.File and mutate it to remove all ach.Batch
// records which match a TraceNumber.
func removeBatch(mergableFile *achFile, traceNumber string) error {
	found := true
	for i := range mergableFile.File.Batches {
		entries := mergableFile.File.Batches[i].GetEntries()
		for k := range entries {
			if entries[k].TraceNumber == traceNumber {
				if len(mergableFile.File.Batches) == 1 {
					return os.Remove(mergableFile.filepath)
				}

				found = true
				mergableFile.File.RemoveBatch(mergableFile.File.Batches[i])
				goto finish
			}
		}
	}
finish:
	if found {
		if err := mergableFile.File.Create(); err != nil {
			return fmt.Errorf("problem building ACH file: %v", err)
		}
		return mergableFile.write()
	}
	return nil
}
