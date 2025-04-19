import express from "express";
import { SSEServerTransport } from "@modelcontextprotocol/sdk/server/sse.js";
import { mcpServer, logger } from "./mcp-server";

export function startSseServer() {
  let transport: SSEServerTransport | null = null;

  const app = express();

  app.get("/sse", (req, res) => {
    logger.info("SSE client connected");
    transport = new SSEServerTransport("/messages", res);
    mcpServer.connect(transport);
    req.on("close", () => {
      logger.info("SSE client disconnected");
      // Optionally disconnect transport or handle cleanup if needed
      // mcpServer.disconnect(transport); // Assuming such a method exists
    });
  });

  app.post("/messages", express.json(), (req, res) => {
    if (transport) {
      transport.handlePostMessage(req, res);
    } else {
      res.status(400).send("SSE transport not initialized");
    }
  });

  const port = process.env.PORT || 28090;

  app.listen(port, () => {
    logger.info(`SSE Server started successfully on port ${port}`);
  });
}
