package errors

import "errors"

var (
	ErrorLimit = errors.New("incorrect value for limit. Default 100; max 5000. Valid limits:[5, 10, 20, 50, 100, 500, 1000, 5000]")
	ErrorSymbol = errors.New("symbol cannot be empty")
)
