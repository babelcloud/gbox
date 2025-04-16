import { withLogging } from "../utils.js";
import { config } from "../config.js";
import { GBox } from "../sdk/index.js";
import { MCPLogger } from "../mcp-logger.js";
import { z } from "zod";

export const RUN_BASH_TOOL = "run-bash";
export const RUN_BASH_DESCRIPTION = `Run Bash commands in a sandbox. 
To persist files after sandbox reclamation, save them to /var/gbox/share directory. 
Files in this directory will be retained for a period of time after the sandbox is reclaimed.
The default working directory is /var/gbox.
To read files generated by your program, use the read-file tool with the boxId returned from this tool.`;

export const runBashParams = {
  code: z.string().describe(`The Bash command to run. This command will be executed through the Bash interpreter directly and will not be saved to a file.`),
  boxId: z.string().optional()
    .describe(`The ID of an existing box to run the command in.
      If not provided, the system will try to reuse an existing box with matching image.
      The system will first try to use a running box, then a stopped box (which will be started), and finally create a new one if needed.
      Note that without boxId, multiple calls may use different boxes even if they exist.
      If you need to ensure multiple calls use the same box, you must provide a boxId.
      You can get the list of existing boxes by using the list-boxes tool.
      `),
};

export const handleRunBash = withLogging(
  async (log, { boxId, code }, { signal, sessionId }) => {
    const logger = new MCPLogger(log);
    const gbox = new GBox({
      apiUrl: config.apiServer.url,
      logger,
    });

    logger.info(
      `Executing Bash command in box ${boxId || "new box"} ${
        sessionId ? `for session: ${sessionId}` : ""
      }`
    );

    // Get or create box
    const id = await gbox.box.getOrCreateBox({
      boxId,
      image: config.images.bash,
      sessionId,
      signal,
    });

    // Run command
    const result = await gbox.box.runInBox(
      id,
      ["/bin/bash"],
      code,
      100, // stdoutLineLimit
      100, // stderrLineLimit
      { signal, sessionId }
    );

    log({ level: "info", data: "Bash command executed successfully" });
    if (!result.stderr && !result.stdout) {
      result.stdout = "[No output]";
    }
    return {
      content: [
        {
          type: "text" as const,
          text: JSON.stringify(result, null, 2),
        },
      ],
    };
  }
);
