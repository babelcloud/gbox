import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { boxTemplate, handleBoxResource } from "./resources.js";
import {
  RUN_PYTHON_TOOL,
  RUN_PYTHON_DESCRIPTION,
  RUN_BASH_TOOL,
  RUN_BASH_DESCRIPTION,
  READ_FILE_TOOL,
  READ_FILE_DESCRIPTION,
  WRITE_FILE_TOOL,
  WRITE_FILE_DESCRIPTION,
  LIST_BOXES_TOOL,
  LIST_BOXES_DESCRIPTION,
  handleRunPython,
  handleRunBash,
  handleReadFile,
  runPythonParams,
  runBashParams,
  readFileParams,
  handleWriteFile,
  writeFileParams,
  handleListBoxes,
  VIEW_URL_AS_TOOL,
  VIEW_URL_AS_DESCRIPTION,
  viewUrlAsParams,
  handleViewUrlAs,
} from "./tools/index.js";
import type { RequestHandlerExtra } from "@modelcontextprotocol/sdk/shared/protocol.js";
import { MCPLogger } from "./mcp-logger.js";
import type { LogFn } from "./types.js";
import type { LoggingMessageNotification } from "@modelcontextprotocol/sdk/types.js";

const isSse = process.env.MODE?.toLowerCase() === "sse";

// Create MCP server instance
const mcpServer = new McpServer(
  {
    name: "gbox-mcp-server",
    version: "1.0.0",
  },
  {
    capabilities: {
      prompts: {},
      resources: {},
      tools: {},
      ...(!isSse ? { logging: {} } : {}),
    },
  }
);

const log: LogFn = async (
  params: LoggingMessageNotification["params"]
): Promise<void> => {
  if (isSse) {
    if (params.level === "debug") {
      console.debug(params.data);
    } else if (params.level === "info") {
      console.info(params.data);
    } else if (params.level === "warning") {
      console.warn(params.data);
    } else if (params.level === "error") {
      console.error(params.data);
    } else if (params.level === "notice") {
      console.log(params.data);
    } else if (params.level === "critical") {
      console.error(params.data);
    } else if (params.level === "alert") {
      console.warn(params.data);
    } else if (params.level === "emergency") {
      console.warn(params.data);
    } else if (params.level === "trace") {
      console.trace(params.data);
    } else {
      console.log(params.data);
    }
  } else {
    await mcpServer.server.sendLoggingMessage(params);
  }
};

// Create an instance of MCPLogger
const logger = new MCPLogger(log);

// Register box resource, passing the logger instance
mcpServer.resource("box", boxTemplate(logger), handleBoxResource(logger));

const GBOX_MANUAL = "gbox-manual";
const GBOX_MANUAL_DESCRIPTION = "A manual for the gbox command line tool.";
const GBOX_MANUAL_CONTENT = `
# GBox Manual

## Overview
Gbox is a set of tools that allows you to complete various tasks. All the tools are executed in a sandboxed environment. Gbox is developed by Gru AI.

## Usage
### run-python
If you need to execute a standalone python script, you can use the run-python tool. 

### run-bash
If you need to execute a standalone bash script, you can use the run-bash tool. 

### read-file
If you need to read a file from the sandbox, you can use the read-file tool. 

### write-file
If you need to write a file to the sandbox, you can use the write-file tool. 
`;

// Register gbox manual prompt
mcpServer.prompt(
  GBOX_MANUAL,
  GBOX_MANUAL_DESCRIPTION,
  (_: RequestHandlerExtra) => {
    return {
      messages: [
        {
          role: "user",
          content: {
            type: "text",
            text: GBOX_MANUAL_CONTENT,
          },
        },
      ],
    };
  }
);

// Register tools

mcpServer.tool(
  LIST_BOXES_TOOL,
  LIST_BOXES_DESCRIPTION,
  {},
  handleListBoxes(logger)
);

mcpServer.tool(
  READ_FILE_TOOL,
  READ_FILE_DESCRIPTION,
  readFileParams,
  handleReadFile(logger)
);

mcpServer.tool(
  WRITE_FILE_TOOL,
  WRITE_FILE_DESCRIPTION,
  writeFileParams,
  handleWriteFile(logger)
);

mcpServer.tool(
  RUN_PYTHON_TOOL,
  RUN_PYTHON_DESCRIPTION,
  runPythonParams,
  handleRunPython(logger)
);

mcpServer.tool(
  RUN_BASH_TOOL,
  RUN_BASH_DESCRIPTION,
  runBashParams,
  handleRunBash(logger)
);

mcpServer.tool(
  VIEW_URL_AS_TOOL,
  VIEW_URL_AS_DESCRIPTION,
  viewUrlAsParams,
  handleViewUrlAs(logger)
);

export { mcpServer, logger };
