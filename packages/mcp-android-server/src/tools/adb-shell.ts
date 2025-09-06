import { z } from "zod";
import { attachBox } from "../sdk/index.js";
import type { MCPLogger } from "../mcp-logger.js";
import type { BoxExecuteCommands } from "gbox-sdk";

export const ADB_SHELL_TOOL = "adb_shell";
export const ADB_SHELL_DESCRIPTION =
  "Executes a shell command inside a given Android box using 'adb shell'. This is useful for file system operations, process management, and other diagnostic commands.";

export const adbShellParamsSchema = {
  boxId: z
    .string()
    .describe("The ID of the Android box to run the command on."),
  shellCommand: z
    .string()
    .describe(
      "The shell command to execute (e.g., 'ls -l /sdcard', 'pm list packages', 'dumpsys battery')."
    ),
  timeout: z
    .string()
    .regex(
      /^\d+(ms|s|m|h)$/,
      "Invalid timeout format. Must be a number followed by 'ms', 's', 'm', or 'h'."
    )
    .optional()
    .describe(
      "The timeout for the command. Supported units: ms, s, m, h. (e.g., '5s', '1m'). Defaults to 30s."
    ),
  workingDir: z
    .string()
    .optional()
    .describe(
      "The working directory to run the command in. Defaults to the box's default working directory."
    ),
};

type AdbShellParams = z.infer<z.ZodObject<typeof adbShellParamsSchema>>;

export function handleAdbShell(logger: MCPLogger) {
  return async (args: AdbShellParams) => {
    try {
      const { boxId, shellCommand, timeout, workingDir } = args;
      await logger.info("Executing adb shell command", { args });

      const box = await attachBox(boxId);

      const commandParts = shellCommand.split(" ");

      const commandParams: {
        commands: string[];
        timeout?: string;
        workingDir?: string;
      } = {
        commands: commandParts,
      };

      if (timeout) {
        commandParams.timeout = timeout;
      }
      if (workingDir) {
        commandParams.workingDir = workingDir;
      }

      const result = await box.command(commandParams as BoxExecuteCommands);

      if (result.exitCode !== 0) {
        await logger.error("adb shell command failed", {
          boxId,
          exitCode: result.exitCode,
          stderr: result.stderr,
          stdout: result.stdout,
        });
        return {
          content: [
            {
              type: "text" as const,
              text: `Command failed with exit code ${result.exitCode}:\nSTDOUT:\n${result.stdout}\nSTDERR:\n${result.stderr}`,
            },
          ],
          isError: true,
        };
      }

      await logger.info("adb shell command executed successfully", {
        boxId,
        exitCode: result.exitCode,
      });

      return {
        content: [
          {
            type: "text" as const,
            text: result.stdout || "(No output)",
          },
        ],
      };
    } catch (error) {
      await logger.error("Failed to execute adb shell command", {
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
