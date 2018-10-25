// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package idempotent

import "context"

// Recorder offers a method to determine if a given key has been
// seen before or not. Each invocation of SeenBefore needs to
// record each key found, but there's no minimum duration required.
type Recorder interface {
	SeenBefore(key string) (bool, context.Context)
}
