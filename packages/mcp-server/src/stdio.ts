import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { mcpServer, logger } from "./mcp-server";

// Export a function to start the STDIO server
export async function startStdioServer() {
  // Start server
  const transport = new StdioServerTransport();
  await mcpServer.connect(transport);
}
