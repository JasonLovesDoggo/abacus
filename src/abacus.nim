# example.nim
import htmlgen
import jester
import markdown

const docs = markdown(readFile("docs/ROUTES.md"))

routes:
  get "":
    resp docs, "text/html"



#
# error Http404:
#     resp "not found"
