import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../sdk/index.js";
import { extractImageInfo } from "../sdk/utils.js";

export const SCROLL_TOOL = "scroll";

export const SCROLL_DESCRIPTION =
  "Scroll the current Android sandbox screen vertically. Useful for navigating lists or pages where content is outside the viewport.";

export const scrollParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  direction: z
    .enum(["up", "down"]) // Focus on vertical scrolling for now
    .describe(
      "Direction to scroll the screen (either 'up' or 'down'). Scroll-down is aimed to see the content below the current view. Scroll-up is aimed to see the content above the current view."
    ),
};

type ScrollParams = z.infer<z.ZodObject<typeof scrollParamsSchema>>;

export function handleScroll(logger: MCPLogger) {
  return async ({ boxId, direction }: ScrollParams) => {
    try {
      await logger.info("Scroll command invoked", { boxId, direction });

      const box = await attachBox(boxId);

      // scroll and swipe are opposite concepts: invert direction for swipe
      const invertedDirection = direction === "up" ? "down" : "up";

      const { height } = (await box.display()).resolution;

      const result = await box.action.swipe({
        direction: invertedDirection,
        options: {
          screenshot: {
            outputFormat: "base64",
            delay: "500ms",
          },
        },
        distance: Math.round(height / 2),
      });

      return {
        content: [
          {
            type: "text" as const,
            text: `Scrolled ${direction}`,
          },
          {
            type: "image" as const,
            ...extractImageInfo(result.screenshot.after.uri),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to run scroll action", {
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
