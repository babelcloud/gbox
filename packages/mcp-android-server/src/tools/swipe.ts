import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../sdk/index.js";
import { extractImageInfo } from "../sdk/utils.js";

export const SWIPE_TOOL = "swipe";

export const SWIPE_DESCRIPTION =
  "Perform a swipe gesture on the Android device. Useful for navigating carousels or moving between screens.";

export const swipeParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  direction: z
    .enum(["up", "down", "left", "right"])
    .describe("Direction of the swipe gesture."),
  distance: z
    .enum(["tiny", "short", "medium", "long"])
    .optional()
    .describe(
      "Distance of the swipe. Supported values are 'tiny', 'short', 'medium', and 'long'. Defaults to 'medium'."
    ),
  location: z
    .string()
    .optional()
    .describe(
      "Optional description of where on the screen to start the swipe (e.g. 'bottom half', 'toolbar area'). Defaults to the centre of the screen."
    ),
};

type SwipeParams = z.infer<z.ZodObject<typeof swipeParamsSchema>>;

export function handleSwipe(logger: MCPLogger) {
  return async ({ boxId, direction, distance, location }: SwipeParams) => {
    try {
      await logger.info("Swipe command invoked", {
        boxId,
        direction,
        distance,
        location,
      });

      const box = await attachBox(boxId);

      const actionResult = await box.action.swipe({
        location,
        direction,
        distance,
        duration: "200ms",
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
            text: "Swipe action completed successfully",
          },
          {
            type: "image" as const,
            ...extractImageInfo(actionResult.screenshot.after.uri),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to run swipe action", {
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
