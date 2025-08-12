import { z } from "zod";
import { CreateAndroid } from "gbox-sdk";
import { gboxSDK } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import { openUrlInBrowser } from "./utils.js";

export const START_GBOX_TOOL = "start_gbox";
export const START_GBOX_DESCRIPTION =
  "Start a GBOX(Android) by the given ID. If the GBOX ID is not provided, a new GBOX will be created. MUST call this tool first when starting a task.";

export const startGboxParamsSchema = {
  gboxId: z.string().optional().describe("The ID of the GBOX to start. If not provided, a new GBOX will be created."),
};

type StartGboxParams = z.infer<
  z.ZodObject<typeof startGboxParamsSchema>
>;

export function handleStartGbox(logger: MCPLogger) {
  return async (args: StartGboxParams) => {
    try {
      await logger.info("Starting GBOX", args);

      let { gboxId } = args;

      let box;
      if (!gboxId) {
        box = await gboxSDK.create({
          type: "android",
          ...args,
        } as CreateAndroid);
        gboxId = box.data?.id;
        await logger.info("GBOX created successfully", {
          boxId: gboxId,
        });
      } else {
        box = await gboxSDK.get(gboxId);
      }

      await logger.info("GBOX started successfully", {
        boxId: gboxId,
      });

      const result = {
        success: false,
        boxId: "",
        liveViewUrl: "",
      };
      if (box) {
        if (box) {
          const liveViewUrl = await box.liveView();
          openUrlInBrowser(liveViewUrl.url);
          await logger.info("Live view opened successfully", {
            boxId: gboxId,
            url: liveViewUrl.url,
          });
          result.success = true;
          result.boxId = gboxId;
          result.liveViewUrl = liveViewUrl.url;
        }
      }

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(result, null, 2),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to create Android box", error);
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
