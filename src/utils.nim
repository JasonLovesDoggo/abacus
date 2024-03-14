import redis

proc generateKey*(namespace: string, key: string): string =
  return namespace & ":" & key


proc getRedisSession(): redis.Redis =
  let redisClient = redis.open(host="localhost")

  discard redisClient.ping()
  # if not redisClient.connected:
  #   raise newException(ConnectionError, "Could not connect to Redis")

  result = redisClient


let session* = getRedisSession()
