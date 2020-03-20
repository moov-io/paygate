// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"os"

	"github.com/moov-io/paygate/internal/transfers"
	"github.com/moov-io/paygate/pkg/id"
)

func (c *Controller) removeTransfer(xfer transfers.RemoveTransferRequest) error {
	userID := id.User(xfer.XUserID)

	fileID, err := c.transferRepo.GetFileIDForTransfer(xfer.Transfer.ID, userID)
	if fileID == "" || err != nil {
		return fmt.Errorf("missing fileID for transfer=%s: %v", xfer.Transfer.ID, err)
	}

	file, err := c.loadRemoteACHFile(fileID)
	if err != nil {
		return fmt.Errorf("unable to read file=%s for transfer=%s: %v", fileID, xfer.Transfer.ID, err)
	}

	dep, err := c.depRepo.GetUserDepository(xfer.Transfer.ReceiverDepository, userID)
	if err != nil {
		return fmt.Errorf("missing receiver Depository for transfer=%s: %v", xfer.Transfer.ID, err)
	}

	mergableFile, err := c.grabLatestMergedACHFile(dep.RoutingNumber, file)
	if err != nil {
		return fmt.Errorf("problem getting latest mergable file for transfer=%s: %v", xfer.Transfer.ID, err)
	}

	// If the mergableFile only contains our transfer just delete it and move on
	if len(mergableFile.File.Batches) == 1 {
		return os.Remove(mergableFile.filepath)
	}

	traceNumber, err := c.transferRepo.GetTraceNumber(xfer.Transfer.ID)
	if err != nil {
		return fmt.Errorf("problem getting trace number for transfer=%s: %v", xfer.Transfer.ID, err)
	}

	found := false
	for i := range mergableFile.File.Batches {
		entries := mergableFile.File.Batches[i].GetEntries()
		for k := range entries {
			if entries[k].TraceNumber == traceNumber {
				found = true
				mergableFile.File.RemoveBatch(mergableFile.File.Batches[i])
				break
			}
		}
		if found {
			break
		}
	}
	// found = false
	// for i := range mergableFile.File.IATBatches {
	// 	entries := mergableFile.File.IATBatches[i].Entries
	// 	for k := range entries {
	// 		if entries[k].TraceNumber == traceNumber {
	// 			found = true
	// 			mergableFile.File.IATBatches = append(mergableFile.File.IATBatches[:k], mergableFile.File.IATBatches[k+1:]...)
	// 			break
	// 		}
	// 	}
	// 	if found {
	// 		break
	// 	}
	// }
	if err := mergableFile.File.Create(); err != nil {
		return fmt.Errorf("problem building ACH file: %v", err)
	}
	return mergableFile.write()
}