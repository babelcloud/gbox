import { z } from "zod";
import { attachBox } from "../sdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { ActionAI } from "gbox-sdk";
import { extractImageInfo } from "../sdk/utils.js";
import type { ActionAIResponse } from "gbox-sdk/resources/v1/boxes.js";

export const UI_ACTION_TOOL = "ui_action";
export const UI_ACTION_DESCRIPTION =
  "Perform an action on the UI of the android box (natural language instruction). Here's some example instructions: \n\n" +
  "Tap the email input field\n" +
  "Tap the submit button\n" +
  "Tap the plus button in the upper right corner\n" +
  "Scroll up to next \n" +
  "Fill the search field with text: 'gbox ai' \n" +
  "Swipe to next screen\n" +
  "Swipe up to next video\n" +
  "Press back button\n" +
  "Double click the video\n" +
  "Slide the top slide to the next screen\n" +
  "Pull down to refresh the page";
export const uiActionParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  instruction: z
    .string()
    .describe(
      "Direct instruction of the UI action to perform, e.g. 'Tap the login button'"
    ),
  background: z
    .string()
    .optional()
    .describe(
      "Contextual background for the action, to help the AI understand previous steps and the current state of the UI."
    ),
  // includeScreenshot: z
  //   .boolean()
  //   .optional()
  //   .describe(
  //     "Whether to include screenshots in the action response (default false)"
  //   ),
  // outputFormat: z
  //   .enum(["base64", "storageKey"])
  //   .optional()
  //   .describe("Output format for screenshot URIs (default 'base64')"),
  // settings: z
  //   .object({
  //     systemPrompt: z
  //       .string()
  //       .default(
  //         "You are a helpful assistant that can operate Android devices. \n" +
  //           "You are given an instruction and a background of the task to perform. \n" +
  //           "You can see the current screen in the image. Analyze what you see and determine the next action needed to complete the task. \n" +
  //           "Take your time to analyze the screen and plan your actions carefully. Tips: - You should execute the action directly by the instruction. \n" +
  //           "- If you see the Keyboard on the bottom of the screen, that means the field you should type is already focused. You should type directly no need to focus on the field."
  //       )
  //       .describe(
  //         "System prompt that defines the AI's behavior and capabilities when executing UI actions. \n" +
  //           "This prompt instructs the AI on how to interpret the screen, understand user instructions, and determine the appropriate UI actions to take. \n" +
  //           "A well-crafted system prompt can significantly improve the accuracy and reliability of AI-driven UI automation. \n" +
  //           "If not provided, uses the default computer use instruction template that includes basic screen interaction guidelines."
  //       ),
  //   })
  //   .optional()
  //   .describe("Settings for the AI action"),
};

// Define parameter types - infer from the Zod schema
type UiActionParams = z.infer<z.ZodObject<typeof uiActionParamsSchema>>;

export function handleUiAction(logger: MCPLogger) {
  return async ({ boxId, instruction, background }: UiActionParams) => {
    try {
      await logger.info("Performing UI action", { boxId, instruction });

      const box = await attachBox(boxId);

      // Map to SDK ActionAI type
      const actionParams: ActionAI = {
        instruction,
        ...(background && { background }),
        options: {
          screenshot: {
            phases: ["after"],
            outputFormat: "base64",
            delay: "500ms",
          },
        },
      };

      const result = (await box.action.ai(
        actionParams
      )) as ActionAIResponse.AIActionScreenshotResult;

      await logger.info("UI action completed", {
        boxId,
        imageCount: result?.screenshot?.after?.uri ? 1 : 0,
      });

      return {
        content: [
          {
            type: "text" as const,
            text: "Action completed successfully",
          },
          ...(result?.screenshot?.after?.uri
            ? [
                {
                  type: "image" as const,
                  ...extractImageInfo(result.screenshot.after.uri),
                },
              ]
            : []),
        ],
      };
    } catch (error) {
      await logger.error("Failed to perform AI action", {
        boxId,
        error,
      });
      return {
        content: [
          {
            type: "text" as const,
            text: `Error: ${
              error instanceof Error ? error.message : String(error)
            }`,
          },
        ],
        isError: true,
      };
    }
  };
}
