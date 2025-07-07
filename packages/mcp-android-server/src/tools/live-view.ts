import { z } from "zod";
import { exec } from "child_process";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";

export const OPEN_LIVE_VIEW_TOOL = "open_live_view";
export const OPEN_LIVE_VIEW_DESCRIPTION = "Open the live view URL for an Android box in the default browser.";

export const openLiveViewParamsSchema = {
  boxId: z.string().describe("ID of the box"),
};

// Define parameter types - infer from the Zod schema
type OpenLiveViewParams = z.infer<z.ZodObject<typeof openLiveViewParamsSchema>>;

export function handleOpenLiveView(logger: MCPLogger) {
  return async (args: OpenLiveViewParams) => {
    try {
      const { boxId } = args;
      await logger.info("Opening live view", { boxId });
      
      const box = await attachBox(boxId);
      const liveViewUrl = await box.liveView();

      // Determine the appropriate command to open the URL based on the OS
      const command =
        process.platform === "darwin"
          ? `open "${liveViewUrl.url}"`
          : process.platform === "win32"
          ? `start "" "${liveViewUrl.url}"`
          : `xdg-open "${liveViewUrl.url}"`;

      // Execute the command to open the browser
      exec(command, (err) => {
        if (err) {
          console.error(`Failed to open browser for URL ${liveViewUrl.url}:`, err);
        }
      });

      await logger.info("Live view opened successfully", { boxId, url: liveViewUrl.url });

      return {
        content: [
          {
            type: "text" as const,
            text: `Opening live view in browser: ${liveViewUrl.url}`,
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to open live view", { boxId: args?.boxId, error });
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