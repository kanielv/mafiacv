package tools

import "errors"

var (
	ErrInvalidArgs = errors.New("tools: invalid arguments")
	ErrUnknownTool = errors.New("tools: unknown tool")
)
