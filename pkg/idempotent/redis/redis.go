// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

// redis is a simple inmemory Recorder implementation. This implementation
// is intended for simple usecases (local dev) and not production workloads.
package redis

import (
	"context"

	redis "github.com/gomodule/redigo/redis"
)

var (
	// Used for Redis value
	defaultAddress = "localhost:6379"
	defaultValue   = struct{}{}
)

func New() *Redis {
	return &Redis{}
}

type Redis struct {
}

func (r *Redis) SeenBefore(key string) (bool, context.Context) {
	var ctx context.Context
	conn, err := redis.Dial("tcp", defaultAddress)
	if err != nil {
		ctx = context.WithValue(ctx, "redis dial error", err)
	}
	defer conn.Close()
	seen, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		ctx = context.WithValue(ctx, "redis exist error", err)
	}
	if !seen {
		_, err := conn.Do("SET", key, defaultValue)
		if err != nil {
			ctx = context.WithValue(ctx, "redis set error", err)
		}
	}
	return seen, ctx
}

func (r *Redis) FlushAll() error {
	conn, err := redis.Dial("tcp", defaultAddress)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Do("FLUSHALL")
	if err != nil {
		return err
	}
	return nil
}
