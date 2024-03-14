import prologue
import std/json
import ./database
import ./types
import ./utils

const docs_url: string = "https://github.com/JasonLovesDoggo/abacus/blob/master/docs/ROUTES.md"


proc docsView*(ctx: Context) {.async.} =
  resp redirect(docs_url)

proc statsView*(ctx: Context) {.async.} =
  # let dbStats = stats(session)
  # let jsonData = %* {
  #   "total_keys": dbStats.key_count,
  #   "last_save": dbStats.last_save
  # }
  # echo(jsonData)
  resp "<h1>Hello, Prologue!</h1>"


proc hitView*(ctx: Context) {.async.} =
  let key = ctx.getPathParams("key", "")
  resp key
  # let key = generateKey(namespace, key)
  # let value = session.get(key, incr=true)
  # if value.isNil:
  #     raise (ref exceptions.KeyNotFound)(msg: "The key $key or namespace $namespace could not be found. Please check your input and try again.")
  # return value
  #
  # let value = session.get(key)
  # if value.len == 0:
  #   resp notFoundResponse(fmt"Key {key} not found")
  # else:
  #   resp % value


# proc indexView*(ctx: Context) {.async.} =
#   if ctx.getQueryParams("save").len != 0:
#     let
#       edit = ctx.getQueryParams("task").strip
#       status = ctx.getQueryParams("status").strip
#       id = ctx.getPathParams("id", "")
#     var statusId = 0
#     if status == "open":
#         statusId = 1
#     resp htmlResponse(fmt"<p>The item number {id} was successfully updated</p><a href=/>Back to list</a>")
#   else:
#     let db= open("todo.db", "", "", "")
#     let id = ctx.getPathParams("id", "")
#     let data = db.getAllRows(sql"SELECT task FROM todo WHERE id LIKE ?", id)
#     resp htmlResponse(editList(id.parseInt, data[0]))
