package mcp

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
)

// errorResult builds an MCP error result from err. When err carries an
// errhint "To fix:" hint, it is already part of err.Error() and surfaces inline.
func errorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

// invalidTypeResult builds the "invalid type" error, listing valid types.
// extraHint is appended verbatim so call sites can add trailing guidance.
func invalidTypeResult(typeName string, ps *resolver.PrefixSet, extraHint string) *mcp.CallToolResult {
	return errorResult(errhint.WithFix(
		fmt.Errorf("invalid type %q", typeName),
		"use one of: "+strings.Join(ps.TypeNames(), ", ")+extraHint,
	))
}
