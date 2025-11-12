// Base client abstract class for common functionality
import { ControlService } from "./services/control-service";
import { ReconnectionService } from "./services/reconnection-service";
import { StatsService } from "./services/stats-service";
import { VideoRenderService } from "./services/video-render-service";
import { ErrorHandlingService } from "./services/error-handling-service";
import {
  ControlClient,
  ClientOptions,
  ConnectionParams,
  ConnectionState,
  Stats,
  ErrorContext,
} from "./types";

export abstract class BaseClient implements ControlClient {
  // Common properties
  protected container: HTMLElement;
  protected isConnected: boolean = false;
  protected isReconnecting: boolean = false;
  protected currentDevice: string | null = null;
  protected lastConnectedDevice: string | null = null;
  protected connectionState: ConnectionState = "disconnected";

  // Services
  protected controlService!: ControlService;
  protected reconnectionService!: ReconnectionService;
  protected statsService!: StatsService;
  protected videoRenderService!: VideoRenderService;
  protected errorHandlingService!: ErrorHandlingService;

  // Options and callbacks
  protected options: Required<ClientOptions>;
  protected onConnectionStateChange?: (
    state: ConnectionState,
    message?: string
  ) => void;
  protected onError?: (error: Error) => void;
  protected onStatsUpdate?: (stats: Stats) => void;

  // Mouse dragging state
  public isMouseDragging: boolean = false;

  // Latency measurement properties
  protected pingTimes: number[] = [];
  protected pendingPings: Map<string, number> = new Map();
  protected pingInterval: number | null = null;

  constructor(container: HTMLElement, options: ClientOptions = {}) {
    this.container = container;
    this.options = {
      onConnectionStateChange: options.onConnectionStateChange ?? (() => {}),
      onError: options.onError ?? (() => {}),
      onStatsUpdate: options.onStatsUpdate ?? (() => {}),
      enableAudio: options.enableAudio ?? true,
      audioCodec: options.audioCodec ?? "opus",
    };

    this.onConnectionStateChange = this.options.onConnectionStateChange;
    this.onError = this.options.onError;
    this.onStatsUpdate = this.options.onStatsUpdate;

    // Initialize services
    this.initializeServices(container);
  }

  /**
   * Initialize all services
   */
  private initializeServices(container: HTMLElement): void {
    // Control service
    this.controlService = new ControlService();
    this.controlService.setClient(this);

    // Reconnection service
    this.reconnectionService = new ReconnectionService({
      onReconnectAttempt: (attempt, maxAttempts) => {
        this.onConnectionStateChange?.(
          "connecting",
          `Reconnecting... (${attempt}/${maxAttempts})`
        );
      },
      onReconnectSuccess: () => {
        this.onConnectionStateChange?.("connected", "Reconnected successfully");
      },
      onReconnectFailure: (error) => {
        this.onError?.(error);
      },
      onMaxAttemptsReached: () => {
        this.onConnectionStateChange?.(
          "error",
          "Max reconnection attempts reached"
        );
      },
    });

    // Stats service
    this.statsService = new StatsService({
      onStatsUpdate: (stats) => this.onStatsUpdate?.(stats),
      enableFPS: true,
      enableResolution: true,
      enableLatency: true,
    });

    // Video render service
    this.videoRenderService = new VideoRenderService({
      container,
      onStatsUpdate: (stats) => {
        this.statsService.updateResolution(
          parseInt(stats.resolution?.split("x")[0] || "0", 10),
          parseInt(stats.resolution?.split("x")[1] || "0", 10)
        );
        this.onStatsUpdate?.(stats);
      },
      onError: (error) => {
        this.handleError(error, "VideoRenderService", "render");
      },
    });

    // Error handling service
    this.errorHandlingService = new ErrorHandlingService({
      onError: (error, _context) => {
        this.onError?.(error);
      },
      onRecoverableError: (error, context) => {
        console.warn(
          `[${this.constructor.name}] Recoverable error in ${context}:`,
          error.message
        );
      },
      onFatalError: (error, context) => {
        console.error(
          `[${this.constructor.name}] Fatal error in ${context}:`,
          error
        );
        this.onError?.(error);
      },
    });

    // Register recovery strategies
    this.registerRecoveryStrategies();
  }

  /**
   * Register recovery strategies for this client
   */
  protected registerRecoveryStrategies(): void {
    // Override in subclasses to register specific recovery strategies
  }

  /**
   * Abstract method to establish connection - must be implemented by subclasses
   */
  protected abstract establishConnection(
    params: ConnectionParams
  ): Promise<void>;

  /**
   * Abstract method to cleanup connection - must be implemented by subclasses
   */
  protected abstract cleanupConnection(): Promise<void>;

  /**
   * Abstract method to check if control is connected - must be implemented by subclasses
   */
  protected abstract isControlConnectedInternal(): boolean;

