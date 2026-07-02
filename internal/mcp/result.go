package mcp

import "github.com/mark3labs/mcp-go/mcp"

// errorResult builds an MCP error result from err. When err carries an
// errhint "To fix:" hint, it is already part of err.Error() and surfaces inline.
func errorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}
