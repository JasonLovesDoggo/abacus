import json
import prologue
import prologue/middlewares/utils
import prologue/middlewares/staticfile
from prologue/openapi import serveDocs
import ./urls
from ./views import docsView

let
  env = loadPrologueEnv(".env")
  settings = newSettings(appName = "abacus",
                debug = env.getOrDefault("debug", true),
                port = Port(env.getOrDefault("port", 8080)),
                secretKey = env.getOrDefault("secretKey", "")
    )

var app = newApp(settings = settings)

app.use(debugRequestMiddleware())
app.serveDocs("docs/openapi.json")
app.addRoute(urls.urlPatterns, "")
app.registerErrorHandler(Http404, docsView)
app.run()
