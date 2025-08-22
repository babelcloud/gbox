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
  exec(command, (err) => {
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
  resizeRatio: number,
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
  resizeRatio: number,
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
  screenSize: { width: number; height: number },
): Promise<{ base64Data: string; mimeType: string }> {
  try {
    const resizeRatio = calculateResizeRatio(
      screenSize.width,
      screenSize.height,
    );
    const resizedBuffer = await resizeImage(
      Buffer.from(base64Data, "base64"),
      resizeRatio,
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
  compress: boolean = true,
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
  box: AndroidBoxOperator,
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
      box,
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
  images.forEach((img) => {
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
  instruction: string,
): Promise<{ x: number; y: number }[]> {
  const screenshotUri = (await box.action.screenshot()).uri;
  const { base64Data, mimeType } = await getImageDataFromUri(
    screenshotUri,
    box,
  );
  const coordinates = await getCUACoordinates(
    instruction,
    "data:" + mimeType + ";base64," + base64Data,
  );
  if (coordinates.length === 0) {
    await logger.info("No CUA Coordinates found", { instruction });
    return [];
  }
  await logger.info("CUA Coordinates found", { coordinates });
  // restore coordinates to original screen size
  const { width, height } = (await box.display()).resolution;
  const resizeRatio = calculateResizeRatio(width, height);
  const restoredCoordinates = coordinates.map((coordinate) =>
    restoreCoordinate(coordinate.x, coordinate.y, resizeRatio),
  );
  logger.info("Restored coordinates", { restoredCoordinates });
  return restoredCoordinates;
}
