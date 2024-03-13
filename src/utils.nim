import exceptions
from database as db import nil
import asyncdispatch

proc generateKey(namespace: string, key: string): string =
  return namespace & ":" & key

proc hit*(namespace: string, key: string): Future[string] {.async.} =
    let key = generateKey(namespace, key)
    let value = await db.get(key, incr=true)
    if value.isNil:
        raise (ref exceptions.KeyNotFound)(msg: "The key $key or namespace $namespace could not be found. Please check your input and try again.")
    return value

