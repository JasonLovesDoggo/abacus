# example.nim
import htmlgen
import jester

const docs_url: string = "https://github.com/JasonLovesDoggo/abacus/blob/master/docs/ROUTES.md"

routes:
  get "":
    redirect docs_url



#
# error Http404:
#     resp "not found"
