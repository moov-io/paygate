// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
)

func SumAmounts(amounts ...string) (*Amount, error) {
	total, _ := NewAmount("USD", "0.00")
	for i := range amounts {
		amt, _ := NewAmount("USD", "0.00")
		if err := amt.FromString(amounts[i]); err != nil {
			return nil, fmt.Errorf("problem reading '%s': %v", amounts[i], err)
		}
		sum, _ := total.Plus(*amt)
		total = &sum
	}
	return total, nil
}
