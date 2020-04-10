// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/achx"
	"github.com/moov-io/paygate/internal/depository/verification/microdeposit"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/pkg/id"
)

func collectTraceNumbers(f *ach.File) []string {
	var out []string
	for i := range f.Batches {
		entries := f.Batches[i].GetEntries()
		for k := range entries {
			out = append(out, entries[k].TraceNumber)
		}
	}
	for i := range f.IATBatches {
		for k := range f.IATBatches[i].Entries {
			out = append(out, f.IATBatches[i].Entries[k].TraceNumber)
		}
	}
	return out
}

func (c *Controller) makeFileFromTransfer(userID id.User, transferID id.Transfer) (*ach.File, error) {
	transfer, err := c.transferRepo.GetTransfer(transferID)
	if err != nil {
		return nil, err
	}
	originator, origDep, receiver, recDep := c.getTransferObjects(userID, transfer)
	if originator == nil || origDep == nil || receiver == nil || recDep == nil {
		return nil, err
	}
	gateway, err := c.gatewayRepo.GetUserGateway(userID)
	if gateway == nil || err != nil {
		return nil, fmt.Errorf("gateway=%#v error=%v", gateway, err)
	}

	file, err := achx.ConstructFile(string(transferID), gateway, transfer, originator, origDep, receiver, recDep)
	if err != nil {
		return nil, err
	}
	if err := file.Create(); err != nil {
		return nil, err
	}
	return file, nil
}

func (c *Controller) makeFileFromMicroDeposit(mc microdeposit.UploadableCredit) (*ach.File, error) {
	gateway, err := c.gatewayRepo.GetUserGateway(id.User(mc.UserID))
	if gateway == nil || err != nil {
		return nil, fmt.Errorf("gateway=%#v error=%v", gateway, err)
	}

	receiver := &model.Receiver{
		ID:     model.ReceiverID(fmt.Sprintf("%s-micro-deposit-verify", base.ID())),
		Status: model.ReceiverVerified, // Something to pass constructACHFile validation logic
		// Metadata: dep.Holder,             // Depository holder is getting the micro deposit
	}
	receiverDep, err := c.depRepo.GetDepository(id.Depository(mc.DepositoryID))
	if err != nil {
		return nil, fmt.Errorf("problem getting depository=%s: %v", mc.DepositoryID, err)
	}

	orig, origDep := c.odfiAccount.Metadata()
	transfer := &model.Transfer{
		Originator:             orig.ID, // e.g. Moov, Inc
		OriginatorDepository:   origDep.ID,
		Type:                   model.PushTransfer,
		StandardEntryClassCode: ach.PPD,
		Status:                 model.TransferPending,
		UserID:                 mc.UserID,
	}

	file, err := achx.ConstructFile("", gateway, transfer, orig, origDep, receiver, receiverDep)
	if err != nil {
		return nil, err
	}
	if err := file.Create(); err != nil {
		return nil, err
	}
	return file, nil
}

func (c *Controller) getTransferObjects(userID id.User, transfer *model.Transfer) (*model.Originator, *model.Depository, *model.Receiver, *model.Depository) {
	// Originator
	orig, err := c.origRepo.GetUserOriginator(transfer.Originator, userID)
	if orig == nil || err != nil {
		return nil, nil, nil, nil
	}
	if err := orig.Validate(); err != nil {
		return nil, nil, nil, nil
	}

	origDep, err := c.depRepo.GetUserDepository(transfer.OriginatorDepository, userID)
	if origDep == nil || err != nil {
		return nil, nil, nil, nil
	}
	if err := origDep.Validate(); err != nil {
		return nil, nil, nil, nil
	}

	// Receiver
	receiver, err := c.receiverRepository.GetUserReceiver(transfer.Receiver, userID)
	if receiver == nil || err != nil {
		return nil, nil, nil, nil
	}
	if err := receiver.Validate(); err != nil {
		return nil, nil, nil, nil
	}

	receiverDep, err := c.depRepo.GetUserDepository(transfer.ReceiverDepository, userID)
	if receiverDep == nil || err != nil {
		return nil, nil, nil, nil
	}
	if err := receiverDep.Validate(); err != nil {
		return nil, nil, nil, nil
	}

	return orig, origDep, receiver, receiverDep
}
