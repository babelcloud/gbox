import { z } from "zod";
import { attachBox } from "../sdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionPressButton } from "gbox-sdk";
import { extractImageInfo } from "../sdk/utils.js";

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
    ),
};

// Define parameter types - infer from the Zod schema
type PressButtonParams = z.infer<z.ZodObject<typeof pressButtonParamsSchema>>;

export function handlePressButton(logger: MCPLogger) {
  return async ({ boxId, buttons }: PressButtonParams) => {
    try {
      await logger.info("Pressing buttons", {
        boxId,
        buttons: buttons.join(" + "),
      });

      const box = await attachBox(boxId);
      const result = await box.action.pressButton({
        buttons: buttons,
        options: {
          screenshot: {
            outputFormat: "base64",
            delay: "500ms",
          },
        },
      });

      return {
        content: [
          {
            type: "text" as const,
            text: "Button pressed successfully",
          },
          {
            type: "image" as const,
            ...extractImageInfo(result.screenshot.after.uri),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to press buttons", {
        boxId,
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
