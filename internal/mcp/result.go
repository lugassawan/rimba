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

// invalidTypeResult builds the standard "invalid type" error result, listing
// the valid prefix types. extraHint is appended verbatim to the fix hint,
// letting call sites add site-specific trailing guidance (e.g. a default-type
// note) while sharing the same underlying message construction.
func invalidTypeResult(typeName string, ps *resolver.PrefixSet, extraHint string) *mcp.CallToolResult {
	return errorResult(errhint.WithFix(
		fmt.Errorf("invalid type %q", typeName),
		"use one of: "+strings.Join(ps.TypeNames(), ", ")+extraHint,
	))
}
