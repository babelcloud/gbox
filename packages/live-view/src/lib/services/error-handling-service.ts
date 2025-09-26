// Error handling service for centralized error management and recovery
import { ReconnectionService } from "./reconnection-service";

export interface ErrorHandlingServiceOptions {
  onError?: (error: Error, context: string) => void;
  onRecoverableError?: (error: Error, context: string) => void;
  onFatalError?: (error: Error, context: string) => void;
  enableRetry?: boolean;
  maxRetries?: number;
  retryDelay?: number;
  enableErrorReporting?: boolean;
  enableErrorRecovery?: boolean;
  errorContext?: string;
}

export interface ErrorContext {
  component: string;
  operation: string;
  timestamp: number;
  metadata?: Record<string, unknown>;
}

export interface ErrorRecoveryStrategy {
  canRecover: (error: Error, context: ErrorContext) => boolean;
  recover: (error: Error, context: ErrorContext) => Promise<void>;
  maxRetries?: number;
  retryDelay?: number;
}

export class ErrorHandlingService {
  private options: Required<ErrorHandlingServiceOptions>;
  private reconnectionService: ReconnectionService | null = null;
  private errorHistory: Array<{
    error: Error;
    context: ErrorContext;
    timestamp: number;
  }> = [];
  private recoveryStrategies: Map<string, ErrorRecoveryStrategy> = new Map();
  private isActive: boolean = false;

  constructor(options: ErrorHandlingServiceOptions = {}) {
    this.options = {
      onError: options.onError ?? (() => {}),
      onRecoverableError: options.onRecoverableError ?? (() => {}),
      onFatalError: options.onFatalError ?? (() => {}),
      enableRetry: options.enableRetry ?? true,
      maxRetries: options.maxRetries ?? 3,
      retryDelay: options.retryDelay ?? 1000,
      enableErrorReporting: options.enableErrorReporting ?? true,
      enableErrorRecovery: options.enableErrorRecovery ?? true,
      errorContext: options.errorContext ?? "ErrorHandlingService",
      ...options,
    };

    this.setupDefaultRecoveryStrategies();
  }

  /**
   * Start error handling service
   */
  start(): void {
    if (this.isActive) {
      console.log("[ErrorHandlingService] Already active");
      return;
    }

    this.isActive = true;
    console.log("[ErrorHandlingService] Started");
  }

  /**
   * Stop error handling service
   */
  stop(): void {
    if (!this.isActive) return;

    this.isActive = false;
    this.reconnectionService?.stop();
    console.log("[ErrorHandlingService] Stopped");
  }

  /**
   * Handle an error with context
   */
  handleError(error: Error, context: ErrorContext): void {
    if (!this.isActive) return;

    // Add to error history
    this.errorHistory.push({
      error,
      context,
      timestamp: Date.now(),
    });

    // Keep only last 100 errors
    if (this.errorHistory.length > 100) {
      this.errorHistory.shift();
    }

    // Log error
    this.logError(error, context);

    // Determine if error is recoverable
    const isRecoverable = this.isRecoverableError(error, context);

    if (isRecoverable && this.options.enableErrorRecovery) {
      this.handleRecoverableError(error, context);
    } else {
      this.handleFatalError(error, context);
    }
  }

  /**
   * Handle a recoverable error
   */
  private handleRecoverableError(error: Error, context: ErrorContext): void {
    console.log(
      `[ErrorHandlingService] Recoverable error in ${context.component}:`,
      error.message
    );

    this.options.onRecoverableError(error, context.component);
    this.options.onError(error, context.component);

    // Try to recover using registered strategies
    const strategy = this.recoveryStrategies.get(context.component);
    if (strategy && strategy.canRecover(error, context)) {
      this.attemptRecovery(error, context, strategy);
    } else {
      // Use default recovery mechanism
      this.attemptDefaultRecovery(error, context);
    }
  }

  /**
   * Handle a fatal error
   */
  private handleFatalError(error: Error, context: ErrorContext): void {
    console.error(
      `[ErrorHandlingService] Fatal error in ${context.component}:`,
      error
    );

    this.options.onFatalError(error, context.component);
    this.options.onError(error, context.component);
  }

