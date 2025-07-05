import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionScreenshot } from "gbox-sdk";

export const GET_SCREENSHOT_TOOL = "get_screenshot";
export const GET_SCREENSHOT_DESCRIPTION = "Take a screenshot of the current display for a given box.";

export const getScreenshotParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  outputFormat: z
    .enum(["base64", "storageKey"])
    .optional()
    .default("base64")
    .describe("The output format for the screenshot."),
};

// Define parameter types - infer from the Zod schema
type GetScreenshotParams = z.infer<z.ZodObject<typeof getScreenshotParamsSchema>>;

export function handleGetScreenshot(logger: MCPLogger) {
  return async (args: GetScreenshotParams) => {
    try {
      const { boxId, outputFormat } = args;
      await logger.info("Taking screenshot", { boxId, outputFormat });
      
      const box = await attachBox(boxId);
      
      // Map to SDK ActionScreenshot type
      const actionParams: ActionScreenshot = {
        outputFormat: outputFormat ?? "base64"
      };
      
      const result = await box.action.screenshot(actionParams);

      // The SDK returns a `uri` string. It may be a bare base64 string or a data URI.
      let mimeType = "image/png";
      let base64Data = result.uri;

      if (result.uri.startsWith("data:")) {
        const match = result.uri.match(/^data:(.+);base64,(.*)$/);
        if (match) {
          mimeType = match[1];
          base64Data = match[2];
        }
      }

      await logger.info("Screenshot taken successfully", { boxId });

      // Return image content for MCP
      return {
        content: [
          {
            type: "image" as const,
            data: base64Data,
            mimeType,
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to take screenshot", { boxId: args?.boxId, error });
      return {
        content: [
          {
            type: "text" as const,
            text: `Error: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
        isError: true,
      };
    }
  };
}