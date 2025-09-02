import { z } from "zod";
import { attachBox } from "../sdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionScreenshot } from "gbox-sdk";
import { extractImageInfo, maybeResizeAndCompressImage } from "../sdk/utils.js";

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
  return async (args: ScreenshotParams) => {
    try {
      const { boxId } = args;
      await logger.info("Taking screenshot", { boxId });

      const box = await attachBox(boxId);

      // Map to SDK ActionScreenshot type
      const actionParams: ActionScreenshot = {
        outputFormat: "base64",
      };

      const result = await box.action.screenshot(actionParams);

      await logger.info("Screenshot taken successfully", { boxId });
      const imageInfo = extractImageInfo(result.uri);
      const processedData = await maybeResizeAndCompressImage(
        imageInfo,
        (await box.display()).resolution
      );

      // Return image content for MCP
      return {
        content: [
          {
            type: "image" as const,
            data: processedData.base64Data,
            mimeType: processedData.mimeType,
          },
        ],
      };
      // Handle different output formats
      // if (outputFormat === "storageKey") {
      //   // For storageKey format, get the presigned URL for the storage key using SDK
      //   const presignedUrl = await box.storage.createPresignedUrl({ storageKey: result.uri });

      //   await logger.info("Presigned URL created", {
      //     boxId,
      //     storageKey: result.uri,
      //     presignedUrl
      //   });

      //   return {
      //     content: [
      //       {
      //         type: "text" as const,
      //         text: `Screenshot URL: ${presignedUrl}`,
      //       },
      //     ],
      //   };
      // } else {
      //   // For base64 format, parse the data URI
      //   let mimeType = "image/png";
      //   let base64Data = result.uri;

      //   if (result.uri.startsWith("data:")) {
      //     const match = result.uri.match(/^data:(.+);base64,(.*)$/);
      //     if (match) {
      //       mimeType = match[1];
      //       base64Data = match[2];
      //     }
      //   }

      //   // Return image content for MCP
      //   return {
      //     content: [
      //       {
      //         type: "image" as const,
      //         data: base64Data,
      //         mimeType,
      //       },
      //     ],
      //   };
      // }
    } catch (error) {
      await logger.error("Failed to take screenshot", {
        boxId: args?.boxId,
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
