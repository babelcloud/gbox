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
  UI_ACTION_TOOL,
  UI_ACTION_DESCRIPTION,
  uiActionParamsSchema,
  handleUiAction,
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
  handleTypeText,
  TYPE_TEXT_TOOL,
  TYPE_TEXT_DESCRIPTION,
  typeTextParamsSchema,
} from "./tools/index.js";
import type { LogFn } from "./types.js";
import type { LoggingMessageNotification } from "@modelcontextprotocol/sdk/types.js";
import type { RequestHandlerExtra } from "@modelcontextprotocol/sdk/shared/protocol.js";
import z from "zod";

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
const ANDROID_APK_TESTING_GUIDE = "android-apk-testing-guide";
const ANDROID_APK_TESTING_GUIDE_DESCRIPTION =
  "Test the Android project on gbox (virtual or physical device).";
const ANDROID_APK_TESTING_GUIDE_CONTENT = `## 🔒 Critical Rules

- **Always** open the Android box's (id:{boxId}) **Live View URL** (via MCP Tool \`open_live_view\`) in your default browser **immediately after** creating or starting a Gbox Android box.
- Call the \`install_apk\` tool to install the APK: {apkPath}.
- **Wait for the APK to finish installing** before interacting with the app.  
  You can pass the parameter \`open=true\` to automatically launch the app after installation.
- If multiple boxes are running, ensure you're **only operating on the correct box** for your current test session.
- Use MCP Tool adb_shell to execute adb shell in log watching or other infomation obtaining.
- Use MCP Tool logcat to get the log of the app.
- Do not try to exec adb command in Terminal, because the Android box is running on cloud, there is no adb connection locally.
---

## 🛠️ Using the \`ui_action\` Tool for UI Testing

Use the \`ui_action\` tool to control the Android UI with natural language commands.  
Here are some example commands you can use:

- Tap the email input field  
- Tap the submit button  
- Tap the plus button in the upper right corner  
- Fill the search field with text: \`gbox ai\`  
- Press the back button  
- Double-click the video  

---

## ✅ Example: Proper Testing Flow

1. **Create** a new Android box and obtain its \`boxId\`.
2. **Open** the Live View for that \`boxId\` in your browser.
3. **Install** the APK using its **absolute path**, e.g.:  /abs/path/to/repo/geoquiz/app/build/outputs/apk/debug/app-debug.apk, Add the parameter \`open=true\` if you'd like the app to launch automatically.
4. **Use \`ui_action\`** to simulate user interactions based on your test case.
5. After each action, **review the Live View screenshot** to confirm the result.
6. Continue the **action-review loop** until your test scenario is complete.

---

## ❌ Common Mistakes to Avoid

- 🚫 Not opening the **Live View URL** right after creating the box.
- 🚫 Using a **relative path** (e.g., \`./app/build/outputs/apk/debug/app-debug.apk\`) for \`install_apk\`.
- 🚫 Sending **multiple UI actions in one command** or using **unclear/vague language** with \`ui_action\`.
`;

mcpServer.prompt(
  ANDROID_APK_TESTING_GUIDE,
  ANDROID_APK_TESTING_GUIDE_DESCRIPTION,
  () => {
    return {
      argsSchema: {
        apkPath: z.string().describe("The absolute path to the APK file."),
        boxId: z.string().describe("The ID of the Android box."),
      },
      messages: [
        {
          role: "user",
          content: {
            type: "text",
            text: ANDROID_APK_TESTING_GUIDE_CONTENT,
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
  UI_ACTION_TOOL,
  UI_ACTION_DESCRIPTION,
  uiActionParamsSchema,
  handleUiAction(logger)
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

mcpServer.tool(
  TYPE_TEXT_TOOL,
  TYPE_TEXT_DESCRIPTION,
  typeTextParamsSchema,
  handleTypeText(logger)
);

export { mcpServer, logger };
