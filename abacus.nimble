# Package

version       = "0.1.0"
author        = "Jason Cameron <dev@jasoncameron.dev>"
description   = "A simple counting API"
license       = "LGPL-3.0-or-later"
srcDir        = "src"
bin           = @["abacus"]


# Dependencies

requires "nim >= 2.0.2"
requires "jester ~= 0.6.0"
requires "redis >= 0.4.0"

