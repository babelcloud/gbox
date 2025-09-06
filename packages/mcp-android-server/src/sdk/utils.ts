import { exec, execSync, spawn } from "child_process";
import { logger } from "../mcp-server.js";

export const MAX_SCREEN_LENGTH = 1784;
export const SCREENSHOT_SIZE_THRESHOLD = Math.floor(0.7 * 1024 * 1024); // 700 KB

/**
 * Install scrcpy based on the current operating system
 * @returns Promise<boolean> - true if installation successful, false otherwise
 */
export async function installScrcpy(): Promise<boolean> {
  try {
    const currentOS = process.platform;

    if (currentOS === "darwin") {
      // macOS
      execSync("brew install scrcpy", { stdio: "inherit" });
    } else if (currentOS === "linux") {
      // Linux
      execSync("sudo apt-get update && sudo apt-get install -y scrcpy", {
        stdio: "inherit",
      });
    } else if (currentOS === "win32") {
      // Windows
      execSync("choco install scrcpy", { stdio: "inherit" });
    } else {
      throw new Error(`Unsupported operating system: ${currentOS}`);
    }

    await logger.info("scrcpy installation successful");
    return true;
  } catch {
    await logger.warning("scrcpy installation failed, please install manually");

    const currentOS = process.platform;
    if (currentOS === "darwin") {
      await logger.warning("run: brew install scrcpy");
    } else if (currentOS === "linux") {
      await logger.warning("run: sudo apt-get install scrcpy");
    } else if (currentOS === "win32") {
      await logger.warning("run: choco install scrcpy");
    }

    return false;
  }
}

/**
 * Sanitizes result objects by truncating base64 data URIs to improve readability
 * while preserving the full data for actual image display
 */
export const sanitizeResult = (obj: unknown): unknown => {
  if (typeof obj !== "object" || obj === null) {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(sanitizeResult);
  }

  const sanitized: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(obj)) {
    if (key === "uri" && typeof value === "string") {
      // Truncate base64 data URIs
      if (value.startsWith("data:")) {
        const match = value.match(/^(data:.+;base64,)(.*)$/);
        if (match && match[2].length > 20) {
          sanitized[key] = match[1] + match[2].substring(0, 20) + "...";
        } else {
          sanitized[key] = value;
        }
      } else if (value.length > 20) {
        // Truncate other long strings that might be base64
        sanitized[key] = value.substring(0, 20) + "...";
      } else {
        sanitized[key] = value;
      }
    } else {
      sanitized[key] = sanitizeResult(value);
    }
  }
  return sanitized;
};

export const openUrlInBrowser = (url: string) => {
  const command =
    process.platform === "darwin"
      ? `open "${url}"`
      : process.platform === "win32"
        ? `start "" "${url}"`
        : `xdg-open "${url}"`;

  // Execute the command to open the browser
  exec(command, (err: Error | null) => {
    if (err) {
      console.error(`Failed to open browser for URL ${url}:`, err);
    }
  });
};

/**
 * Extract image data and MIME type from a URI string
 * @param uri - The URI string to parse
 * @returns Object containing mimeType and base64Data
 */
/**
 * Type for image data extracted from URI
 */
export type ImageInfo = {
  mimeType: string;
  data: string;
};

/**
 * Type for screen resolution
 */
export type Resolution = {
  width: number;
  height: number;
};

export const extractImageInfo = (uri: string): ImageInfo => {
  let mimeType = "image/png";
  let data = uri;

  if (uri.startsWith("data:")) {
    const match = uri.match(/^data:(.+);base64,(.*)$/);
    if (match) {
      mimeType = match[1];
      data = match[2];
    }
  }

  return { mimeType, data };
};

export function calculateResizeRatio({
  width,
  height,
}: {
  width: number;
  height: number;
}): number {
  const ratio = Math.min(MAX_SCREEN_LENGTH / width, MAX_SCREEN_LENGTH / height);
  if (ratio >= 1) {
    return 1;
  }
  return ratio;
}

/**
 * Start local scrcpy instead of opening browser
 * This function handles the local environment setup and scrcpy launch
 */
interface Logger {
  info: (message: string, data?: unknown) => Promise<void>;
  warning: (message: string, data?: unknown) => Promise<void>;
  error: (message: string, data?: unknown) => Promise<void>;
}

export async function startLocalScrcpy(
  logger: Logger,
  deviceId: string
): Promise<{ success: boolean; message: string }> {
  // Global process references for unified management
  let scrcpyProcess: ReturnType<typeof spawn> | null = null;

  try {
    await logger.info("Checking local environment...");

    let scrcpyAvailable = false;

    // Check if scrcpy is installed
    try {
      execSync("scrcpy --version", { stdio: "pipe" });
      scrcpyAvailable = true;
      await logger.info("scrcpy is installed");
    } catch {
      await logger.info("scrcpy not installed, installing...");
      scrcpyAvailable = await installScrcpy();
    }

    // If tools are available, execute related commands
    if (scrcpyAvailable) {
      await logger.info("Executing local commands...", { deviceId });
      // Stop previous processes first (if they exist)
      await cleanupProcesses(scrcpyProcess, logger);
      // Start scrcpy to connect to device
      const match = deviceId.match(/^([^-]+)-usb$/);
      if (!match) {
        throw new Error("Invalid device ID");
      }
      const deviceName = match[1];
      const scrcpyArgs = [
        "--video-codec=h265",
        "--max-size=1920",
        "--max-fps=30",
        "--no-audio",
        "--keyboard=uhid",
      ];
      if (match[1]) {
        scrcpyArgs.unshift("-s", deviceName);
      }
      try {
        scrcpyProcess = spawn("scrcpy", scrcpyArgs, {
          stdio: ["pipe", "pipe", "pipe"],
        });
        if (scrcpyProcess.stdout) {
          scrcpyProcess.stdout.on("data", (data: Buffer) => {
            const output = data.toString();
            logger.info("scrcpy: " + output);
            if (output.includes("[server] INFO")) {
              logger.info("scrcpy started");
            }
          });
        }

        if (scrcpyProcess.stderr) {
          scrcpyProcess.stderr.on("data", (data: Buffer) => {
            const output = data.toString();
            logger.info("scrcpy stderr: " + output);
          });
        }

        scrcpyProcess.on("error", (error: Error) => {
          logger.error("scrcpy process error", error);
        });

        scrcpyProcess.on("exit", (code: number | null) => {
          logger.warning(`scrcpy process exited, exit code: ${code}`);
        });

        await logger.info("scrcpy started");
      } catch (scrcpyError) {
        await logger.warning("scrcpy startup failed", scrcpyError);
        throw scrcpyError;
      }
      return {
        success: true,
        message: "Local scrcpy started successfully",
      };
    } else {
      throw new Error("Required tools not installed or unavailable");
    }
  } catch (error) {
    await logger.error("Failed to start local scrcpy", error);
    await cleanupProcesses(scrcpyProcess, logger);
    return {
      success: false,
      message: `Failed to start local scrcpy: ${error instanceof Error ? error.message : String(error)}`,
    };
  }
}

/**
 * Helper function to clean up processes
 */
async function cleanupProcesses(
  scrcpyProcess: ReturnType<typeof spawn> | null,
  logger: Logger
) {
  const processes = [{ name: "scrcpy", process: scrcpyProcess }];

  for (const { name, process } of processes) {
    if (process && !process.killed) {
      try {
        process.kill();
        await logger.info(`Stopped previous ${name} process`);
      } catch (e) {
        await logger.warning(`Failed to stop previous ${name} process`, e);
      }
    }
  }
}