  /**
   * Connect to device
   */
  async connect(
    deviceSerial: string,
    apiUrl: string,
    wsUrl?: string
  ): Promise<void> {
    console.log(
      `[${this.constructor.name}] Connecting to device: ${deviceSerial}`
    );

    // Always disconnect first to ensure clean state
    if (this.isConnected) {
      console.log(`[${this.constructor.name}] Cleaning up existing connection`);
      await this.disconnect();
      // Wait for cleanup to complete
      await new Promise((resolve) => setTimeout(resolve, 500));
    }

    this.currentDevice = deviceSerial;
    this.lastConnectedDevice = deviceSerial;
    this.isReconnecting = false;
    this.connectionState = "connecting";
    this.onConnectionStateChange?.("connecting", "Connecting to device...");

    try {
      // Start services
      this.startServices();

      // Establish connection
      await this.establishConnection({ deviceSerial, apiUrl, wsUrl });

      // Update state
      this.isConnected = true;
      this.connectionState = "connected";
      this.onConnectionStateChange?.("connected", "Connected successfully");
    } catch (error) {
      console.error(`[${this.constructor.name}] Connection failed:`, error);
      this.handleError(error as Error, this.constructor.name, "connect");
      this.connectionState = "error";
      this.onConnectionStateChange?.("error", "Connection failed");
      throw error;
    }
  }

  /**
   * Disconnect from device
   */
  async disconnect(): Promise<void> {
    console.log(`[${this.constructor.name}] Disconnecting`);

    // Stop services
    this.stopServices();

    // Cleanup connection
    try {
      await this.cleanupConnection();
    } catch (error) {
      console.warn(`[${this.constructor.name}] Error during cleanup:`, error);
    }

    // Update state
    this.isConnected = false;
    this.isReconnecting = false;
    this.connectionState = "disconnected";
    this.currentDevice = null;
    this.isMouseDragging = false;

    this.onConnectionStateChange?.("disconnected", "Disconnected");
  }

  /**
   * Start all services
   */
  protected startServices(): void {
    this.statsService.start();
    this.videoRenderService.start();
    this.errorHandlingService.start();
  }

  /**
   * Stop all services
   */
  protected stopServices(): void {
    this.statsService.stop();
    this.videoRenderService.stop();
    this.errorHandlingService.stop();
    this.reconnectionService.stop();
  }

  /**
   * Start reconnection process
   */
  protected startReconnection(): void {
    if (this.isReconnecting || !this.currentDevice) return;

    this.isReconnecting = true;
    this.connectionState = "connecting";
    this.onConnectionStateChange?.("connecting", "Reconnecting...");

    this.reconnectionService.startReconnection(async () => {
      if (this.lastConnectedDevice) {
        await this.connect(
          this.lastConnectedDevice,
          this.getLastApiUrl(),
          this.getLastWsUrl()
        );
      }
    }, this.constructor.name);
  }

  /**
   * Stop reconnection process
   */
  protected stopReconnection(): void {
    this.reconnectionService.stop();
    this.isReconnecting = false;
  }

  /**
   * Handle error with context
   */
  protected handleError(
    error: Error,
    component: string,
    operation: string,
    metadata?: Record<string, unknown>
  ): void {
    const context: ErrorContext = {
      component,
      operation,
      timestamp: Date.now(),
      metadata,
    };

    this.errorHandlingService.handleError(error, context);
  }

  /**
   * Update connection state
   */
  protected updateConnectionState(
    state: ConnectionState,
    message?: string
  ): void {
    this.connectionState = state;
    this.onConnectionStateChange?.(state, message);
  }

  /**
   * Check if control is connected
   */
  isControlConnected(): boolean {
    return this.isConnected && this.isControlConnectedInternal();
  }

