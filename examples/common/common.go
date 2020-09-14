// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/customers/cmd/server/accounts/validator"
	customers "github.com/moov-io/customers/pkg/client"
	"github.com/moov-io/paygate/pkg/client"
)

// Common functions and values for reuse between PayGate go examples
var (
	HttpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	RequestID   = base.ID()
	TeachersFCU = "221475786"
	ChaseCO     = "102001017"
)

func CreateCustomer(first, last, email string) (*customers.Customer, error) {
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
	req.Header.Add("x-request-id", RequestID)
	resp, err := HttpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	var cust customers.Customer
	if err := json.NewDecoder(resp.Body).Decode(&cust); err != nil {
		return nil, err
	}
	return &cust, err
}

func ApproveCustomer(customer *customers.Customer) (*customers.Customer, error) {
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
	req.Header.Add("x-request-id", RequestID)
	resp, err := HttpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var cust customers.Customer
	if err := json.NewDecoder(resp.Body).Decode(&cust); err != nil {
		return nil, err
	}
	return &cust, err
}

func CreateAccount(customer *customers.Customer, accountNumber, routingNumber, acctType string) (*customers.Account, error) {
	jsonData := map[string]string{"accountNumber": accountNumber, "routingNumber": routingNumber, "type": acctType}
	url := "http://localhost:8087/customers/" + customer.CustomerID + "/accounts"
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	req.Header.Add("x-request-id", RequestID)
	resp, err := HttpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	var acct customers.Account
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return nil, err
	}
	return &acct, err
}

func ApproveAccount(customer *customers.Customer, account *customers.Account) (bool, error) {
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
	req.Header.Add("x-request-id", RequestID)
	resp, err := HttpClient.Do(req)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 200 {
		return true, err
	}
	return false, err
}

func InitiateMicroDeposits(customer *customers.Customer, account *customers.Account) (string, error) {
	params := &customers.InitAccountValidationRequest{
		Strategy: "test",
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(params); err != nil {
		t.Fatal(err)
	}
	body := bytes.NewReader(buf.Bytes())
	url := "http://localhost:8087/customers/" + customer.CustomerID + "/accounts/" + account.AccountID + "/validations"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}
	req.Header.Add("x-request-id", RequestID)
	req.Header.Add("x-user-id", "moov")
	resp, err := HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("micro deposits failed")
	}

	var response customers.CompleteAccountValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	return response.ValidationID, nil
}

func GetMicroDeposits(account *customers.Account) (*client.MicroDeposits, error) {
	req, err := http.NewRequest("GET", "http://localhost:8082/accounts/"+account.AccountID+"/micro-deposits", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-request-id", RequestID)
	req.Header.Add("x-user-id", "moov")
	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	jsonVal, err := getJSONResponse(resp)
	var microDeposits client.MicroDeposits
	if err := json.Unmarshal([]byte(jsonVal), &microDeposits); err != nil {
		return nil, err
	}
	return &microDeposits, err
}

func MakeTransfer(sourceCustomer *customers.Customer, sourceCustomerAccount *customers.Account, destCustomer *customers.Customer, destCustomerAccount *customers.Account) (*client.Transfer, error) {
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

	req, err := http.NewRequest("POST", "http://localhost:8082/transfers", bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-user-id", "moov")
	req.Header.Add("x-request-id", RequestID)
	resp, err := HttpClient.Do(req)
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

func GetTransfer(transferId string) (*client.Transfer, error) {
	req, err := http.NewRequest("GET", "http://localhost:8082/transfers/"+transferId, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-user-id", "moov")
	resp, err := HttpClient.Do(req)
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

func GetCustomerAccounts(customer *customers.Customer) ([]*customers.Account, error) {
	req, err := http.NewRequest("GET", "http://localhost:8087/customers/"+customer.CustomerID+"/accounts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-request-id", RequestID)
	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	jsonVal, err := getJSONResponse(resp)
	var accounts []*customers.Account
	if err := json.Unmarshal([]byte(jsonVal), &accounts); err != nil {
		return nil, err
	}
	return accounts, err
}

func VerifyMicroDeposits(customer *customers.Customer, account *customers.Account, validationID string, microDeposits *client.MicroDeposits) (bool, error) {
	validateRequest := &customers.CompleteAccountValidationRequest{
		VendorRequest: validator.VendorRequest{
			"micro-deposits": microDeposits.Amounts,
		},
	}

	jsonValue, err := json.Marshal(validateRequest)
	if err != nil {
		return false, err
	}
	url := "http://localhost:8087/customers/" + customer.CustomerID + "/accounts/" + account.AccountID + "/validations/" + validationID
	req, err := http.NewRequest("POST",
		url,
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		return false, err
	}
	req.Header.Add("x-request-id", RequestID)
	req.Header.Add("x-user-id", "moov")
	resp, err := HttpClient.Do(req)

	if err != nil {
		return false, err
	}
	return resp.StatusCode == 200, err
}

func TriggerCutOff() (bool, error) {
	var body bytes.Buffer
	url := "http://localhost:9092/trigger-cutoff"
	req, err := http.NewRequest("PUT", url, &body)
	if err != nil {
		return false, err
	}
	resp, err := HttpClient.Do(req)
	if err != nil {
		return false, err
	}
	return resp.StatusCode == 200, nil
}

func PrintServerFiles(path string) {
	infos, err := filepath.Glob(filepath.Join(path, "*.ach"))
	if err != nil {
		panic(err)
	}
	for i := range infos {
		fmt.Println(infos[i])
	}
}

// helper func for getting json response bodies
func getJSONResponse(response *http.Response) (string, error) {
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)
	return bodyString, nil
}
