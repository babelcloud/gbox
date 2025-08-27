import { exec } from "child_process";
import sharp from "sharp";
import { logger } from "../mcp-server.js";
import { getCUACoordinates } from "./cua.service.js";
import { AndroidBoxOperator } from "gbox-sdk";

export const MAX_SCREEN_LENGTH = 1784;
export const SCREENSHOT_SIZE_THRESHOLD = Math.floor(0.7 * 1024 * 1024); // 700 KB

/**
 * Sanitizes result objects by truncating base64 data URIs to improve readability
 * while preserving the full data for actual image display
 */
export const sanitizeResult = (obj: any): any => {
  if (typeof obj !== "object" || obj === null) {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(sanitizeResult);
  }

  const sanitized: any = {};
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
  exec(command, err => {
    if (err) {
      console.error(`Failed to open browser for URL ${url}:`, err);
    }
  });
};

export const parseUri = (uri: string) => {
  let mimeType = "image/png";
  let base64Data = uri;

  if (uri.startsWith("data:")) {
    const match = uri.match(/^data:(.+);base64,(.*)$/);
    if (match) {
      mimeType = match[1];
      base64Data = match[2];
    }
  }

  return { mimeType, base64Data };
};

export function calculateResizeRatio(width: number, height: number): number {
  const ratio = Math.min(MAX_SCREEN_LENGTH / width, MAX_SCREEN_LENGTH / height);
  if (ratio >= 1) {
    return 1;
  }
  return ratio;
}

export function restoreCoordinate(
  x: number,
  y: number,
  resizeRatio: number
): { x: number; y: number } {
  // round to integer
  const restored = {
    x: Math.round(x / resizeRatio),
    y: Math.round(y / resizeRatio),
  };
  return restored;
}

/**
 * Resize the image according to the provided ratio.
 * If `resizeRatio` is >= 1 (meaning no down-scaling is necessary) or we cannot
 * determine the image dimensions, the original buffer is returned.
 *
 * @param buffer - The original image buffer
 * @param resizeRatio - Scale factor to apply (e.g. 0.5 halves width & height)
 * @returns The resized image buffer (or the original buffer if no resizing was necessary)
 */
export async function resizeImage(
  buffer: Buffer,
  resizeRatio: number
): Promise<Buffer> {
  // If ratio indicates no resize, return as-is
  if (resizeRatio >= 1) {
    return buffer;
  }

  try {
    // Handle static and animated images. `failOnError: false` lets Sharp fall back gracefully.
    const image = sharp(buffer, { animated: true, failOnError: false });
    const metadata = await image.metadata();

    const width = metadata.width ?? 0;
    const height = metadata.height ?? 0;

    // If we are unable to determine size, skip resizing.
    if (!width || !height) {
      return buffer;
    }

    const newWidth = Math.round(width * resizeRatio);

    const resizedBuffer = await image.resize(newWidth).toBuffer();

    return resizedBuffer;
  } catch (err) {
    console.error(`Failed to resize image: ${err}`);
    return buffer;
  }
}

/**
 * Resize and compress the image if it exceeds the size threshold.
 * Accepts and returns base64 (without data URI prefix) for easier transport.
 * It applies format-specific optimisations that balance quality and size.
 */