  /**
   * Attempt error recovery using a specific strategy
   */
  private async attemptRecovery(
    error: Error,
    context: ErrorContext,
    strategy: ErrorRecoveryStrategy
  ): Promise<void> {
    try {
      console.log(
        `[ErrorHandlingService] Attempting recovery for ${context.component}`
      );
      await strategy.recover(error, context);
      console.log(
        `[ErrorHandlingService] Recovery successful for ${context.component}`
      );
    } catch (recoveryError) {
      console.error(
        `[ErrorHandlingService] Recovery failed for ${context.component}:`,
        recoveryError
      );
      this.handleFatalError(recoveryError as Error, context);
    }
  }

  /**
   * Attempt default recovery mechanism
   */
  private attemptDefaultRecovery(error: Error, context: ErrorContext): void {
    if (!this.options.enableRetry) return;

    // Use reconnection service for network-related errors
    if (this.isNetworkError(error)) {
      this.setupReconnectionService();
      this.reconnectionService?.startReconnection(async () => {
        // Default recovery action - this should be overridden by the caller
        console.log(
          `[ErrorHandlingService] Default recovery for ${context.component}`
        );
      }, context.component);
    } else {
      // For non-network errors, just retry after delay
      setTimeout(() => {
        console.log(
          `[ErrorHandlingService] Retrying ${context.component} after delay`
        );
      }, this.options.retryDelay);
    }
  }

  /**
   * Check if error is recoverable
   */
  private isRecoverableError(error: Error, _context: ErrorContext): boolean {
    // Network errors are usually recoverable
    if (this.isNetworkError(error)) return true;

    // WebRTC connection errors are recoverable
    if (this.isWebRTCError(error)) return true;

    // WebSocket connection errors are recoverable
    if (this.isWebSocketError(error)) return true;

    // Media source errors might be recoverable
    if (this.isMediaSourceError(error)) return true;

    // Canvas rendering errors are usually recoverable
    if (this.isCanvasError(error)) return true;

    return false;
  }

  /**
   * Check if error is network-related
   */
  private isNetworkError(error: Error): boolean {
    const networkErrorPatterns = [
      "Failed to fetch",
      "NetworkError",
      "Network request failed",
      "Connection refused",
      "Connection timeout",
      "DNS resolution failed",
      "No internet connection",
      "Network is unreachable",
    ];

    return networkErrorPatterns.some((pattern) =>
      error.message.toLowerCase().includes(pattern.toLowerCase())
    );
  }

  /**
   * Check if error is WebRTC-related
   */
  private isWebRTCError(error: Error): boolean {
    const webrtcErrorPatterns = [
      "WebRTC",
      "RTC",
      "ICE",
      "SDP",
      "peer connection",
      "signaling",
      "offer",
      "answer",
      "candidate",
    ];

    return webrtcErrorPatterns.some((pattern) =>
      error.message.toLowerCase().includes(pattern.toLowerCase())
    );
  }

  /**
   * Check if error is WebSocket-related
   */
  private isWebSocketError(error: Error): boolean {
    const websocketErrorPatterns = [
      "WebSocket",
      "ws",
      "connection closed",
      "connection failed",
      "connection timeout",
    ];

    return websocketErrorPatterns.some((pattern) =>
      error.message.toLowerCase().includes(pattern.toLowerCase())
    );
  }

  /**
   * Check if error is MediaSource-related
   */
  private isMediaSourceError(error: Error): boolean {
    const mediaSourceErrorPatterns = [
      "MediaSource",
      "SourceBuffer",
      "appendBuffer",
      "audio element",
      "video element",
    ];

    return mediaSourceErrorPatterns.some((pattern) =>
      error.message.toLowerCase().includes(pattern.toLowerCase())
    );
  }

  /**
   * Check if error is Canvas-related
   */
  private isCanvasError(error: Error): boolean {
    const canvasErrorPatterns = [
      "Canvas",
      "2D context",
      "drawImage",
      "getContext",
      "rendering",
    ];

    return canvasErrorPatterns.some((pattern) =>
      error.message.toLowerCase().includes(pattern.toLowerCase())
    );
  }

  /**
   * Log error with context
   */
  private logError(error: Error, context: ErrorContext): void {
    if (!this.options.enableErrorReporting) return;

    const errorInfo = {
      message: error.message,
      name: error.name,
      stack: error.stack,
      component: context.component,
      operation: context.operation,
      timestamp: new Date(context.timestamp).toISOString(),
      metadata: context.metadata,
    };

    console.error(
      `[ErrorHandlingService] Error in ${context.component}:`,
      errorInfo
    );
  }

