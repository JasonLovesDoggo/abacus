import redis, asyncdispatch

proc get*(key: string, incr: bool): Future[string] {.async.} =
  result = await redis.get(await openAsync(host="localhost"), key)
  if incr:
    discard await redis.incr(await openAsync(host="localhost"), key)


proc main() {.async.} =
  let redisClient = await openAsync(host="localhost")

  ## Set the key `nim_redis:test` to the value `Hello, World`
  await redisClient.setk("nim_redis:test", "Hello, World")

  ## Get the value of the key `nim_redis:test`
  let value = await redisClient.get("nim_redis:test")

  assert(value == "Hello, World")

waitFor main()

