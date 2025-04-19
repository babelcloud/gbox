import type { Logger } from "./sdk/types";
import type { LogFn } from "./types";

export class MCPLogger implements Logger {
  constructor(private logFn: LogFn) {}

  debug(message: string, ...args: any[]): void {
    this.logFn({
      level: "debug",
      data: this.format(message, ...args),
    });
  }

  info(message: string, ...args: any[]): void {
    this.logFn({
      level: "info",
      data: this.format(message, ...args),
    });
  }

  warn(message: string, ...args: any[]): void {
    this.logFn({
      level: "warning",
      data: this.format(message, ...args),
    });
  }

  error(message: string, ...args: any[]): void {
    this.logFn({
      level: "error",
      data: this.format(message, ...args),
    });
  }

  private format(message: string, ...args: any[]): string {
    if (args.length === 0) return message;

    try {
      return args.reduce((msg, arg) => {
        if (typeof arg === "object") {
          return msg.replace("%o", JSON.stringify(arg));
        }
        return msg.replace(/%[sdfo]/, String(arg));
      }, message);
    } catch (error) {
      return message;
    }
  }
}