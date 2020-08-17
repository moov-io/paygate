// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/moov-io/paygate/examples/common"
	"log"
)

func main() {
	fmt.Printf("using RequestID %s\n\n", common.RequestID)

	// Create source customer
	customer, err := common.CreateCustomer("Micro", "Deposits", "valiation@company.io")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created micro-deposit source customer %s\n", customer.CustomerID)

	// Approve customer
	customer, err = common.ApproveCustomer(customer)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Customer status is %s\n", customer.Status)

	// Create account
	account, err := common.CreateAccount(customer, "123456", common.TeachersFCU, "Checking")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created customer account %s\n", account.AccountID)

	// Approve account
	_, err = common.ApproveAccount(customer, account)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Println("Approved micro-deposit source account")
	fmt.Println("===========")

	fmt.Println("In ./examples/config.yaml replace the 'validation:' YAML block with:")
	fmt.Println("validation:")
	fmt.Println("  microDeposits:")
	fmt.Println("    source:")
	fmt.Printf("      customerID: \"%s\"\n", customer.CustomerID)
	fmt.Printf("      accountID: \"%s\"\n", account.AccountID)
	fmt.Println("===========")
	fmt.Println("")
	fmt.Println("Restart PayGate with 'docker-compose up' and run go run /examples/micro-deposits/main.go")
}
