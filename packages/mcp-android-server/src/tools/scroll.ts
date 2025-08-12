import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../gboxsdk/index.js";
import type { ActionSwipe } from "gbox-sdk";
import { parseUri } from "./utils.js";
import { ActionSwipeResponse } from "gbox-sdk/resources/v1/boxes.mjs";

export const SCROLL_TOOL = "scroll";

export const SCROLL_DESCRIPTION =
  "Scroll the current Android sandbox screen vertically. Useful for navigating lists or pages where content is outside the viewport.";

export const scrollParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  direction: z
    .enum(["up", "down"]) // Focus on vertical scrolling for now
    .describe("Direction to scroll the screen (either 'up' or 'down'). Scroll-down is aimed to see the content below the current view. Scroll-up is aimed to see the content above the current view.")
};

type ScrollParams = z.infer<z.ZodObject<typeof scrollParamsSchema>>;

export function handleScroll(logger: MCPLogger) {
  return async (args: ScrollParams) => {
    try {
      const { boxId, direction } = args;
      await logger.info("Scroll command invoked", { boxId, direction });

      const box = await attachBox(boxId);

      // scroll and swipe are opposite concepts: invert direction for swipe
      const invertedDirection = direction === "up" ? "down" : "up";

      const { height } = (await box.display()).resolution;
      const actionParams: ActionSwipe = {
        direction: invertedDirection as any,
        includeScreenshot: true,
        outputFormat: "base64",
        screenshotDelay: "500ms",
        distance: Math.round(height / 2),
      };

      const result = await box.action.swipe(actionParams) as ActionSwipeResponse.ActionIncludeScreenshotResult;

      // Build content: brief text + after screenshot if available
      const content: Array<
        | { type: "text"; text: string }
        | { type: "image"; data: string; mimeType: string }
      > = [];

      content.push({
        type: "text",
        text: `Scrolled ${direction}`,
      });

      if (result?.screenshot?.after?.uri) {
        const { mimeType, base64Data } = parseUri(result.screenshot.after.uri);
        content.push({ type: "image", data: base64Data, mimeType });
      }

      return { content };
    } catch (error) {
      await logger.error("Failed to run scroll action", {
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


