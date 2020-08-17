// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"github.com/moov-io/paygate/examples/common"
	"log"
	"path/filepath"
)

func main() {
	fmt.Printf("using RequestID %s\n\n", common.RequestID)

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

	// Create destination account
	destinationCustomerAccount, err := common.CreateAccount(destinationCustomer, "654321", common.ChaseCO, "Checking")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created destination customer account %s\n", destinationCustomerAccount.AccountID)

	fmt.Println("Initiating micro-deposits...")
	depositSuccess, err := common.InitiateMicroDeposits(destinationCustomer, destinationCustomerAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	if !depositSuccess {
		log.Fatalf("ERROR: %v", "micro deposits failed")
	}
	fmt.Println("Initiated micro-deposits for destination account")

	// Trigger cutoff
	_, err = common.TriggerCutOff()
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	microDeposits, err := common.GetMicroDeposits(destinationCustomerAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var indentedJson, _ = json.MarshalIndent(microDeposits, "", "  ")
	fmt.Println("Found micro-deposits:" + string(indentedJson))

	// Verify Micro Deposits
	_, err = common.VerifyMicroDeposits(destinationCustomer, destinationCustomerAccount, microDeposits)
	if err != nil {
		log.Fatalf("ERROR verifying micro deposits: %v", err)
	}

	// Get customer accounts
	accounts, err := common.GetCustomerAccounts(destinationCustomer)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var accountJson, _ = json.MarshalIndent(accounts[0], "", "  ")
	fmt.Println("Customer accounts:" + string(accountJson))

	fmt.Println("Success! The account was validated with micro-deposits")
	fmt.Println("")
	fmt.Println("An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/")
	common.PrintServerFiles(filepath.Join("testdata", "ftp-server", "outbound"))
}
