import prologue

import ./views

const urlPatterns* = @[
  # pattern("/hit/{namespace}/{key}", hitView),
  # pattern("/retrieve/{namespace}/{key}", retrieveView),
  # pattern("/create/{namespace}/{key}", createView),
  # pattern("/delete/{namespace}/{key}", deleteView),
  # pattern("/reset/{namespace}/{key}", resetView),
  # pattern("/keys/{namespace}", keysView),
  pattern("/stats", statsView),
]


