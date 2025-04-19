import { startStdioServer } from "./stdio.js";
import { startSseServer } from "./sse.js";

const mode = process.env.MODE?.toLowerCase();

if (mode === "sse") {
  console.log("Starting MCP Server in SSE mode...");
  startSseServer();
} else {
  startStdioServer().catch((error) => {
    console.error("Failed to start STDIO server:", error);
    process.exit(1);
  });
}
