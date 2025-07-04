import { z } from "zod";
import { withLogging } from "../utils.js";
import type { Logger } from '../mcp-logger.js';
import { Gbox } from "../gboxsdk/index.js";
import type { Browser, Page } from "playwright";

// Define the Zod schema for the tool's parameters, including name and description
export const ViewUrlAsSchema = z.object({
  name: z.literal("view-url-as"),
  description: z
    .string()
    .default(
      "Fetch and view content from a URL as HTML, Markdown, or screenshot. Optionally specify a boxId to reuse a specific browser session."
    )
    .describe(
      // Using .describe() for a potentially longer description if needed elsewhere
      "Navigates to a URL in a browser context and returns its content as HTML, Markdown, or takes a screenshot. Use the boxId parameter to maintain session state across calls."
    ),
  parameters: z.object({
    url: z.string().url({
      message:
        "The URL to fetch content from (must start with http:// or https://).",
    }),
    as: z.enum(["html", "markdown", "screenshot"], {
      errorMap: () => ({
        message:
          "The desired output format: 'html', 'markdown', or 'screenshot'.",
      }),
    }),
    boxId: z.preprocess(
      // Preprocess to handle null input
      (val) => (val === null ? undefined : val),
      z.string().optional() // Keep the original validation
    ) // Added boxId
      .describe(`The ID of an existing browser box to use.
        If not provided, the system will try to reuse an existing box with a browser image.
        The system will first try to use a running box, then a stopped box (which will be started), and finally create a new one if needed.
        Note that without boxId, multiple calls may use different boxes even if they exist.
        If you need to ensure multiple calls use the same box, you must provide a boxId.
        You can get the list of existing boxes by using the list-boxes tool.
        `),
  }),
});

// Derive constants from the schema
export const VIEW_URL_AS_TOOL = ViewUrlAsSchema.shape.name.value;
export const VIEW_URL_AS_DESCRIPTION =
  ViewUrlAsSchema.shape.description._def.defaultValue(); // Get the default value
export const viewUrlAsParams = ViewUrlAsSchema.shape.parameters.shape; // Use .shape to get the raw shape for McpServer.tool

// Define the expected content structure(s)
type TextContent = { type: "text"; text: string };
type ImageContent = { type: "image"; data: string; mimeType: string };
type ErrorContent = { type: "error"; text: string };
type ResultContent = TextContent | ImageContent | ErrorContent;

// Infer the type for the parameters object from the schema
type ViewUrlAsParams = z.infer<typeof ViewUrlAsSchema.shape.parameters>;

// Define error result interface for consistency with old implementation
export interface BrowserErrorResult {
  success: false;
  error: string;
}

// Type guard function for error results
export function isBrowserErrorResult(obj: any): obj is BrowserErrorResult {
  return obj && obj.success === false && typeof obj.error === "string";
}

