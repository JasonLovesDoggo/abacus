import redis
from ./exceptions import ConnectionError, KeyNotUnique
from ./config import DEFAULT_KEY_LIFE
import ./types
import asyncdispatch

proc create(session: Redis, key: string): RedisString =
  if session.exists(key):
    raise newException(KeyNotUnique, "Key already exists")
  return session.setEX(key, DEFAULT_KEY_LIFE, "0")

proc get*(session: Redis, key: string, incr: bool): RedisString =

  result = session.get(key)
  if result == redisNil:
    result = create(session, key)
  if incr:
    discard session.incr(key)


proc stats*(session: Redis): StatsObj =
  let keyCount: RedisInteger = session.dbsize()
  let lastSave: RedisInteger = session.lastsave()

  echo "Key count: ", keyCount
  echo "Last save: ", lastSave

  return StatsObj(key_count: keyCount, last_save: lastSave)
