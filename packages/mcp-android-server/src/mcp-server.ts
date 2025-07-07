import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { config } from "./config.js";
import { MCPLogger } from "./mcp-logger.js";
import {
  CREATE_ANDROID_BOX_TOOL,
  CREATE_ANDROID_BOX_DESCRIPTION,
  createAndroidBoxParamsSchema,
  handleCreateAndroidBox,
  LIST_BOXES_TOOL,
  LIST_BOXES_DESCRIPTION,
  listBoxesParamsSchema,
  handleListBoxes,
  GET_BOX_TOOL,
  GET_BOX_DESCRIPTION,
  getBoxParamsSchema,
  handleGetBox,
  GET_SCREENSHOT_TOOL,
  GET_SCREENSHOT_DESCRIPTION,
  getScreenshotParamsSchema,
  handleGetScreenshot,
  AI_ACTION_TOOL,
  AI_ACTION_DESCRIPTION,
  aiActionParamsSchema,
  handleAiAction,
  INSTALL_APK_TOOL,
  INSTALL_APK_DESCRIPTION,
  installApkParamsSchema,
  handleInstallApk,
  UNINSTALL_APK_TOOL,
  UNINSTALL_APK_DESCRIPTION,
  uninstallApkParamsSchema,
  handleUninstallApk,
  OPEN_APP_TOOL,
  OPEN_APP_DESCRIPTION,
  openAppParamsSchema,
  handleOpenApp,
  OPEN_LIVE_VIEW_TOOL,
  OPEN_LIVE_VIEW_DESCRIPTION,
  openLiveViewParamsSchema,
  handleOpenLiveView,
  CLOSE_APP_TOOL,
  CLOSE_APP_DESCRIPTION,
  closeAppParamsSchema,
  handleCloseApp,
  PRESS_KEY_TOOL,
  PRESS_KEY_DESCRIPTION,
  pressKeyParamsSchema,
  handlePressKey,
} from "./tools/index.js";
import type { LogFn } from "./types.js";
import type { LoggingMessageNotification } from "@modelcontextprotocol/sdk/types.js";
import type { RequestHandlerExtra } from "@modelcontextprotocol/sdk/shared/protocol.js";

const isSse = config.mode === "sse";

// Create MCP server instance
const mcpServer = new McpServer(
  {
    name: "gbox-android",
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

// Create logger instance
const logger = new MCPLogger(log);

// Add prompt for APK testing rules
const ANDROID_APK_TESTING_RULES = "android-apk-testing-rules";
const ANDROID_APK_TESTING_RULES_DESCRIPTION = "Test the Android project on a virtual or physical device.";
const ANDROID_APK_TESTING_RULES_CONTENT = `# Gbox APK-Testing Rule

## Critical Rules
- Immediately after creating or starting a Gbox Android box, open its live-view URL (\`open_live_view\`) in the default browser.
- Compute the absolute path of \`./app/build/outputs/apk/debug/app-debug.apk\` (e.g. \`/Users/jack/workspace/geoquiz/app/build/outputs/apk/debug/app-debug.apk\`) and pass that to \`install_apk\`.
- Wait for successful installation before launching or opening the app.
- If multiple boxes are running, operate only on the box created for the current test session.

## Examples
<example>
✅ Correct Flow
1. Create Android box → obtain \`boxId\`.
2. Open live-view for \`boxId\`.
3. Install APK with absolute path \`/abs/path/to/repo/geoquiz/app/build/outputs/apk/debug/app-debug.apk\`.
4. Launch the app.
5. Use the ai_action tool to perform UI actions.
6. Keep reviewing the screenshots after the operation to determine if it is as expected.
7. Keep action-review loop until all test done.
</example>

<example type="invalid">
❌ Forgetting to open the live-view URL.
❌ Passing the relative APK path \`./app/build/outputs/apk/debug/app-debug.apk\` to \`install_apk\`.
</example>`;

mcpServer.prompt(
  ANDROID_APK_TESTING_RULES,
  ANDROID_APK_TESTING_RULES_DESCRIPTION,
  () => {
    return {
      messages: [
        {
          role: "user",
          content: {
            type: "text",
            text: ANDROID_APK_TESTING_RULES_CONTENT,
          },
        },
      ],
    };
  }
);

// Register tools with Zod schemas
mcpServer.tool(
  CREATE_ANDROID_BOX_TOOL,
  CREATE_ANDROID_BOX_DESCRIPTION,
  createAndroidBoxParamsSchema,
  handleCreateAndroidBox(logger)
);

mcpServer.tool(
  LIST_BOXES_TOOL,
  LIST_BOXES_DESCRIPTION,
  listBoxesParamsSchema,
  handleListBoxes(logger)
);

mcpServer.tool(
  GET_BOX_TOOL,
  GET_BOX_DESCRIPTION,
  getBoxParamsSchema,
  handleGetBox(logger)
);

mcpServer.tool(
  GET_SCREENSHOT_TOOL,
  GET_SCREENSHOT_DESCRIPTION,
  getScreenshotParamsSchema,
  handleGetScreenshot(logger)
);

mcpServer.tool(
  AI_ACTION_TOOL,
  AI_ACTION_DESCRIPTION,
  aiActionParamsSchema,
  handleAiAction(logger)
);

mcpServer.tool(
  INSTALL_APK_TOOL,
  INSTALL_APK_DESCRIPTION,
  installApkParamsSchema,
  handleInstallApk(logger)
);

mcpServer.tool(
  UNINSTALL_APK_TOOL,
  UNINSTALL_APK_DESCRIPTION,
  uninstallApkParamsSchema,
  handleUninstallApk(logger)
);

mcpServer.tool(
  OPEN_APP_TOOL,
  OPEN_APP_DESCRIPTION,
  openAppParamsSchema,
  handleOpenApp(logger)
);

mcpServer.tool(
  CLOSE_APP_TOOL,
  CLOSE_APP_DESCRIPTION,
  closeAppParamsSchema,
  handleCloseApp(logger)
);

mcpServer.tool(
  OPEN_LIVE_VIEW_TOOL,
  OPEN_LIVE_VIEW_DESCRIPTION,
  openLiveViewParamsSchema,
  handleOpenLiveView(logger)
);

mcpServer.tool(
  PRESS_KEY_TOOL,
  PRESS_KEY_DESCRIPTION,
  pressKeyParamsSchema,
  handlePressKey(logger)
);

export { mcpServer, logger };