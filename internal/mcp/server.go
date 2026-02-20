package mcp

import (
	"errors"

	"github.com/mark3labs/mcp-go/server"
)

var errConfigRequired = errors.New("rimba is not initialized in this repository; run `rimba init` first")

// NewServer creates an MCP server with all rimba tools registered.
func NewServer(hctx *HandlerContext) *server.MCPServer {
	s := server.NewMCPServer("rimba", hctx.Version,
		server.WithToolCapabilities(false),
	)

	registerListTool(s, hctx)
	registerAddTool(s, hctx)
	registerRemoveTool(s, hctx)
	registerStatusTool(s, hctx)
	registerExecTool(s, hctx)
	registerConflictCheckTool(s, hctx)
	registerMergeTool(s, hctx)
	registerSyncTool(s, hctx)
	registerCleanTool(s, hctx)

	return s
}