export async function maybeResizeAndCompressImage(
  base64Data: string,
  mimeType: string,
  screenSize: { width: number; height: number }
): Promise<{ base64Data: string; mimeType: string }> {
  try {
    const resizeRatio = calculateResizeRatio(
      screenSize.width,
      screenSize.height
    );
    const resizedBuffer = await resizeImage(
      Buffer.from(base64Data, "base64"),
      resizeRatio
    );

    if (resizedBuffer.length <= SCREENSHOT_SIZE_THRESHOLD) {
      return { base64Data: resizedBuffer.toString("base64"), mimeType };
    }

    let outputBuffer: Buffer<ArrayBufferLike> = resizedBuffer;

    switch (mimeType) {
      case "image/jpeg":
      case "image/jpg": {
        outputBuffer = await sharp(resizedBuffer)
          .jpeg({ quality: 80, mozjpeg: true })
          .toBuffer();
        break;
      }
      case "image/png": {
        outputBuffer = await sharp(resizedBuffer)
          .png({ quality: 80, compressionLevel: 9, palette: true })
          .toBuffer();
        break;
      }
      case "image/gif": {
        outputBuffer = await sharp(resizedBuffer, { animated: true })
          .gif({ colours: 128 })
          .toBuffer();
        break;
      }
      default:
        // Unsupported type – leave as is
        return { base64Data, mimeType };
    }

    // If compression increased the size (edge-case), keep the original
    if (outputBuffer.length >= resizedBuffer.length) {
      return { base64Data: resizedBuffer.toString("base64"), mimeType };
    }

    return { base64Data: outputBuffer.toString("base64"), mimeType };
  } catch {
    // On any processing error (including invalid base64), fall back to original
    return { base64Data, mimeType };
  }
}

export async function getImageDataFromUri(
  uri: string,
  box: AndroidBoxOperator,
  compress: boolean = true
): Promise<{ base64Data: string; mimeType: string }> {
  const { mimeType, base64Data } = parseUri(uri);
  if (compress) {
    const screenSize = (await box.display()).resolution;
    return await maybeResizeAndCompressImage(base64Data, mimeType, screenSize);
  }
  return { base64Data, mimeType };
}

export async function buildActionReturnValues(
  result: any,
  box: AndroidBoxOperator
): Promise<{
  content: Array<
    | { type: "text"; text: string }
    | { type: "image"; data: string; mimeType: string }
  >;
}> {
  // Prepare image contents for before and after screenshots
  const images: Array<{ type: "image"; data: string; mimeType: string }> = [];
  if (result?.screenshot?.after?.uri) {
    const { base64Data, mimeType } = await getImageDataFromUri(
      result.screenshot.after.uri,
      box
    );
    images.push({ type: "image", data: base64Data, mimeType });
  }
  // Build content array with text and images
  const content: Array<
    | { type: "text"; text: string }
    | { type: "image"; data: string; mimeType: string }
  > = [];

  // Add text result with sanitized data
  content.push({
    type: "text" as const,
    text: "Action completed successfully",
  });

  // Add all images
  images.forEach(img => {
    content.push({
      type: "image" as const,
      data: img.data,
      mimeType: img.mimeType,
    });
  });

  return { content };
}

export async function getBoxCoordinates(
  box: AndroidBoxOperator,
  instruction: string
): Promise<{ x: number; y: number }[]> {
  const screenshotUri = (await box.action.screenshot()).uri;
  const { base64Data, mimeType } = await getImageDataFromUri(
    screenshotUri,
    box
  );
  const coordinates = await getCUACoordinates(
    instruction,
    "data:" + mimeType + ";base64," + base64Data
  );
  if (coordinates.length === 0) {
    await logger.info("No CUA Coordinates found", { instruction });
    return [];
  }
  await logger.info("CUA Coordinates found", { coordinates });
  // restore coordinates to original screen size
  const { width, height } = (await box.display()).resolution;
  const resizeRatio = calculateResizeRatio(width, height);
  const restoredCoordinates = coordinates.map(coordinate =>
    restoreCoordinate(coordinate.x, coordinate.y, resizeRatio)
  );
  logger.info("Restored coordinates", { restoredCoordinates });
  return restoredCoordinates;
}

/**
 * Start local scrcpy instead of opening browser
 * This function handles the local environment setup and scrcpy launch
 */
