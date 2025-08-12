import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import { MCPLogger } from "../mcp-logger.js";
import type { ActionScreenshot } from "gbox-sdk";

export const WAIT_TOOL = "wait";
export const WAIT_TOOL_DESCRIPTION =
  "Waits for a specified duration before next action. Useful when you need to wait for something to load or for an action to complete.";

export const waitParamsSchema = z.object({
  boxId: z.string().describe("ID of the box"),
  duration: z
    .number()
    .int()
    .positive()
    .describe("The duration to wait in milliseconds."),
});

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export function handleWait(logger: MCPLogger) {
  return async (params: z.infer<typeof waitParamsSchema>) => {
    const { boxId, duration } = params;

    // Wait the specified duration
    await sleep(duration);

    // Capture screenshot after waiting
    try {
      const box = await attachBox(boxId);
      const screenshotParams: ActionScreenshot = { outputFormat: "base64" };
      const screenshotResult = await box.action.screenshot(screenshotParams);

      // Extract base64 data and mime type
      let mimeType = "image/png";
      let base64Data = screenshotResult.uri;
      if (screenshotResult.uri.startsWith("data:")) {
        const match = screenshotResult.uri.match(/^data:(.+);base64,(.*)$/);
        if (match) {
          mimeType = match[1];
          base64Data = match[2];
        }
      }

      const message = `Finished waiting for ${duration}ms.`;
      await logger.info(message);

      return {
        content: [
          {
            type: "text" as const,
            text: message,
          },
          {
            type: "image" as const,
            data: base64Data,
            mimeType,
          },
        ],
      };
    } catch (error) {
      // If screenshot fails, still return wait text with error information
      const message = `Finished waiting for ${duration}ms, but failed to capture screenshot.`;
      await logger.error("Failed to capture screenshot after wait", { boxId, error });
      return {
        content: [
          {
            type: "text" as const,
            text: message,
          },
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