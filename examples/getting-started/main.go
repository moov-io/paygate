// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/moov-io/paygate/pkg/client"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/client"
)

const (
	teachersFCU = "221475786" // ODFI routing number
	chaseCO     = "102001017"
)

var (
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	requestID = base.ID()
)

func main() {

	fmt.Printf("using requestID %s\n\n", requestID)

	// Create source customer
	sourceCustomer, err := createCustomer("John", "Doe", "john.doe@moov.io")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created source customer %s\n", sourceCustomer.CustomerID)

	// Approve customer
	sourceCustomer, err = approveCustomer(sourceCustomer)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Customer status is %s\n", sourceCustomer.Status)

	// Create account
	sourceAccount, err := createAccount(sourceCustomer, teachersFCU, "Savings")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created source customer account %s\n", sourceAccount.AccountID)

	// Approve account
	_, err = approveAccount(sourceCustomer, sourceAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Println("Approved source account")
	fmt.Println("===========")

	// Create destination customer
	destinationCustomer, err := createCustomer("Jane", "Doe", "jane.doe@moov.io")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created destination customer %s\n", destinationCustomer.CustomerID)

	// Approve customer
	destinationCustomer, err = approveCustomer(destinationCustomer)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Customer status is %s\n", destinationCustomer.Status)

	// Create account
	destinationCustomerAccount, err := createAccount(destinationCustomer, chaseCO, "Checking")
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Created destination customer account %s\n", destinationCustomerAccount.AccountID)

	// Approve account
	_, err = approveAccount(destinationCustomer, destinationCustomerAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Println("Approved destination account")
	fmt.Println("===========")

	// Initiate a transfer
	transfer, err := makeTransfer(sourceCustomer, sourceAccount, destinationCustomer, destinationCustomerAccount)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Printf("Transfer id is %s\n", transfer.TransferID)
	_, err = triggerCutOff()
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	fmt.Println("===========")

	// Get transfer and display
	transfer, err = getTransfer(transfer.TransferID)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var indentedJson, _ = json.MarshalIndent(transfer, "", "  ")
	fmt.Println(string(indentedJson))
	fmt.Println("Success! A Transfer was created.")
	fmt.Println("An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/")
	printServerFiles(filepath.Join("testdata", "ftp-server", "outbound"))
}

func createCustomer(first, last, email string) (*customers.Customer, error) {
	params := &customers.CreateCustomer{
		FirstName: first,
		LastName:  last,
		Type:      customers.INDIVIDUAL,
		Email:     email,
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(params); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "http://localhost:8087/customers", &body)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	req.Header.Add("x-request-id", requestID)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	var cust customers.Customer
	if err := json.NewDecoder(resp.Body).Decode(&cust); err != nil {
		return nil, err
	}
	return &cust, err
}

func approveCustomer(customer *customers.Customer) (*customers.Customer, error) {
	jsonData := map[string]string{"status": "verified"}
	url := "http://localhost:9097/customers/" + customer.CustomerID + "/status"
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	req.Header.Add("x-request-id", requestID)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var cust customers.Customer
	if err := json.NewDecoder(resp.Body).Decode(&cust); err != nil {
		return nil, err
	}
	return &cust, err
}

func createAccount(customer *customers.Customer, routingNumber, acctType string) (*customers.Account, error) {
	jsonData := map[string]string{"accountNumber": routingNumber, "routingNumber": routingNumber, "type": acctType}
	url := "http://localhost:8087/customers/" + customer.CustomerID + "/accounts"
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}

	fmt.Println("url: " + url)
	fmt.Println("json: " + string(jsonValue))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	req.Header.Add("x-request-id", requestID)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var acct customers.Account
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return nil, err
	}
	return &acct, err
}

func approveAccount(customer *customers.Customer, account *customers.Account) (bool, error) {
	jsonData := map[string]string{"status": "validated"}
	url := "http://localhost:9097/customers/" + customer.CustomerID + "/accounts/" + account.AccountID + "/status"
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonValue))
	if err != nil {
		return false, err
	}
	req.Header.Add("x-request-id", requestID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 200 {
		return true, err
	}
	return false, err
}

func makeTransfer(sourceCustomer *customers.Customer, sourceCustomerAccount *customers.Account, destCustomer *customers.Customer, destCustomerAccount *customers.Account) (*client.Transfer, error) {
	var jsonData = map[string]interface{}{
		"amount": "USD 1.25",
		"source": map[string]string{
			"customerID": sourceCustomer.CustomerID,
			"accountID":  sourceCustomerAccount.AccountID,
		},
		"destination": map[string]string{
			"customerID": destCustomer.CustomerID,
			"accountID":  destCustomerAccount.AccountID,
		},
		"description": "test transfer",
	}

	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}
	fmt.Println("json: " + string(jsonValue))

	req, err := http.NewRequest("POST", "http://localhost:8082/transfers", bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-user-id", "moov")
	req.Header.Add("x-request-id", requestID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	jsonVal, err := getJSONResponse(resp)
	if err != nil {
		return nil, err
	}
	var transfer client.Transfer
	if err := json.Unmarshal([]byte(jsonVal), &transfer); err != nil {
		return nil, err
	}
	return &transfer, err
}

func getTransfer(transferId string) (*client.Transfer, error) {
	req, err := http.NewRequest("GET", "http://localhost:8082/transfers/"+transferId, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-user-id", "moov")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	jsonVal, err := getJSONResponse(resp)
	var transfer client.Transfer
	if err := json.Unmarshal([]byte(jsonVal), &transfer); err != nil {
		return nil, err
	}
	return &transfer, err
}

func triggerCutOff() (bool, error) {
	var body bytes.Buffer
	url := "http://localhost:9092/trigger-cutoff"
	req, err := http.NewRequest("PUT", url, &body)
	if err != nil {
		return false, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 200 {
		return true, err
	}
	return false, err
}

func printServerFiles(path string) {
	var files []string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		fmt.Println(file)
	}
}

// helper func for printing json response bodies
func getJSONResponse(response *http.Response) (string, error) {
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)
	fmt.Println("body: " + bodyString)
	return bodyString, nil
}
