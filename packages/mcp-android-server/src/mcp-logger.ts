import type { LogFn } from "./types.js";

export class MCPLogger {
  private logFn: LogFn;

  constructor(logFn: LogFn) {
    this.logFn = logFn;
  }

  async debug(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "debug",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async info(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "info",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async warning(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "warning",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async error(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "error",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async notice(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "notice",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async critical(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "critical",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async alert(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "alert",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async emergency(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "emergency",
      data: args.length > 0 ? { message, args } : message,
    });
  }

  async trace(message: string, ...args: any[]): Promise<void> {
    await this.logFn({
      level: "debug",
      data: args.length > 0 ? { message, args } : message,
    });
  }
}