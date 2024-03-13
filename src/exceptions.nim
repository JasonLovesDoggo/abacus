type KeyNotFound = object of CatchableError
type KeyAlreadyExists = object of CatchableError
type KeyNotUnique = object of CatchableError

export KeyNotFound, KeyAlreadyExists, KeyNotUnique