  // ControlClient interface implementation
  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    _metaState?: number
  ): void {
    // This is a simplified implementation - subclasses should override this
    console.log(`[${this.constructor.name}] Key event: ${keycode} ${action}`);
  }

  sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    _pressure?: number
  ): void {
    // This will be implemented by subclasses
    console.log(
      `[${this.constructor.name}] Touch event: ${action} at (${x}, ${y})`
    );
  }

  sendControlAction(action: string, _params?: Record<string, unknown>): void {
    this.controlService.handleControlAction(action);
  }

  sendClipboardSet(_text: string, _paste?: boolean): void {
    this.controlService.handleClipboardPaste();
  }

  requestKeyframe(): void {
    // This will be implemented by subclasses
    console.log(`[${this.constructor.name}] Requesting keyframe`);
  }

  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void {
    // This is a simplified implementation - subclasses should override this
    console.log(
      `[${this.constructor.name}] Mouse event: ${action} at (${event.clientX}, ${event.clientY})`
    );
  }

  handleTouchEvent(_event: TouchEvent, action: "down" | "up" | "move"): void {
    // This is a simplified implementation - subclasses should override this
    console.log(`[${this.constructor.name}] Touch event: ${action}`);
  }

  // Abstract methods that subclasses must implement
  protected abstract getLastApiUrl(): string;
  protected abstract getLastWsUrl(): string | undefined;

  // Getters for common properties
  get connected(): boolean {
    return this.isConnected;
  }

  get state(): ConnectionState {
    return this.connectionState;
  }

  get device(): string | null {
    return this.currentDevice;
  }

  // Service getters
  get control(): ControlService {
    return this.controlService;
  }

  get stats(): StatsService {
    return this.statsService;
  }

  get videoRender(): VideoRenderService {
    return this.videoRenderService;
  }

  get errorHandling(): ErrorHandlingService {
    return this.errorHandlingService;
  }

  /**
   * Start ping measurement for latency
   */
  protected startPingMeasurement(): void {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
    }

    this.pingTimes = [];
    this.pendingPings = new Map();

    // Measure ping every 2 seconds
    this.pingInterval = window.setInterval(() => {
      this.measurePing();
    }, 2000);
  }

  /**
   * Stop ping measurement
   */
  protected stopPingMeasurement(): void {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
    this.pendingPings.clear();
  }

  /**
   * Measure ping latency - abstract method to be implemented by subclasses
   */
  protected abstract measurePing(): void;

  /**
   * Handle ping response - common logic for both clients
   */
  protected handlePingResponse(message: {
    type: string | number;
    id?: string;
  }): void {
    if (
      message.type === "pong" &&
      message.id &&
      this.pendingPings.has(message.id)
    ) {
      const pingStart = this.pendingPings.get(message.id);
      if (pingStart) {
        const latency = performance.now() - pingStart;

        // Store ping time for averaging
        this.pingTimes.push(latency);

        // Keep only last 5 ping times
        if (this.pingTimes.length > 5) {
          this.pingTimes.shift();
        }

        // Update latency in stats service
        this.statsService.recordPingTime(latency);

        this.pendingPings.delete(message.id);
      }
    }
  }

  /**
   * Get average latency from ping measurements
   */
  protected getAverageLatency(): number {
    if (this.pingTimes.length === 0) return 0;
    return Math.round(
      this.pingTimes.reduce((a, b) => a + b, 0) / this.pingTimes.length
    );
  }

  /**
   * Convert mouse/touch coordinates to normalized coordinates (0-1)
   */
  protected normalizeCoordinates(
    clientX: number,
    clientY: number,
    targetElement: HTMLElement
  ): { x: number; y: number } {
    const rect = targetElement.getBoundingClientRect();
    const x = (clientX - rect.left) / rect.width;
    const y = (clientY - rect.top) / rect.height;

    // Ensure coordinates are within valid range
    return {
      x: Math.max(0, Math.min(1, x)),
      y: Math.max(0, Math.min(1, y)),
    };
  }

  /**
   * Build WebSocket URL from various input formats
   */
  protected buildWebSocketUrl(
    baseUrl: string,
    _deviceSerial: string,
    path: string = "/control"
  ): string {
    // If it's already a WebSocket URL, just replace the path
    if (baseUrl.startsWith("ws://") || baseUrl.startsWith("wss://")) {
      const cleanUrl = baseUrl.replace(/\/ws$/, ""); // Remove /ws suffix if present
      return `${cleanUrl}${path}`;
    }

    // Convert HTTP to WebSocket protocol
    if (baseUrl.startsWith("http://")) {
      return baseUrl.replace("http://", "ws://") + path;
    }

    if (baseUrl.startsWith("https://")) {
      return baseUrl.replace("https://", "wss://") + path;
    }

    // Relative path, use current host with appropriate protocol
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}${baseUrl}${path}`;
  }

  /**
   * Build control WebSocket URL for device
   */
  public buildControlWebSocketUrl(
    baseUrl: string,
    deviceSerial: string,
    customPath?: string
  ): string {
    let path: string;

    if (customPath) {
      // 使用自定义路径，替换 {serial} 占位符
      path = customPath.replace("{serial}", deviceSerial);
    } else {
      // 使用统一的默认路径 - 两个客户端都使用相同的路径
      path = `/api/devices/${deviceSerial}/control`;
    }

    return this.buildWebSocketUrl(baseUrl, deviceSerial, path);
  }

  /**
   * Build control WebSocket URL from ConnectionParams
   */
  public buildControlWebSocketUrlFromParams(params: ConnectionParams): string {
    const baseUrl = params.wsUrl || params.apiUrl;
    return this.buildControlWebSocketUrl(
      baseUrl,
      params.deviceSerial,
      params.controlPath
    );
  }

  // Cleanup method
  destroy(): void {
    this.stopPingMeasurement();
    this.disconnect();
    this.videoRenderService.destroy();
  }
}
