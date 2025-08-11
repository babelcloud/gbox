import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionPressButton } from "gbox-sdk";
import { parseUri, sanitizeResult } from "./utils.js";
import { ActionPressButtonResponse } from "gbox-sdk/resources/v1/boxes/actions.js";

export const PRESS_BUTTON_TOOL = "press_button";

export const PRESS_BUTTON_DESCRIPTION =
  "Press device hardware buttons such as power or volume controls. Use this to simulate hardware button presses on the Android device.";

// Extract supported buttons type from SDK
type SupportedButton = ActionPressButton["buttons"][number];

// List of supported buttons derived from SDK docs
const SUPPORTED_BUTTONS = [
  "power",
  "volumeUp",
  "volumeDown",
  "volumeMute",
  "home",
  "back",
  "menu",
  "appSwitch",
] as const satisfies readonly SupportedButton[];

export const pressButtonParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  buttons: z
    .array(z.enum(SUPPORTED_BUTTONS))
    .min(1)
    .describe(
      "Array of hardware buttons to press. Can be a single button like ['power'] or multiple like ['power', 'volumeUp']"
    )   
};

// Define parameter types - infer from the Zod schema
type PressButtonParams = z.infer<z.ZodObject<typeof pressButtonParamsSchema>>;

export function handlePressButton(logger: MCPLogger) {
  return async (args: PressButtonParams) => {
    try {
      const { boxId, buttons } = args;
      await logger.info("Pressing buttons", { boxId, buttons: buttons.join(" + ") });

      const box = await attachBox(boxId);

      // Map to SDK ActionPressButton type
      const actionParams: ActionPressButton = {
        buttons: buttons as ActionPressButton["buttons"],
        includeScreenshot: true,
        outputFormat: "base64",
        screenshotDelay: "500ms",
      };

      const result = await box.action.pressButton(actionParams) as ActionPressButtonResponse.ActionIncludeScreenshotResult;

      // Prepare image contents for screenshots
      const images: Array<{ type: "image"; data: string; mimeType: string }> = [];

      // Add screenshots if available
      // if (result?.screenshot?.trace?.uri) {
      //   const { mimeType, base64Data } = parseUri(result.screenshot.trace.uri);
      //   images.push({ type: "image", data: base64Data, mimeType });
      // }

      // if (result?.screenshot?.before?.uri) {
      //   const { mimeType, base64Data } = parseUri(result.screenshot.before.uri);
      //   images.push({ type: "image", data: base64Data, mimeType });
      // }

      if (result?.screenshot?.after?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.after.uri);
        images.push({ type: "image", data: base64Data, mimeType });
      }

      await logger.info("Buttons pressed successfully", {
        boxId,
        buttons: buttons.join(" + "),
        imageCount: images.length,
      });

      // Build content array with text and images
      const content: Array<
        | { type: "text"; text: string }
        | { type: "image"; data: string; mimeType: string }
      > = [];

      // Add text result with sanitized data
      content.push({
        type: "text" as const,
        text: "Button pressed successfully"
      });

      // Add all images
      images.forEach((img) => {
        content.push({
          type: "image" as const,
          data: img.data,
          mimeType: img.mimeType,
        });
      });

      return { content };
    } catch (error) {
      await logger.error("Failed to press buttons", {
        boxId: args?.boxId,
        error,
      });
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