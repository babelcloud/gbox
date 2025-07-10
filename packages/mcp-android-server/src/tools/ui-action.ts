import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionAI } from "gbox-sdk";
import { sanitizeResult } from "./utils.js";


export const UI_ACTION_TOOL = "ui_action";
export const UI_ACTION_DESCRIPTION = "Use natural language instructions to perform UI operations on the box. You can describe what you want to do in plain language (e.g., ‘click the login button’, ‘scroll down to find settings’, ‘input my email address’), and the AI will automatically convert your instruction into the appropriate UI action and execute it on the box. But make sure the instruction is one step every time.";

export const uiActionParamsSchema = {
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
  settings: z
    .object({
      systemPrompt: z
        .string()
        .default("You are a helpful assistant that can operate Android devices. You are given an instruction and a background of the task to perform. You can see the current screen in the image. Analyze what you see and determine the next action needed to complete the task. Take your time to analyze the screen and plan your actions carefully. Tips: - You should execute the action directly by the instruction. - If you see the ADB Keyboard on the bottom of the screen, that means the field you should type is already focused. You should type directly no need to focus on the field. - You don't need to take screenshot before or after the action as it will be taken automatically by the action executor.")
        .describe(
          "System prompt that defines the AI's behavior and capabilities when executing UI actions. This prompt instructs the AI on how to interpret the screen, understand user instructions, and determine the appropriate UI actions to take. A well-crafted system prompt can significantly improve the accuracy and reliability of AI-driven UI automation. If not provided, uses the default computer use instruction template that includes basic screen interaction guidelines."
        ),
    })
    .optional()
    .describe("Settings for the AI action"),
};

// Define parameter types - infer from the Zod schema
type UiActionParams = z.infer<z.ZodObject<typeof uiActionParamsSchema>>;

export function handleUiAction(logger: MCPLogger) {
  return async (args: UiActionParams) => {
    try {
      const { boxId, instruction, background, includeScreenshot, outputFormat, screenshotDelay, settings } = args;
      await logger.info("Performing AI action", { boxId, instruction });
      
      const box = await attachBox(boxId);

      // Map to SDK ActionAI type
      const actionParams: ActionAI = {
        instruction,
        ...(background && { background }),
        includeScreenshot: includeScreenshot ?? false,
        outputFormat: outputFormat ?? "base64",
        ...(screenshotDelay && { screenshotDelay: screenshotDelay as ActionAI['screenshotDelay'] }),
        ...(settings && { settings })
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