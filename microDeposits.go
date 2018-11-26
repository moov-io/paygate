// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	// TODO(Adam): remove, this is just for testing
	zzone, _                 = NewAmount("USD", "0.01")
	zzthree, _               = NewAmount("USD", "0.03")
	fixedMicroDepositAmounts = []Amount{*zzone, *zzthree}
)

// 200 - Micro deposits verified // w.WriteHeader(http.StatusOK)
// 201 - Micro deposits initiated // TODO(adam): just do 200 also ? // w.WriteHeader(http.StatusCreated)
// 400 - Invalid Amounts // w.WriteHeader(http.StatusBadRequest)
// 404 - A depository with the specified ID was not found. // w.WriteHeader(http.StatusNotFound)
// 409 - Too many attempts. Bank already verified. // w.WriteHeader(http.StatusConflict)

// initiateMicroDeposits will write micro deposits into the underlying database and kick off the ACH transfer(s).
//
// Note: No money is actually transferred yet. Only fixedMicroDepositAmounts amounts are written
func initiateMicroDeposits(repo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "initiateMicroDeposits")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Write micro deposits into our db
		if err := repo.initiateMicroDeposits(id, userId, fixedMicroDepositAmounts); err != nil {
			internalError(w, err, "initiateMicroDeposits")
			return
		}

		// TODO: whatever is needed to actually transfer money

		w.WriteHeader(http.StatusOK)
	}
}

type confirmDepositoryRequest struct {
	Amounts []string `json:"amounts"`
}

// confirmMicroDeposits checks our database for a depository's micro deposits (used to validate the user owns the Depository)
// and if successful changes the Depository status to DepositoryVerified.
func confirmMicroDeposits(repo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "initiateMicroDeposits")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Read amounts from request JSON
		var req confirmDepositoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			encodeError(w, err)
			return
		}

		var amounts []Amount
		for i := range req.Amounts {
			amt := &Amount{}
			if err := amt.FromString(req.Amounts[i]); err != nil {
				continue
			}
			amounts = append(amounts, *amt)
		}
		if err := repo.confirmMicroDeposits(id, userId, amounts); err != nil {
			encodeError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// getMicroDeposits will retrieve the micro deposits for a given depository. If an amount does not parse it will be discardded silently.
func (r *sqliteDepositoryRepo) getMicroDeposits(id DepositoryID, userId string) ([]Amount, error) {
	query := `select amount from micro_deposits where user_id = ? and depository_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(userId, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var amounts []Amount
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			continue
		}

		amt := &Amount{}
		if err := amt.FromString(value); err != nil {
			continue
		}
		amounts = append(amounts, *amt)
	}

	return amounts, nil
}

// initiateMicroDeposits will save the provided []Amount into our database. If amounts have already been saved then
// no new amounts will be added.
func (r *sqliteDepositoryRepo) initiateMicroDeposits(id DepositoryID, userId string, amounts []Amount) error {
	existing, err := r.getMicroDeposits(id, userId)
	if err != nil || len(existing) > 0 {
		return fmt.Errorf("not initializing more micro deposits, already have %d or got error=%v", len(existing), err)
	}

	// write amounts
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now, query := time.Now(), `insert into micro_deposits (depository_id, user_id, amount, created_at) values (?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range amounts {
		_, err = stmt.Exec(id, userId, amounts[i].String(), now) // write amount into db
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// confirmMicroDeposits will compare the provided guessAmounts against what's been persisted for a user. If the amounts do not match
// or there are a mismatched amount the call will return a non-nil error.
func (r *sqliteDepositoryRepo) confirmMicroDeposits(id DepositoryID, userId string, guessAmounts []Amount) error {
	createdAmounts, err := r.getMicroDeposits(id, userId)
	if err != nil || len(createdAmounts) == 0 {
		return fmt.Errorf("unable to confirm micro deposits, got %d micro deposits or error=%v", len(createdAmounts), err)
	}

	// Check amounts, all must match
	if (len(guessAmounts) < len(createdAmounts)) || len(guessAmounts) == 0 {
		return fmt.Errorf("incorrect amount of guesses, got %d", len(guessAmounts)) // don't share len(createdAmounts), that's an info leak
	}

	found := 0
	for i := range createdAmounts {
		for k := range guessAmounts {
			if createdAmounts[i].Equal(guessAmounts[k]) {
				found += 1
			}
		}
	}

	if found != len(createdAmounts) {
		return errors.New("incorrect micro deposit guesses")
	}

	return nil
}
