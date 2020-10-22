// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

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

	_, err = common.TriggerCutOff()
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	fmt.Println("===========")

	// Get transfer and display
	transfer, err = common.GetTransfer(transfer.TransferID)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var indentedJson, _ = json.MarshalIndent(transfer, "", "  ")
	fmt.Println(string(indentedJson))
	fmt.Println("")
	fmt.Println("Success! A Transfer was created.")
	fmt.Println("An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/")
	fmt.Println("")
	fmt.Println("Uploaded files:")
	common.PrintServerFiles(filepath.Join("testdata", "ftp-server", "outbound"))
}
