import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../gboxsdk/index.js";
import {
  buildActionReturnValues,
  getBoxCoordinates,
} from "../gboxsdk/utils.js";

export const TAP_TOOL = "tap";

export const TAP_DESCRIPTION =
  "Tap a UI element on the Android device. Provide a clear description of the element to ensure it can be identified unambiguously.";

export const tapParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  target: z
    .string()
    .describe(
      "Description of the element to tap (e.g. 'login button', 'search field'). MUST be detailed enough to identify the element unambiguously."
    ),
};

type TapParams = z.infer<z.ZodObject<typeof tapParamsSchema>>;

export function handleTap(logger: MCPLogger) {
  return async (args: TapParams) => {
    try {
      const { boxId, target } = args;
      await logger.info("Tap command invoked", { boxId, target });

      const box = await attachBox(boxId);
      const boxCoordinates = await getBoxCoordinates(box, "Click " + target);
      if (boxCoordinates.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No coordinates found" }],
        };
      }
      const clickAction = {
        ...boxCoordinates[0],
        includeScreenshot: true,
        outputFormat: "base64" as const,
        screenshotDelay: "500ms" as const,
      };
      const result = (await box.action.click(clickAction)) as any;

      return buildActionReturnValues(result, box);
    } catch (error) {
      await logger.error("Failed to run tap action", {
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
