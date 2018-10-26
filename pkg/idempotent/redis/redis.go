// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

// redis is an immemory Recorder implementation.

package redis

import (
	"context"
	"os"
	"time"

	redis "github.com/gomodule/redigo/redis"
)

var (
	// Used for Redis value
	defaultAddress = "localhost:6379"
	defaultTimeout = 86400 //The default timeout should be 24 hours.
	defaultValue   = struct{}{}
)

func New() *Redis {
	return &Redis{}
}

type Redis struct {
}

func (r *Redis) SeenBefore(key string) bool {
	ctx, _ := context.WithTimeout(context.TODO(), 25*time.Millisecond)
	if addr := os.Getenv("REDIS_INSTANCE"); addr != "" {
		defaultAddress = addr
	}
	conn, err := redis.Dial("tcp", defaultAddress)
	if err != nil {
		ctx = context.WithValue(ctx, "redis dial error", err)
	}
	defer conn.Close()
	conn.Do("WATCH", key)
	seen, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		ctx = context.WithValue(ctx, "redis exist error", err)
	}
	if !seen {
		conn.Do("MULTI")
		_, err := conn.Do("SETEX", key, defaultTimeout, defaultValue)
		conn.Do("EXEC")
		if err != nil {
			ctx = context.WithValue(ctx, "redis set error", err)
		}
	}
	return seen
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
