import { z } from "zod";
import { MCPLogger } from "../mcp-logger.js";

export const WAIT_TOOL = "wait";
export const WAIT_TOOL_DESCRIPTION =
  "Waits for a specified duration before returning. Useful when you need to wait for something to load or for an action to complete.";

export const waitParamsSchema = z.object({
  duration: z
    .number()
    .int()
    .positive()
    .describe("The duration to wait in milliseconds."),
});

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export function handleWait(logger: MCPLogger) {
  return async (params: z.infer<typeof waitParamsSchema>) => {
    const { duration } = params;

    await sleep(duration);
    const message = `Finished waiting for ${duration}ms.`;
    await logger.info(message);
    return {
      content: [
        {
          type: "text" as const,
          text: message,
        },
      ],
    };
  };
} 