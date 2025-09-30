import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../sdk/index.js";
import { extractImageInfo } from "../sdk/utils.js";

export const LONG_PRESS_TOOL = "long_press";

export const LONG_PRESS_DESCRIPTION =
  "Perform a long press (press and hold) on a UI element on the Android device. Useful for triggering context menus, selecting text, or initiating drag operations.";

export const longPressParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  target: z
    .string()
    .describe(
      "Description of the element to long press (e.g. 'text input field', 'message bubble', 'app icon'). MUST be detailed enough to identify the element unambiguously."
    ),
  duration: z
    .string()
    .regex(
      /^\d+(ms|s|m|h)$/,
      "Invalid duration format. Must be a number followed by 'ms', 's', 'm', or 'h'."
    )
    .optional()
    .describe(
      "How long to hold the press (e.g. '1s', '500ms', '2s'). Defaults to '1s'. Supported units: ms, s, m, h."
    ),
};

type LongPressParams = z.infer<z.ZodObject<typeof longPressParamsSchema>>;

export function handleLongPress(logger: MCPLogger) {
  return async ({ boxId, target, duration }: LongPressParams) => {
    try {
      await logger.info("Long press command invoked", {
        boxId,
        target,
        duration,
      });

      const box = await attachBox(boxId);

      const result = await box.action.longPress({
        target,
        duration: duration || "1s",
        options: {
          screenshot: {
            phases: ["after"],
            outputFormat: "base64",
            delay: "500ms",
          },
        },
      });

      return {
        content: [
          {
            type: "text" as const,
            text: "Long press action completed successfully",
          },
          {
            type: "image" as const,
            ...extractImageInfo(result.screenshot.after.uri),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to run long press action", {
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