export async function startLocalScrcpy(
  boxId: string,
  logger: any
): Promise<{ success: boolean; message: string }> {
  // Global process references for unified management
  let gboxCliProcess: any = null;
  let adbConnectProcess: any = null;
  let scrcpyProcess: any = null;

  try {
    await logger.info("Checking local environment...");

    // Check if gbox cli is installed
    const { execSync, spawn } = await import("child_process");
    let gboxCliAvailable = false;
    let scrcpyAvailable = false;

    try {
      execSync("gbox --version", { stdio: "pipe" });
      gboxCliAvailable = true;
      await logger.info("gbox cli is installed");
    } catch {
      await logger.info("gbox cli not installed, installing...");
      try {
        // Install gbox cli using brew
        execSync("brew install gbox", { stdio: "inherit" });
        gboxCliAvailable = true;
        await logger.info("gbox cli installation successful");
      } catch (installError) {
        await logger.warning(
          "gbox cli installation failed, please install manually: brew install gbox"
        );
      }
    }

    // Check if scrcpy is installed
    try {
      execSync("scrcpy --version", { stdio: "pipe" });
      scrcpyAvailable = true;
      await logger.info("scrcpy is installed");
    } catch {
      await logger.info("scrcpy not installed, installing...");
      try {
        // Install scrcpy based on operating system
        const currentOS = process.platform;
        if (currentOS === "darwin") {
          // macOS
          execSync("brew install scrcpy", { stdio: "inherit" });
        } else if (currentOS === "linux") {
          // Linux
          execSync(
            "sudo apt-get update && sudo apt-get install -y scrcpy",
            { stdio: "inherit" }
          );
        } else if (currentOS === "win32") {
          // Windows
          execSync("choco install scrcpy", { stdio: "inherit" });
        }
        scrcpyAvailable = true;
        await logger.info("scrcpy installation successful");
      } catch (installError) {
        await logger.warning("scrcpy installation failed, please install manually");
        const currentOS = process.platform;
        if (currentOS === "darwin") {
          await logger.warning("macOS: brew install scrcpy");
        } else if (currentOS === "linux") {
          await logger.warning("Linux: sudo apt-get install scrcpy");
        } else if (currentOS === "win32") {
          await logger.warning("Windows: choco install scrcpy");
        }
      }
    }

    // If tools are available, execute related commands
    if (gboxCliAvailable && scrcpyAvailable) {
      await logger.info("Executing local commands...");

      // Stop previous processes first (if they exist)
      await cleanupProcesses(gboxCliProcess, adbConnectProcess, scrcpyProcess, logger);

      // Start gbox cli to enable adb proxy forwarding
      await new Promise<void>((resolve, reject) => {
        gboxCliProcess = spawn("gbox", ["adb-expose"], {
          stdio: ["pipe", "pipe", "pipe"],
        });

        // Set process exit handling
        gboxCliProcess.on("error", (error: string) => {
          logger.error("gbox cli process error", error);
          reject(error);
        });

        gboxCliProcess.on("exit", (code: string) => {
          logger.warning(`gbox cli process exited, exit code: ${code}`);
        });

        gboxCliProcess.stdin.write("1\n\n");
        gboxCliProcess.stdin.write("y\n");

        let wsDialSuccess = false;
        const timeout = setTimeout(() => {
          if (!wsDialSuccess) {
            reject(new Error("gbox cli startup timeout"));
          }
        }, 30000); // 30 second timeout

        gboxCliProcess.stdout.on("data", (data: any) => {
          const output = data.toString();
          logger.info("gbox cli: " + output);
          if (output.includes("ws dial success")) {
            wsDialSuccess = true;
            clearTimeout(timeout);
            resolve();
          }
        });

        gboxCliProcess.stderr.on("data", (data: any) => {
          const output = data.toString();
          logger.info("gbox cli stderr: " + output);
          if (output.includes("ws dial success")) {
            wsDialSuccess = true;
            clearTimeout(timeout);
            resolve();
          }
        });
      });

      // Wait a bit to ensure port forwarding is fully established
      await new Promise(resolve => setTimeout(resolve, 3000));

      // Connect adb to local port
      await new Promise<void>((resolve, reject) => {
        adbConnectProcess = spawn(
          "adb",
          ["connect", "localhost:5555"],
          {
            stdio: ["pipe", "pipe", "pipe"],
          }
        );
        adbConnectProcess.on("error", (error: any) => {
          logger.error("adb connect process error", error);
          reject(error);
        });
        const timeout = setTimeout(() => {
          reject(new Error("adb connect timeout"));
        }, 15000); // 15 second timeout

        adbConnectProcess.stdout.on("data", (data: any) => {
          const output = data.toString();
          logger.info("adb connect: " + output);
          if (output.includes("connected")) {
            clearTimeout(timeout);
            resolve();
          }
        });

        adbConnectProcess.stderr.on("data", (data: any) => {
          logger.info("adb connect stderr: " + data.toString());
        });
      });

      await logger.info("gbox cli port forwarding completed");

      // Start scrcpy to connect to device
      try {
        scrcpyProcess = spawn(
          "scrcpy",
          [
            "-s",
            "localhost:5555", // Explicitly specify device address
            "--video-codec=h265",
            "--max-size=1920",
            "--max-fps=30", // Reduce frame rate for stability
            "--no-audio",
            "--keyboard=uhid",
            "--stay-awake", // Keep device awake
          ],
          {
            stdio: ["pipe", "pipe", "pipe"],
          }
        );

        scrcpyProcess.on("error", (error: any) => {
          logger.error("scrcpy process error", error);
        });

        scrcpyProcess.on("exit", (code: any) => {
          logger.warning(`scrcpy process exited, exit code: ${code}`);
          if (code !== 0) {
            logger.error(`scrcpy abnormal exit, exit code: ${code}`);
            // Try to reconnect
            setTimeout(async () => {
              try {
                await logger.info("Attempting to restart scrcpy...");
                const reconnectResult = await startLocalScrcpy(boxId, logger);
                if (reconnectResult.success) {
                  await logger.info("scrcpy reconnection successful");
                }
              } catch (reconnectError) {
                await logger.error("scrcpy reconnection failed", reconnectError);
              }
            }, 2000);
          }
        });

        // Wait for scrcpy to start
        await new Promise<void>((resolve, reject) => {
          const timeout = setTimeout(() => {
            reject(new Error("scrcpy startup timeout"));
          }, 10000);

          scrcpyProcess.stdout.on("data", (data: any) => {
            const output = data.toString();
            logger.info("scrcpy: " + output);
            if (output.includes("Connected to") || output.includes("Starting display")) {
              clearTimeout(timeout);
              resolve();
            }
          });

          scrcpyProcess.stderr.on("data", (data: any) => {
            const output = data.toString();
            logger.info("scrcpy stderr: " + output);
            if (output.includes("Connected to") || output.includes("Starting display")) {
              clearTimeout(timeout);
              resolve();
            }
          });

          // If process starts successfully but no specific output, also consider it successful
          setTimeout(() => {
            clearTimeout(timeout);
            resolve();
          }, 3000);
        });

        await logger.info("scrcpy started");
      } catch (scrcpyError) {
        await logger.warning("scrcpy startup failed", scrcpyError);
        throw scrcpyError;
      }
    } else {
      throw new Error("Required tools not installed or unavailable");
    }

    return { success: true, message: "Local scrcpy started successfully" };
  } catch (error) {
    await logger.error("Failed to start local scrcpy", error);
    // Clean up processes
    await cleanupProcesses(gboxCliProcess, adbConnectProcess, scrcpyProcess, logger);
    return { 
      success: false, 
      message: `Failed to start local scrcpy: ${error instanceof Error ? error.message : String(error)}` 
    };
  }
}

/**
 * Helper function to clean up processes
 */
async function cleanupProcesses(
  gboxCliProcess: any,
  adbConnectProcess: any,
  scrcpyProcess: any,
  logger: any
) {
  const processes = [
    { name: "gbox cli", process: gboxCliProcess },
    { name: "adb connect", process: adbConnectProcess },
    { name: "scrcpy", process: scrcpyProcess }
  ];

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
