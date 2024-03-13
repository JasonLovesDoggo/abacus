import htmlgen
import jester
import utils
import exceptions
import json
from utils as u import nil
const docs_url: string = "https://github.com/JasonLovesDoggo/abacus/blob/master/docs/ROUTES.md"

routes:
  get "hit//@namespace/@key":
    try:
      let value = u.hit(@"namespace", @"key")
      resp %*{"count": value}
    except error KeyNotFound:
      resp %*{"message": error.message}

  error Http404:
      redirect docs_url
