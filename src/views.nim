import prologue

const docs_url: string = "https://github.com/JasonLovesDoggo/abacus/blob/master/docs/ROUTES.md"


proc docsView*(ctx: Context) {.async.} =
  resp redirect(docs_url)

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