  /**
   * Register a recovery strategy for a specific component
   */
  registerRecoveryStrategy(
    component: string,
    strategy: ErrorRecoveryStrategy
  ): void {
    this.recoveryStrategies.set(component, strategy);
    console.log(
      `[ErrorHandlingService] Registered recovery strategy for ${component}`
    );
  }

  /**
   * Unregister a recovery strategy
   */
  unregisterRecoveryStrategy(component: string): void {
    this.recoveryStrategies.delete(component);
    console.log(
      `[ErrorHandlingService] Unregistered recovery strategy for ${component}`
    );
  }

  /**
   * Setup reconnection service
   */
  private setupReconnectionService(): void {
    if (!this.reconnectionService) {
      this.reconnectionService = new ReconnectionService({
        maxAttempts: this.options.maxRetries,
        baseDelay: this.options.retryDelay,
        onReconnectSuccess: () => {
          console.log("[ErrorHandlingService] Reconnection successful");
        },
        onReconnectFailure: (error) => {
          console.error("[ErrorHandlingService] Reconnection failed:", error);
        },
        onMaxAttemptsReached: () => {
          console.error(
            "[ErrorHandlingService] Max reconnection attempts reached"
          );
        },
      });
    }
  }

  /**
   * Setup default recovery strategies
   */
  private setupDefaultRecoveryStrategies(): void {
    // WebRTC recovery strategy
    this.registerRecoveryStrategy("WebRTCClient", {
      canRecover: (error) =>
        this.isWebRTCError(error) || this.isNetworkError(error),
      recover: async (_error, _context) => {
        console.log(
          "[ErrorHandlingService] WebRTC recovery: restarting connection"
        );
        // This would be implemented by the WebRTCClient
      },
      maxRetries: 3,
      retryDelay: 2000,
    });

    // H264 recovery strategy
    this.registerRecoveryStrategy("H264Client", {
      canRecover: (error) =>
        this.isNetworkError(error) || this.isMediaSourceError(error),
      recover: async (_error, _context) => {
        console.log("[ErrorHandlingService] H264 recovery: restarting stream");
        // This would be implemented by the H264Client
      },
      maxRetries: 5,
      retryDelay: 1000,
    });

    // Video render recovery strategy
    this.registerRecoveryStrategy("VideoRenderService", {
      canRecover: (error) => this.isCanvasError(error),
      recover: async (_error, _context) => {
        console.log(
          "[ErrorHandlingService] Video render recovery: recreating canvas"
        );
        // This would be implemented by the VideoRenderService
      },
      maxRetries: 2,
      retryDelay: 500,
    });
  }

  /**
   * Get error history
   */
  getErrorHistory(): Array<{
    error: Error;
    context: ErrorContext;
    timestamp: number;
  }> {
    return [...this.errorHistory];
  }

  /**
   * Clear error history
   */
  clearErrorHistory(): void {
    this.errorHistory = [];
  }

  /**
   * Get error statistics
   */
  getErrorStatistics(): {
    totalErrors: number;
    recoverableErrors: number;
    fatalErrors: number;
    errorsByComponent: Record<string, number>;
    errorsByType: Record<string, number>;
  } {
    const stats = {
      totalErrors: this.errorHistory.length,
      recoverableErrors: 0,
      fatalErrors: 0,
      errorsByComponent: {} as Record<string, number>,
      errorsByType: {} as Record<string, number>,
    };

    this.errorHistory.forEach(({ error, context }) => {
      // Count by component
      stats.errorsByComponent[context.component] =
        (stats.errorsByComponent[context.component] || 0) + 1;

      // Count by error type
      stats.errorsByType[error.name] =
        (stats.errorsByType[error.name] || 0) + 1;

      // Count recoverable vs fatal
      if (this.isRecoverableError(error, context)) {
        stats.recoverableErrors++;
      } else {
        stats.fatalErrors++;
      }
    });

    return stats;
  }

  /**
   * Update service options
   */
  updateOptions(newOptions: Partial<ErrorHandlingServiceOptions>): void {
    this.options = { ...this.options, ...newOptions };
  }

  /**
   * Check if service is active
   */
  get active(): boolean {
    return this.isActive;
  }

  /**
   * Create error context
   */
  static createErrorContext(
    component: string,
    operation: string,
    metadata?: Record<string, unknown>
  ): ErrorContext {
    return {
      component,
      operation,
      timestamp: Date.now(),
      metadata,
    };
  }
}
