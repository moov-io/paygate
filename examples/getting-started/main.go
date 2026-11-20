// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/moov-io/paygate/examples/common"
)

func main() {
	fmt.Printf("using requestID %s\n\n", common.RequestID)

	// Create source customer
	sourceCustomer, err := common.CreateCustomer("John", "Doe", "john.doe@moov.io")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created source customer %s\n", sourceCustomer.CustomerID)

	// Approve customer
	sourceCustomer, err = common.ApproveCustomer(sourceCustomer)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Customer status is %s\n", sourceCustomer.Status)

	// Create account
	sourceAccount, err := common.CreateAccount(sourceCustomer, "123456", common.TeachersFCU, "Savings")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created source customer account %s\n", sourceAccount.AccountID)

	// Approve account
	_, err = common.ApproveAccount(sourceCustomer, sourceAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Println("Approved source account")
	fmt.Println("===========")

	// Create destination customer
	destinationCustomer, err := common.CreateCustomer("Jane", "Doe", "jane.doe@moov.io")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created destination customer %s\n", destinationCustomer.CustomerID)

	// Approve customer
	destinationCustomer, err = common.ApproveCustomer(destinationCustomer)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Customer status is %s\n", destinationCustomer.Status)

	// Create account
	destinationCustomerAccount, err := common.CreateAccount(destinationCustomer, "654321", common.ChaseCO, "Checking")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created destination customer account %s\n", destinationCustomerAccount.AccountID)

	// Approve account
	_, err = common.ApproveAccount(destinationCustomer, destinationCustomerAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Println("Approved destination account")
	fmt.Println("===========")

	// Initiate a transfer
	transfer, err := common.MakeTransfer(sourceCustomer, sourceAccount, destinationCustomer, destinationCustomerAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Transfer id is %s\n", transfer.TransferID)

	// Wait for the transfer to be merged and uploaded. Triggering cutoff immediately
	// after creation can race the merge step and leave the transfer pending.
	transfer, err = common.WaitUntilProcessed(transfer.TransferID, 15*time.Second)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	fmt.Println("===========")
	var indentedJson, _ = json.MarshalIndent(transfer, "", "  ")
	fmt.Println(string(indentedJson))
	fmt.Println("")
	fmt.Println("Success! A Transfer was created.")
	fmt.Println("An ACH file was uploaded to the test FTP server.")
	fmt.Println("docker compose mounts ./testdata/ftp-server into the FTP container at /data,")
	fmt.Println("so uploaded files appear on the host at ./testdata/ftp-server/outbound/")
	fmt.Println("")
	fmt.Println("Uploaded files:")
	common.PrintServerFiles(filepath.Join("testdata", "ftp-server", "outbound"))
}
