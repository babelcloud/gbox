import type { LoggingMessageNotification } from "@modelcontextprotocol/sdk/types.js";

export type LogFn = (params: LoggingMessageNotification["params"]) => Promise<void>;