import prologue
import prologue/middlewares
import ./urls
from ./views import docsView

let
  env = loadPrologueEnv(".env")
  settings = newSettings(appName = "abacus",
                reusePort = env.getOrDefault("reusePort", false),
                debug = env.getOrDefault("debug", true),
                port = Port(env.getOrDefault("port", 8080)),
                secretKey = env.getOrDefault("secretKey", "PleaseChangeMe")
    )

if env.getOrDefault("secretKey", "PleaseChangeMe") == "PleaseChangeMe":
  raise newException(ValueError, "Please change the secret key from the default value in .env")

var app = newApp(settings = settings)

app.use(debugRequestMiddleware())
app.use(CorsMiddleware(allowMethods = @["GET", "POST", "DELETE"]))
app.addRoute(urls.urlPatterns, "")
app.registerErrorHandler(Http404, docsView)
app.run()
