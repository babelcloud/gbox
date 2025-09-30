// Reconnection service for handling connection retries with exponential backoff
export interface ReconnectionOptions {
  maxAttempts?: number;
  baseDelay?: number;
  maxDelay?: number;
  onReconnectAttempt?: (attempt: number, maxAttempts: number) => void;
  onReconnectSuccess?: () => void;
  onReconnectFailure?: (error: Error) => void;
  onMaxAttemptsReached?: () => void;
}

export interface ReconnectionCallbacks {
  onReconnectAttempt?: (attempt: number, maxAttempts: number) => void;
  onReconnectSuccess?: () => void;
  onReconnectFailure?: (error: Error) => void;
  onMaxAttemptsReached?: () => void;
}

export class ReconnectionService {
  private attempts: number = 0;
  private timer: number | null = null;
  private isReconnecting: boolean = false;
  private options: Required<ReconnectionOptions>;

  constructor(options: ReconnectionOptions = {}) {
    this.options = {
      maxAttempts: options.maxAttempts ?? 5,
      baseDelay: options.baseDelay ?? 1000,
      maxDelay: options.maxDelay ?? 10000,
      onReconnectAttempt: options.onReconnectAttempt ?? (() => {}),
      onReconnectSuccess: options.onReconnectSuccess ?? (() => {}),
      onReconnectFailure: options.onReconnectFailure ?? (() => {}),
      onMaxAttemptsReached: options.onMaxAttemptsReached ?? (() => {}),
    };
  }

  /**
   * Start reconnection process
   * @param reconnectFn Function to execute for reconnection
   * @param context Optional context for logging
   */
  async startReconnection(
    reconnectFn: () => Promise<void>,
    context: string = "ReconnectionService"
  ): Promise<void> {
    if (this.isReconnecting) {
      console.log(`[${context}] Already reconnecting, skipping`);
      return;
    }

    this.isReconnecting = true;
    await this.attemptReconnection(reconnectFn, context);
  }

  /**
   * Attempt reconnection with exponential backoff
   */
  private async attemptReconnection(
    reconnectFn: () => Promise<void>,
    context: string
  ): Promise<void> {
    if (this.attempts >= this.options.maxAttempts) {
      console.log(`[${context}] Max reconnection attempts reached, giving up`);
      this.isReconnecting = false;
      this.options.onMaxAttemptsReached();
      return;
    }

    this.attempts++;
    const delay = this.calculateDelay();

    // Only log on first attempt or when reaching max attempts
    if (this.attempts === 1 || this.attempts === this.options.maxAttempts) {
      console.log(
        `[${context}] Scheduling reconnection in ${delay}ms (attempt ${this.attempts}/${this.options.maxAttempts})`
      );
    }

    this.options.onReconnectAttempt(this.attempts, this.options.maxAttempts);

    this.timer = window.setTimeout(async () => {
      try {
        await reconnectFn();
        
        // Success - reset state
        this.reset();
        this.options.onReconnectSuccess();
        console.log(`[${context}] Reconnection successful`);
      } catch (error) {
        // Only log errors, not every attempt
        if (this.attempts >= this.options.maxAttempts) {
          console.error(`[${context}] Reconnection failed after ${this.attempts} attempts:`, error);
        }
        this.options.onReconnectFailure(error as Error);
        
        // Schedule next attempt
        this.attemptReconnection(reconnectFn, context);
      }
    }, delay);
  }

  /**
   * Calculate delay using exponential backoff
   */
  private calculateDelay(): number {
    const exponentialDelay = this.options.baseDelay * Math.pow(2, this.attempts - 1);
    return Math.min(exponentialDelay, this.options.maxDelay);
  }

  /**
   * Reset reconnection state
   */
  reset(): void {
    this.attempts = 0;
    this.isReconnecting = false;
    this.clearTimer();
  }

  /**
   * Stop reconnection process
   */
  stop(): void {
    this.isReconnecting = false;
    this.clearTimer();
  }

  /**
   * Clear the current timer
   */
  private clearTimer(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  /**
   * Check if currently reconnecting
   */
  get isActive(): boolean {
    return this.isReconnecting;
  }

  /**
   * Get current attempt number
   */
  get currentAttempt(): number {
    return this.attempts;
  }

  /**
   * Get maximum attempts
   */
  get maxAttempts(): number {
    return this.options.maxAttempts;
  }

  /**
   * Get maximum delay
   */
  get maxDelay(): number {
    return this.options.maxDelay;
  }

  /**
   * Update reconnection options
   */
  updateOptions(newOptions: Partial<ReconnectionOptions>): void {
    this.options = { ...this.options, ...newOptions };
  }

  /**
   * Create a reconnection service with predefined configurations
   */
  static createForWebRTC(options: Partial<ReconnectionOptions> = {}): ReconnectionService {
    return new ReconnectionService({
      maxAttempts: 30,
      baseDelay: 3000,
      maxDelay: 10000,
      ...options,
    });
  }

  static createForH264(options: Partial<ReconnectionOptions> = {}): ReconnectionService {
    return new ReconnectionService({
      maxAttempts: 5,
      baseDelay: 1000,
      maxDelay: 10000,
      ...options,
    });
  }

  static createForWebSocket(options: Partial<ReconnectionOptions> = {}): ReconnectionService {
    return new ReconnectionService({
      maxAttempts: 5,
      baseDelay: 1000,
      maxDelay: 10000,
      ...options,
    });
  }
}
