package utils

import "github.com/redis/go-redis/v9"

// IncrByIfExists atomically increments KEYS[1] by ARGV[1] only if the key
// already exists. Returns the new value, or nil (redis.Nil) if the key was
// missing. Collapses the prior EXISTS+INCRBY pair into one RTT and removes
// the TOCTOU window between them.
var IncrByIfExists = redis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 0 then
  return nil
end
return redis.call("INCRBY", KEYS[1], ARGV[1])
`)

// CreateWithAdmin atomically creates the counter key and writes the admin key
// in a single RTT. KEYS[1]=counter, KEYS[2]=admin, ARGV[1]=initialValue,
// ARGV[2]=ttlSeconds, ARGV[3]=adminToken. Returns 1 if created, 0 if the
// counter already existed (in which case the admin key is untouched, so the
// original owner keeps control).
var CreateWithAdmin = redis.NewScript(`
if redis.call("SET", KEYS[1], ARGV[1], "NX", "EX", ARGV[2]) == false then
  return 0
end
redis.call("SET", KEYS[2], ARGV[3])
return 1
`)
