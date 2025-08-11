import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../gboxsdk/index.js";
import { handleUiAction } from "./ui-action.js";
import { parseUri } from "./utils.js";

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
  return async (args: SwipeParams) => {
    try {
      const { boxId, direction, distance, location } = args;
      await logger.info("Swipe command invoked", { boxId, direction, distance, location });

      const box = await attachBox(boxId);

      // calculate distance in pixels
    const { width, height } = (await box.display()).resolution;
    let distanceInPixels = 0;
    switch (direction) {
      case "up":
        distanceInPixels =
          distance === "tiny" ? 20 : distance === "short" ? 150 : distance === "long" ? Math.round(height / 2) : Math.round(height / 4);
        break;
      case "down":
        distanceInPixels =
          distance === "tiny" ? 20 : distance === "short" ? 150 : distance === "long" ? Math.round(height / 2) : Math.round(height / 4);
        break;
      case "left":
        distanceInPixels =
          distance === "tiny" ? 20 : distance === "short" ? 150 : distance === "long" ? Math.round(width / 2) : Math.round(width / 4);
        break;
      case "right":
        distanceInPixels =
          distance === "tiny" ? 20 : distance === "short" ? 150 : distance === "long" ? Math.round(width / 2) : Math.round(width / 4);
        break;
    }
      let actionResult;
      if (!location) {
        actionResult = await box.action.swipe({
          direction,
          distance: distanceInPixels,
          duration: "500ms",
          includeScreenshot: true,
          outputFormat: "base64",
          screenshotDelay: "500ms",
        });
      } else {
        const instruction = `Swipe ${direction},${distance}, at ${location}`;
        actionResult = await handleUiAction(logger)({
          boxId,
          instruction,
        });
      }

      // Build response content
      const contentItems: Array<
        | { type: "text"; text: string }
        | { type: "image"; data: string; mimeType: string }
      > = [];

      // Add sanitized JSON of the final result
      contentItems.push({
        type: "text",
        text: "Swipe action completed successfully",
      });
      // Prefer showing the final after screenshot if present
      const afterUri = (actionResult as any)?.screenshot?.after?.uri;
      if (afterUri) {
        const { mimeType, base64Data } = parseUri(afterUri);
        contentItems.push({ type: "image", data: base64Data, mimeType });
      }

      return {
        content: contentItems,
        isError: false,
      };
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


