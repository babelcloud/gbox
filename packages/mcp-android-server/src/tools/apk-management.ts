import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { AndroidInstall } from "gbox-sdk";

export const INSTALL_APK_TOOL = "install_apk";
export const INSTALL_APK_DESCRIPTION = "Install an APK file into the Gbox Android box.";

export const UNINSTALL_APK_TOOL = "uninstall_apk";
export const UNINSTALL_APK_DESCRIPTION = "Uninstall an app from the Android box by package name.";

export const OPEN_APP_TOOL = "open_app";
export const OPEN_APP_DESCRIPTION = "Launch an installed application by package name on the Android box.";

export const CLOSE_APP_TOOL = "close_app";
export const CLOSE_APP_DESCRIPTION = "Close an installed application by package name on the Android box.";

export const installApkParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  apk: z
    .string()
    .optional()
    .describe(
      "Local file path or HTTP(S) URL of the APK to install, for example: '/Users/jack/abc.apk', if local file provided, Gbox SDK will upload it to the box and install it. if apk is a url, Gbox SDK will download it to the box and install it(please make sure the url is public internet accessible)."
    ),
};

export const uninstallApkParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  packageName: z.string().describe("Android package name to uninstall"),
};

export const openAppParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  packageName: z.string().describe("Android package name to open, for example: 'com.android.settings'"),
};

export const closeAppParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  packageName: z.string().describe("Android package name to close, for example: 'com.android.settings'"),
};

// Define parameter types - infer from the Zod schemas
type InstallApkParams = z.infer<z.ZodObject<typeof installApkParamsSchema>>;
type UninstallApkParams = z.infer<z.ZodObject<typeof uninstallApkParamsSchema>>;
type OpenAppParams = z.infer<z.ZodObject<typeof openAppParamsSchema>>;
type CloseAppParams = z.infer<z.ZodObject<typeof closeAppParamsSchema>>;

export function handleInstallApk(logger: MCPLogger) {
  return async (args: InstallApkParams) => {
    try {
      const { boxId, apk } = args;
      await logger.info("Installing APK", { boxId, apk });
      
      const box = await attachBox(boxId);
      let apkPath = apk;
      if (apk?.startsWith("file://")) {
        apkPath = apk.slice(7);
      }

      // Map to SDK AndroidInstall type
      const installParams: AndroidInstall = { apk: apkPath! };
      const appOperator = await box.app.install(installParams);

      await logger.info("APK installed successfully", { boxId, apk: apkPath });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify(appOperator.data, null, 2),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to install APK", { boxId: args?.boxId, apk: args?.apk, error });
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

export function handleUninstallApk(logger: MCPLogger) {
  return async (args: UninstallApkParams) => {
    try {
      const { boxId, packageName } = args;
      await logger.info("Uninstalling APK", { boxId, packageName });
      
      const box = await attachBox(boxId);
      await box.app.uninstall(packageName, {});

      await logger.info("APK uninstalled successfully", { boxId, packageName });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify({ packageName, status: "uninstalled" }),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to uninstall APK", { boxId: args?.boxId, packageName: args?.packageName, error });
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

export function handleOpenApp(logger: MCPLogger) {
  return async (args: OpenAppParams) => {
    try {
      const { boxId, packageName } = args;
      await logger.info("Opening app", { boxId, packageName });
      
      const box = await attachBox(boxId);
      const app = await box.app.get(packageName);
      await app.open();

      await logger.info("App opened successfully", { boxId, packageName });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify({ packageName, status: "opened" }),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to open app", { boxId: args?.boxId, packageName: args?.packageName, error });
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

export function handleCloseApp(logger: MCPLogger) {
  return async (args: CloseAppParams) => {
    try {
      const { boxId, packageName } = args;
      await logger.info("Closing app", { boxId, packageName });

      const box = await attachBox(boxId);
      const app = await box.app.get(packageName);
      await app.close();

      await logger.info("App closed successfully", { boxId, packageName });

      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify({ packageName, status: "closed" }),
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to close app", { boxId: args?.boxId, packageName: args?.packageName, error });
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
  }
}