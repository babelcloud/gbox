import { z } from "zod";
import { CreateAndroid } from "gbox-sdk";
import { gboxSDK } from "../sdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import { openUrlInBrowser, startLocalScrcpy } from "../sdk/utils.js";
import { deviceList } from "../sdk/android.service.js";
import { calculateResizeRatio } from "../sdk/utils.js";

export const START_BOX_TOOL = "start_box";
export const START_BOX_DESCRIPTION =
  "Start a GBOX(Android) by the given ID. If the GBOX ID is not provided, a new GBOX will be created. MUST call this tool first when starting a task.";

export const startBoxParamsSchema = {
  gboxId: z
    .string()
    .optional()
    .describe(
      "The ID of the GBOX to start. If not provided, a new GBOX will be created."
    ),
};

type StartBoxParams = z.infer<z.ZodObject<typeof startBoxParamsSchema>>;

export function handleStartBox(logger: MCPLogger) {
  return async (args: StartBoxParams) => {
    try {
      await logger.info("Starting Box", args);

      let { gboxId } = args;

      let box;
      let deviceId = "";
      let deviceModel = "";
      if (!gboxId) {
        // If local physical device available, use it
        const devices = await deviceList();
        logger.info("Devices", { devices });
        if (devices.length > 0) {
          // Always use the first available device
          deviceId = devices[0].deviceId.trim();
          deviceModel = devices[0].productModel.trim();
        }
        const labels: Record<string, string> = {};
        if (deviceId) {
          labels["gbox.ai/device-id"] = deviceId;
        }
        if (deviceModel) {
          labels["gbox.ai/model"] = deviceModel;
        }

        box = await gboxSDK.create({
          type: "android",
          config: {
            labels,
            deviceType: deviceId ? "physical" : "virtual",
          },
        } as CreateAndroid);
        gboxId = box.data?.id;
        await logger.info("GBOX created successfully", {
          boxId: gboxId,
        });

        const { resolution } = await box.display();
        box.action.updateSettings({
          scale: calculateResizeRatio(resolution),
        });

        await logger.info("Box action settings scaled to 0.5 successfully", {
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
          await logger.info("Live view created successfully", {
            boxId: gboxId,
            url: liveViewUrl.url,
          });
          if (!deviceId) {
            openUrlInBrowser(liveViewUrl.url);
          } else {
            // Start local scrcpy instead of opening browser
            const scrcpyResult = await startLocalScrcpy(logger, deviceId);
            if (scrcpyResult.success) {
              await logger.info("Local scrcpy started successfully", {
                boxId: gboxId,
                message: scrcpyResult.message,
              });
            } else {
              await logger.warning("Local scrcpy failed to start", {
                boxId: gboxId,
                message: scrcpyResult.message,
              });
            }
          }
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
