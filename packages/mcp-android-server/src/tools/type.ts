import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionType, ActionPressKey } from "gbox-sdk";
import { getImageDataFromUri } from "../gboxsdk/utils.js";
import {
  ActionPressKeyResponse,
  ActionTypeResponse,
} from "gbox-sdk/resources/v1/boxes.mjs";

export const TYPE_TOOL = "type";

export const TYPE_DESCRIPTION =
  "Type text content into the currently focused input on the Android device. Optionally press Enter after typing, and choose to replace or append.";

export const typeParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  content: z.string().describe("The text content to type."),
  pressEnterAfterType: z
    .boolean()
    .optional()
    .describe(
      "Whether to press the Enter key after typing the content. Defaults to false.",
    ),
  replace: z
    .boolean()
    .optional()
    .describe(
      "If true, replace existing text; if false, append to the end of current text. Defaults to false.",
    ),
};

type TypeParams = z.infer<z.ZodObject<typeof typeParamsSchema>>;

export function handleType(logger: MCPLogger) {
  return async (args: TypeParams) => {
    try {
      const { boxId, content, pressEnterAfterType, replace } = args;
      await logger.info("Typing content", {
        boxId,
        length: content.length,
        pressEnterAfterType: Boolean(pressEnterAfterType),
        replace: Boolean(replace),
      });

      const box = await attachBox(boxId);

      // First, type the content
      const typeParams: ActionType = {
        text: content,
        mode: replace ? "replace" : "append",
        includeScreenshot: true,
        outputFormat: "base64",
        screenshotDelay: "500ms",
      };

      const typeResult = (await box.action.type(
        typeParams,
      )) as ActionTypeResponse.ActionIncludeScreenshotResult;

      // Optionally press Enter afterwards
      let finalResult: any = typeResult;
      if (pressEnterAfterType) {
        const pressParams: ActionPressKey = {
          keys: ["enter"],
          includeScreenshot: true,
          outputFormat: "base64",
          screenshotDelay: "500ms",
        };
        finalResult = (await box.action.pressKey(
          pressParams,
        )) as ActionPressKeyResponse.ActionIncludeScreenshotResult;
      }

      // Build response content
      const contentItems: Array<
        | { type: "text"; text: string }
        | { type: "image"; data: string; mimeType: string }
      > = [];

      // Add sanitized JSON of the final result
      contentItems.push({
        type: "text",
        text: "Text typed successfully",
      });

      // Prefer showing the final after screenshot if present
      const afterUri = finalResult?.screenshot?.after?.uri;
      if (afterUri) {
        const { base64Data, mimeType } = await getImageDataFromUri(
          afterUri,
          box,
        );
        contentItems.push({ type: "image", data: base64Data, mimeType });
      }

      return { content: contentItems };
    } catch (error) {
      await logger.error("Failed to type content", {
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
