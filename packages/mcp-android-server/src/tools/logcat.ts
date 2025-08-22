import { z } from "zod";
import { attachBox } from "../gboxsdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";

export const LOGCAT_TOOL = "logcat";

export const LOGCAT_DESCRIPTION =
  "Execute logcat command to capture Android system logs with filtering options.";

export const logcatParamsSchema = {
  boxId: z.string().describe("ID of the box"),
  source: z
    .string()
    .optional()
    .describe(
      "Log source/buffer to read from (e.g., 'main', 'system', 'radio', 'events'). Corresponds to the `-b` flag.",
    ),
  filterSpecs: z
    .string()
    .optional()
    .default("*:V")
    .describe(
      "Filter specification in format 'tag:priority' (default: '*:V' for all verbose).",
    ),
  regexFilter: z
    .string()
    .optional()
    .describe(
      "Regular expression to filter log lines (optional). Corresponds to the `--regex` flag.",
    ),
  pid: z
    .number()
    .optional()
    .describe(
      "Process ID to filter logs for specific process (optional). Corresponds to the `--pid` flag.",
    ),
  tailLines: z
    .number()
    .optional()
    .default(100)
    .describe(
      "Number of recent log lines to retrieve (default: 100). Corresponds to the `--tail` flag.",
    ),
};

// Define parameter types - infer from the Zod schema
type LogcatParams = z.infer<z.ZodObject<typeof logcatParamsSchema>>;

export function handleLogcat(logger: MCPLogger) {
  return async (args: LogcatParams) => {
    try {
      const { boxId, source, filterSpecs, regexFilter, pid, tailLines } = args;
      await logger.info("Executing logcat command", { args });

      const box = await attachBox(boxId);

      // Build the logcat command from the parameters
      const command: string[] = ["logcat"];

      // Always dump the log and exit, as this tool doesn't support streaming.
      command.push("-d");

      if (source) {
        command.push("-b", source);
      }

      if (tailLines) {
        command.push("--tail", tailLines.toString());
      }

      if (pid) {
        command.push(`--pid=${pid}`);
      }

      if (regexFilter) {
        command.push("--regex", regexFilter);
      }

      if (filterSpecs) {
        command.push(filterSpecs);
      }

      await logger.info(`Running command: ${command.join(" ")}`, { boxId });

      const result = await box.command({
        commands: command,
      });

      if (result.exitCode !== 0) {
        await logger.error("logcat command failed", {
          boxId,
          exitCode: result.exitCode,
          stderr: result.stderr,
        });
        return {
          content: [
            {
              type: "text" as const,
              text: `logcat exited with code ${result.exitCode}:\n${result.stderr}`,
            },
          ],
          isError: true,
        };
      }

      await logger.info("logcat command executed successfully", { boxId });

      return {
        content: [
          {
            type: "text" as const,
            text: result.stdout || "(No output)",
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to execute logcat command", {
        boxId: args?.boxId,
        error,
      });
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
