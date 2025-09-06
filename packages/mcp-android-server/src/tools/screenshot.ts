import { z } from "zod";
import { attachBox } from "../sdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionScreenshot } from "gbox-sdk";
import { extractImageInfo } from "../sdk/utils.js";

export const SCREENSHOT_TOOL = "screenshot";
export const SCREENSHOT_DESCRIPTION =
  "Take a screenshot of the current display for a given box. The output format can be either base64 or an Presigned URL";

export const screenshotParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  // outputFormat: z
  //   .enum(["base64", "storageKey"])
  //   .optional()
  //   .default("storageKey")
  //   .describe("The output format for the screenshot."),
};

// Define parameter types - infer from the Zod schema
type ScreenshotParams = z.infer<z.ZodObject<typeof screenshotParamsSchema>>;

export function handleScreenshot(logger: MCPLogger) {
  return async ({ boxId }: ScreenshotParams) => {
    try {
      await logger.info("Taking screenshot", { boxId });

      const box = await attachBox(boxId);

      // Map to SDK ActionScreenshot type
      const actionParams: ActionScreenshot = {
        outputFormat: "base64",
      };

      const result = await box.action.screenshot(actionParams);

      await logger.info("Screenshot taken successfully", { boxId });

      // Return image content for MCP
      return {
        content: [
          {
            type: "image" as const,
            ...extractImageInfo(result.uri),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to take screenshot", {
        boxId,
        error,
      });
      return {
        content: [
          {
            type: "text" as const,
            text: `Error: ${
              error instanceof Error ? error.message : String(error)
            }`,
          },
        ],
        isError: true,
      };
    }
  };
}