// Define the internal action function for the tool
async function _handleViewUrlAs(
  logger: Logger, // Logger comes first from withLogging
  { url, as, boxId }: ViewUrlAsParams, // Destructure params
  { signal, sessionId }: { signal: AbortSignal; sessionId?: string }
): Promise<{ content: Array<ResultContent> }> {
  const gbox = new Gbox();

  let actualBoxId: string | null = null; // To store the obtained box ID
  let browser: Browser | null = null; // Declare browser outside try
  let page: Page | null = null; // Declare page outside try

  try {
    // Get or create box using the same logic as the old implementation
    const boxResult = await gbox.boxes.getOrCreateBox({
      boxId,
      sessionId,
      signal,
      waitTimeoutSeconds: 60,
    });

    // Ensure we have a valid boxId
    if (!boxResult.boxId) {
      logger.error("Failed to get or create box");
      return {
        content: [
          {
            type: "error" as const,
            text: "Failed to get or create box",
          },
        ],
      };
    }

    actualBoxId = boxResult.boxId;

    logger.info(
      `Attempting to view URL: ${url} as ${as} in box ${actualBoxId} ${
        sessionId ? `for session: ${sessionId}` : ""
      }`
    );

    // Get CDP URL for the box
    const cdpUrl = await gbox.boxes.getBoxCdpUrl(actualBoxId, {
      signal,
      sessionId,
    });

    if (!cdpUrl) {
      logger.error("Failed to get CDP URL for box");
      return {
        content: [
          {
            type: "error" as const,
            text: "Failed to get CDP URL for box",
          },
        ],
      };
    }
    // wait for box cdpserver to be ready
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Connect to browser via CDP
    const { chromium } = await import("playwright");

    browser = await chromium.connectOverCDP(cdpUrl);
    page = await browser.newPage();

    // Set a reasonable timeout
    await page.setDefaultTimeout(30000);
    
    // Navigate to the URL
    await page.goto(url, { 
      waitUntil: 'load',
      timeout: 30000 
    });

    logger.debug(`Navigated to URL: ${url}`);

    let resultText: string | undefined; // Make optional as it's not always set
    switch (as) {
      case "html": {
        const content = await page.content();
        logger.info(
          `Fetched HTML content for ${url}. Length: ${content?.length ?? 0}`
        );
        resultText = content ?? "";
        break;
      }
      case "markdown": {
        // For markdown, we'll extract text content in a more meaningful way
        const content = await page.evaluate(() => {
          // Simple extraction of meaningful text content
          const textContent = document.body.innerText || document.body.textContent || '';
          return textContent;
        });
        logger.info(
          `Fetched Markdown content for ${url}. Length: ${content?.length ?? 0}`
        );
        resultText = content ?? "";
        break;
      }
      case "screenshot": {
        const screenshot = await page.screenshot({ 
          type: 'png',
          fullPage: true 
        });
        
        if (screenshot) {
          logger.info(
            `Took screenshot for ${url}, returning base64 data (length: ${screenshot.length}).`
          );
          return {
            content: [
              {
                type: "image" as const,
                data: screenshot.toString("base64"),
                mimeType: "image/png",
              },
            ],
          };
        } else {
          const errorMsg = `Screenshot succeeded but content is missing.`;
          logger.error(errorMsg);
          return { content: [{ type: "error", text: errorMsg }] };
        }
      }
      default: {
        const exhaustiveCheck: never = as;
        const errorMsg = `Internal error: Unhandled format '${exhaustiveCheck}'`;
        logger.error(`Unhandled 'as' value: ${exhaustiveCheck}`);
        return { content: [{ type: "error", text: errorMsg }] };
      }
    }

    // Return success content for text-based formats if resultText was set
    if (resultText !== undefined) {
      return { content: [{ type: "text", text: resultText }] };
    } else {
      // Should ideally not be reached if all paths return or throw
      const errorMsg = "Internal error: Reached end of switch without result.";
      logger.error(errorMsg);
      return { content: [{ type: "error", text: errorMsg }] };
    }
  } catch (error: any) {
    const errorMsg = `Operation failed: ${error.message || String(error)}`;
    logger.error(
      `Error viewing URL ${url} as ${as}: ${error.message || error}`
    );
    // Return error content
    return { content: [{ type: "error", text: errorMsg }] };
  } finally {
    // Ensure cleanup: Close browser and page
    if (page) {
      try {
        logger.debug(`Closing page...`);
        await page.close();
      } catch (closePageError: any) {
        logger.warn(
          `Failed to close page: ${
            closePageError.message || closePageError
          }`
        );
      }
    }
    if (browser) {
      try {
        logger.debug(`Closing browser...`);
        await browser.close();
      } catch (closeBrowserError: any) {
        logger.warn(
          `Failed to close browser: ${
            closeBrowserError.message || closeBrowserError
          }`
        );
      }
    }
  }
}

// Export the wrapped function
export const handleViewUrlAs = withLogging(_handleViewUrlAs);

