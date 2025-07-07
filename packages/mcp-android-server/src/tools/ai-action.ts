import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionAI } from "gbox-sdk";
import { sanitizeResult } from "./utils.js";


export const AI_ACTION_TOOL = "ai_action";
export const AI_ACTION_DESCRIPTION = "Perform an action on the UI of the android box (natural language instruction).";

export const aiActionParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  instruction: z
    .string()
    .describe(
      "Direct instruction of the UI action to perform, e.g. 'click the login button'"
    ),
  background: z
    .string()
    .optional()
    .describe(
      "Contextual background for the action, to help the AI understand previous steps"
    ),
  includeScreenshot: z
    .boolean()
    .optional()
    .describe(
      "Whether to include screenshots in the action response (default false)"
    ),
  outputFormat: z
    .enum(["base64", "storageKey"])
    .optional()
    .describe("Output format for screenshot URIs (default 'base64')"),
  screenshotDelay: z
    .string()
    .regex(/^[0-9]+(ms|s|m|h)$/)
    .optional()
    .describe(
      "Delay after performing the action before the final screenshot, e.g. '500ms'"
    ),
};

// Define parameter types - infer from the Zod schema
type AiActionParams = z.infer<z.ZodObject<typeof aiActionParamsSchema>>;

export function handleAiAction(logger: MCPLogger) {
  return async (args: AiActionParams) => {
    try {
      const { boxId, instruction, background, includeScreenshot, outputFormat, screenshotDelay } = args;
      await logger.info("Performing AI action", { boxId, instruction });
      
      const box = await attachBox(boxId);

      // Map to SDK ActionAI type
      const actionParams: ActionAI = {
        instruction,
        ...(background && { background }),
        includeScreenshot: includeScreenshot ?? false,
        outputFormat: outputFormat ?? "base64",
        ...(screenshotDelay && { screenshotDelay: screenshotDelay as ActionAI['screenshotDelay'] })
      };

      const result = await box.action.ai(actionParams) as any;

      // Prepare image contents for before and after screenshots
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

      if (result?.screenshot?.before?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.before.uri);
        images.push({ type: "image", data: base64Data, mimeType });
      }

      if (result?.screenshot?.after?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.after.uri);
        images.push({ type: "image", data: base64Data, mimeType });
      }

      await logger.info("AI action completed", { boxId, imageCount: images.length });

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
      await logger.error("Failed to perform AI action", { boxId: args?.boxId, error });
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