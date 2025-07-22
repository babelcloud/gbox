import { z } from "zod";
import { CreateAndroid } from "gbox-sdk";
import { gboxSDK } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import { openUrlInBrowser } from "./utils.js";

export const CREATE_ANDROID_BOX_TOOL = "create_android_box";
export const CREATE_ANDROID_BOX_DESCRIPTION =
  "Create a fresh Android box and return its metadata.";

export const createAndroidBoxParamsSchema = {
  config: z
    .object({
      deviceType: z
        .enum(["virtual", "physical"])
        .optional()
        .describe("Device type - virtual or physical Android device"),
      envs: z
        .record(z.string())
        .optional()
        .describe("Environment variables for the box."),
      expiresIn: z
        .string()
        .regex(/^\d+(ms|s|m|h)$/)
        .optional()
        .describe(
          'The box will be alive for the given duration (e.g., "30s", "5m", "1h"). Default: 60m'
        ),
      labels: z
        .record(z.string())
        .optional()
        .describe("Key-value pairs of labels for the box."),
    })
    .optional()
    .describe("Configuration for the Android box"),
  wait: z
    .boolean()
    .optional()
    .default(true)
    .describe("Waiting for Box to be assigned an instance, default is true"),
  openLiveView: z
    .boolean()
    .optional()
    .default(true)
    .describe("Whether to open the live view in the default browser after creating the box."),
};

// Define parameter types - infer from the Zod schema
type CreateAndroidBoxParams = z.infer<
  z.ZodObject<typeof createAndroidBoxParamsSchema>
>;

export function handleCreateAndroidBox(logger: MCPLogger) {
  return async (args: CreateAndroidBoxParams) => {
    try {
      await logger.info("Creating Android box", args);

      const created = await gboxSDK.create({
        type: "android",
        ...args,
      } as CreateAndroid);

      await logger.info("Android box created successfully", {
        boxId: created.data?.id,
      });

      if (args.openLiveView && created.data?.id) {
        const box = await gboxSDK.get(created.data.id);
        if (box) {
          const liveViewUrl = await box.liveView();
          openUrlInBrowser(liveViewUrl.url);
          await logger.info("Live view opened successfully", {
            boxId: created.data.id,
            url: liveViewUrl.url,
          });
        }
      }

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(created.data, null, 2),
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
