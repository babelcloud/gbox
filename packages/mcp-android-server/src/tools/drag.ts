import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../gboxsdk/index.js";
import { handleUiAction } from "./ui-action.js";

export const DRAG_TOOL = "drag";

export const DRAG_DESCRIPTION =
  "Drag a UI element on the Android device by long-pressing and moving it to a new location – for example, reorganising home-screen icons or moving an item into a folder.";

export const dragParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  target: z
    .string()
    .describe(
      "Description of the element to drag (e.g. ‘app icon’, ‘list item’). MUST be detailed enough to identify the element unambiguously."
    ),
  destination: z
    .string()
    .describe(
      "Description of the destination where the element should be dropped (e.g. ‘trash bin at bottom’, ‘other folder icon’)."
    ),
};

// Define parameter types - infer from the Zod schema
type DragParams = z.infer<z.ZodObject<typeof dragParamsSchema>>;

export function handleDrag(logger: MCPLogger) {
  return async (args: DragParams) => {
    try {
      const { boxId, target, destination } = args;
      await logger.info("Drag command invoked", { boxId, target, destination });

      // call gbox ai action to drag params
      const aiActionResult = handleUiAction(logger)({
        boxId,
        instruction: `Drag ${target} to ${destination}`,
      });

      return aiActionResult;
    } catch (error) {
      await logger.error("Failed to run drag action", {
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


