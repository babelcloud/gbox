import { z } from "zod";
import type { MCPLogger } from "../mcp-logger.js";
import { attachBox } from "../sdk/index.js";
import { extractImageInfo } from "../sdk/utils.js";

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
  return async ({ boxId, target }: TapParams) => {
    try {
      await logger.info("Tap command invoked", { boxId, target });

      const box = await attachBox(boxId);

      const result = await box.action.tap({
        target,
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
            text: "Tap action completed successfully",
          },
          {
            type: "image" as const,
            ...extractImageInfo(result.screenshot.after.uri),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to run tap action", {
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
