import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionType } from "gbox-sdk";
import { sanitizeResult } from "./utils.js";


export const TYPE_TEXT_TOOL = "type_text";

export const TYPE_TEXT_DESCRIPTION = "Directly inputs text content without triggering physical key events (keydown, etc.), ideal for quickly filling large amounts of text when intermediate input events aren't needed. Before using this tool, you should make sure the input field is focused.";

export const typeTextParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  text: z
    .string()
    .describe("The text to type into the input field, for example: 'Hello, world!'"),
  outputFormat: z
    .enum(["base64", "storageKey"])
    .default("base64")
    .optional()
    .describe("Output format for screenshot URIs (default 'base64')"),
  includeScreenshot: z
    .boolean()
    .default(false)
    .optional()
    .describe("Whether to include screenshots in the action response. If false, the screenshot object will still be returned but with empty URIs. Default is false."),
  screenshotDelay: z
    .string()
    .regex(/^[0-9]+(ms|s|m|h)$/)
    .default("500ms")
    .optional()
    .describe("Delay after performing the action before taking the final screenshot. Supports time units: ms (milliseconds), s (seconds), m (minutes), h (hours). Example: '500ms', '30s', '5m', '1h'. Default: 500ms. Maximum allowed: 30s")
};

// Define parameter types - infer from the Zod schema
type TypeTextParams = z.infer<z.ZodObject<typeof typeTextParamsSchema>>;

export function handleTypeText(logger: MCPLogger) {
  return async (args: TypeTextParams) => {
    try {
      const { boxId, text, includeScreenshot, outputFormat, screenshotDelay } = args;
      await logger.info("Typing text", { boxId, textLength: text.length });

      const box = await attachBox(boxId);
      
      // Map to SDK ActionType type
      const actionParams: ActionType = {
        text,
        includeScreenshot: includeScreenshot ?? false,
        outputFormat: outputFormat ?? "base64",
        ...(screenshotDelay && { screenshotDelay: screenshotDelay as ActionType['screenshotDelay'] })
      };

      const result = await box.action.type(actionParams) as any;

      // Prepare image contents for screenshots
      const images: Array<{ type: "image"; data: string; mimeType: string }> = [];

      const parseUri = (uri: string) => {
        let mimeType = "image/png";
        let base64Data = uri;

        if (uri.startsWith("data:")) {
          const match = uri.match(/^data:(.+);base64,(.*)$/);
          if (match) {
            mimeType = match[1];
            base64Data = match[2];
          }
        }

        return { mimeType, base64Data };
      };

      // Add screenshots if available
      if (result?.screenshot?.trace?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.trace.uri);
        images.push({ type: "image", data: base64Data, mimeType });
      }

      if (result?.screenshot?.before?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.before.uri);
        images.push({ type: "image", data: base64Data, mimeType });
      }

      if (result?.screenshot?.after?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.after.uri);
        images.push({ type: "image", data: base64Data, mimeType });
      }

      await logger.info("Text typed successfully", { boxId, textLength: text.length, imageCount: images.length });

      // Build content array with text and images
      const content: Array<{ type: "text"; text: string } | { type: "image"; data: string; mimeType: string }> = [];

      // Add text result with sanitized data
      content.push({
        type: "text" as const,
        text: JSON.stringify(sanitizeResult(result), null, 2),
      });

      // Add all images
      images.forEach(img => {
        content.push({
          type: "image" as const,
          data: img.data,
          mimeType: img.mimeType,
        });
      });
      
      return { content };
    } catch (error) {
      await logger.error("Failed to type text", { boxId: args?.boxId, error });
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