# example.nim
import htmlgen
import jester

routes:
  get "/":
    resp h1("Hello world")
