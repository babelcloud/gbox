import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../gboxsdk/index.js";
import {
  buildActionReturnValues,
  getBoxCoordinates,
} from "../gboxsdk/utils.js";

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
      "Distance of the swipe. Supported values are 'tiny', 'short', 'medium', and 'long'. Defaults to 'medium'.",
    ),
  location: z
    .string()
    .optional()
    .describe(
      "Optional description of where on the screen to start the swipe (e.g. 'bottom half', 'toolbar area'). Defaults to the centre of the screen.",
    ),
};

type SwipeParams = z.infer<z.ZodObject<typeof swipeParamsSchema>>;

export function handleSwipe(logger: MCPLogger) {
  return async (args: SwipeParams) => {
    try {
      const { boxId, direction, distance, location } = args;
      await logger.info("Swipe command invoked", {
        boxId,
        direction,
        distance,
        location,
      });

      const box = await attachBox(boxId);

      // calculate distance in pixels
      const { width, height } = (await box.display()).resolution;
      let distanceInPixels = 0;
      switch (direction) {
        case "up":
          distanceInPixels =
            distance === "tiny"
              ? 40
              : distance === "short"
                ? 150
                : distance === "long"
                  ? Math.round(height / 2)
                  : Math.round(height / 4);
          break;
        case "down":
          distanceInPixels =
            distance === "tiny"
              ? 40
              : distance === "short"
                ? 150
                : distance === "long"
                  ? Math.round(height / 2)
                  : Math.round(height / 4);
          break;
        case "left":
          distanceInPixels =
            distance === "tiny"
              ? 40
              : distance === "short"
                ? 150
                : distance === "long"
                  ? Math.round(width / 2)
                  : Math.round(width / 4);
          break;
        case "right":
          distanceInPixels =
            distance === "tiny"
              ? 40
              : distance === "short"
                ? 150
                : distance === "long"
                  ? Math.round(width / 2)
                  : Math.round(width / 4);
          break;
      }
      let actionResult;
      if (!location) {
        actionResult = await box.action.swipe({
          direction,
          distance: distanceInPixels,
          duration: "200ms",
          includeScreenshot: true,
          outputFormat: "base64",
          screenshotDelay: "500ms",
        });
      } else {
        const instruction = `Click at ${location}`;
        const boxCoordinates = await getBoxCoordinates(box, instruction);
        if (boxCoordinates.length === 0) {
          return {
            content: [{ type: "text" as const, text: "No coordinates found" }],
          };
        }
        const { x: startX, y: startY } = boxCoordinates[0];
        // Compute end point based on direction
        let endX = startX;
        let endY = startY;
        switch (direction) {
          case "up":
            endY = startY - distanceInPixels;
            break;
          case "down":
            endY = startY + distanceInPixels;
            break;
          case "left":
            endX = startX - distanceInPixels;
            break;
          case "right":
            endX = startX + distanceInPixels;
            break;
        }
        const swipeAction = {
          start: { x: startX, y: startY },
          end: { x: endX, y: endY },
          duration: "200ms",
          includeScreenshot: true,
          outputFormat: "base64" as const,
          screenshotDelay: "500ms" as const,
        };
        actionResult = await box.action.swipe(swipeAction);
      }

      return buildActionReturnValues(actionResult, box);
    } catch (error) {
      await logger.error("Failed to run swipe action", {
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
