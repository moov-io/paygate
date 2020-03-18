// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package microdeposit

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/moov-io/paygate/internal/model"
)

type Credit struct {
	Amount        model.Amount
	FileID        string
	TransactionID string
}

func (m Credit) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount model.Amount `json:"amount"`
	}{
		m.Amount,
	})
}

func microDepositAmounts() []model.Amount {
	rand := func() int {
		n, _ := rand.Int(rand.Reader, big.NewInt(49)) // rand.Int returns [0, N) and we want a range of $0.01 to $0.50
		return int(n.Int64()) + 1
	}
	// generate two amounts and a third that's the sum
	n1, n2 := rand(), rand()
	a1, _ := model.NewAmount("USD", fmt.Sprintf("0.%02d", n1)) // pad 1 to '01'
	a2, _ := model.NewAmount("USD", fmt.Sprintf("0.%02d", n2))
	return []model.Amount{*a1, *a2}
}
