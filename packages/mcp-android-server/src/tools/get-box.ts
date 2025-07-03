import { z } from "zod";
import { gboxSDK } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";

export const GET_BOX_TOOL = "get_box";
export const GET_BOX_DESCRIPTION = "Get box information by ID.";

export const getBoxParamsSchema = {
  boxId: z.string().describe("ID of the box"),
};

// Define parameter types - infer from the Zod schema
type GetBoxParams = z.infer<z.ZodObject<typeof getBoxParamsSchema>>;

export function handleGetBox(logger: MCPLogger) {
  return async (args: GetBoxParams) => {
    try {
      const { boxId } = args;
      await logger.info("Getting box information", { boxId });
      
      const info = await gboxSDK.getInfo(boxId);

      await logger.info("Retrieved box information", { boxId });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(info, null, 2),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to get box information", { boxId: args?.boxId, error });
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