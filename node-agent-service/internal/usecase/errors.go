package usecase

import "errors"

// ErrNoFreePort is returned by CreateDatabase when every port in the
// agent's configured range is already in use.
var ErrNoFreePort = errors.New("no free port available")
