package zebou

import "errors"

var SessionNotFound = errors.New("session not found")
var NoPeerFromContext = errors.New("no peer in attached in context")
