import { z } from "zod";
import { gboxSDK } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";

export const LIST_BOXES_TOOL = "list_boxes";
export const LIST_BOXES_DESCRIPTION =
  "List all current boxes belonging to the current organization(API Key).";

// Zod schema derived from BoxListParams in gbox-sdk
export const listBoxesParamsSchema = {
  deviceType: z
    .string()
    .optional()
    .describe("Filter boxes by their device type (virtual, physical)"),
  labels: z
    .array(z.string())
    .optional()
    .describe(
      "Filter boxes by their labels. Labels are key-value pairs that help identify and categorize boxes."
    ),
  page: z.number().int().optional().describe("Page number"),
  pageSize: z.number().int().optional().describe("Page size"),
  status: z
    .array(z.enum(["all", "pending", "running", "error", "terminated"]))
    .optional()
    .describe(
      "Filter boxes by their current status (pending, running, stopped, error, terminated, all)."
    ),
  type: z
    .array(z.enum(["all", "linux", "android"]))
    .optional()
    .describe(
      "Filter boxes by their type (linux, android, all). Must be an array of types."
    ),
};

// Define parameter types - infer from the Zod schema
type ListBoxesParams = z.infer<z.ZodObject<typeof listBoxesParamsSchema>>;

export function handleListBoxes(logger: MCPLogger) {
  return async (args: ListBoxesParams) => {
    try {
      await logger.info("Listing boxes", args);

      const boxes = await gboxSDK.listInfo(args);

      await logger.info("Retrieved boxes list", {
        count: boxes?.data?.length || 0,
      });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(boxes, null, 2),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to list boxes", error);
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
