// Refactored H264Client extending BaseClient
import { BaseClient } from "./base-client";
import { ControlMessage, ConnectionParams, ClientOptions } from "./types";
import { VideoRenderService } from "./services/video-render-service";

// NAL Unit types
const NALU = {
  SPS: 7, // Sequence Parameter Set
  PPS: 8, // Picture Parameter Set
  IDR: 5, // IDR frame
} as const;

// MSEAudioProcessor - based on successful MSE+ReadableStream approach
class MSEAudioProcessor {
  private mediaSource: MediaSource | null = null;
  private sourceBuffer: SourceBuffer | null = null;
  private audioElement: HTMLAudioElement | null = null;
  private audioElementError: boolean = false;
  public isStreaming: boolean = false;
  private reader: ReadableStreamDefaultReader<Uint8Array> | null = null;
  private abortController: AbortController | null = null;
  private healthCheckInterval: number | null = null;
  private reconnectAttempts: number = 0;
  private maxReconnectAttempts: number = 3; // Reduced from 5 to 3
  private reconnectDelay: number = 5000; // Increased from 1 second to 5 seconds
  private reconnectTimer: number | null = null;
  private currentAudioUrl: string | null = null;
  private stats = {
    bytesReceived: 0,
    chunksProcessed: 0,
    bufferedSeconds: 0,
    startTime: 0,
    lastChunkTime: 0,
  };

  constructor(private container: HTMLElement) {
    // Keep constructor simple, actual initialization in connect method
  }

  // Based on successful MSE approach
  async connect(audioUrl: string): Promise<void> {
    console.log("[MSEAudio] Connecting to:", audioUrl);
    this.currentAudioUrl = audioUrl;

    // Check MSE support
    if (
      !window.MediaSource ||
      !MediaSource.isTypeSupported('audio/webm; codecs="opus"')
    ) {
      throw new Error("Browser does not support WebM/Opus MSE");
    }

    // Add connection timeout
    const connectionTimeout = new Promise<never>((_, reject) => {
      setTimeout(() => {
        reject(new Error("Audio connection timeout after 10 seconds"));
      }, 10000);
    });

    // Reset state
    this.isStreaming = true;
    this.stats.startTime = Date.now();
    this.stats.bytesReceived = 0;
    this.stats.chunksProcessed = 0;
    this.stats.lastChunkTime = Date.now();

    // Create audio element
    this.audioElement = document.createElement("audio");
    this.audioElement.controls = false; // Hide controls, controlled by video player
    this.audioElement.style.display = "none";
    this.audioElement.autoplay = true; // Enable autoplay
    this.audioElement.muted = false; // Ensure not muted
    this.container.appendChild(this.audioElement);

    // Add audio element error handling with detailed logging
    this.audioElement.addEventListener("error", (e) => {
      const errorDetails = {
        error: this.audioElement?.error,
        networkState: this.audioElement?.networkState,
        readyState: this.audioElement?.readyState,
        currentTime: this.audioElement?.currentTime,
        duration: this.audioElement?.duration,
        paused: this.audioElement?.paused,
        muted: this.audioElement?.muted,
        timestamp: new Date().toISOString(),
      };

      console.error("[MSEAudio] ❌ Audio element error:", {
        event: e,
        details: errorDetails,
        errorCode: this.audioElement?.error?.code,
        errorMessage: this.audioElement?.error?.message,
      });

      // Mark audio element as having error, recreate later (like the old implementation)
      this.audioElementError = true;
      // Stop streaming to prevent further errors
      this.isStreaming = false;
    });

    // Create MediaSource
    this.mediaSource = new MediaSource();
    this.audioElement.src = URL.createObjectURL(this.mediaSource);

    // Wait for MediaSource to be ready
    await new Promise<void>((resolve, reject) => {
      if (!this.mediaSource) {
        reject(new Error("MediaSource not created"));
        return;
      }

      this.mediaSource.addEventListener("sourceopen", () => {
        console.log("[MSEAudio] MediaSource opened");
        resolve();
      });

      this.mediaSource.addEventListener("sourceerror", (e) => {
        console.error("[MSEAudio] MediaSource error:", e);
        reject(new Error("MediaSource error"));
      });

      // Timeout after 5 seconds
      setTimeout(() => {
        reject(new Error("MediaSource open timeout"));
      }, 5000);
    });

    // Create source buffer
    if (!this.mediaSource) {
      throw new Error("MediaSource not available");
    }

    this.sourceBuffer = this.mediaSource.addSourceBuffer(
      'audio/webm; codecs="opus"'
    );

    // SourceBuffer event listeners (like the old implementation)
    this.sourceBuffer.addEventListener("updateend", () => {
      // Try to play when we have enough data
      if (
        this.audioElement &&
        this.audioElement.readyState >= 3 &&
        this.audioElement.paused
      ) {
        this.audioElement
          .play()
          .then(() => {
            console.log("[MSEAudio] Audio started playing");
          })
          .catch((e) => {
            console.warn("[MSEAudio] Playback failed:", e.message);
          });
      }
    });

    this.sourceBuffer.addEventListener("error", (e) => {
      console.error("[MSEAudio] SourceBuffer error:", e);
      this.isStreaming = false;
    });

    // Start streaming with timeout
    try {
      await Promise.race([this.startStreaming(audioUrl), connectionTimeout]);
    } catch (error) {
      console.error("[MSEAudio] Connection failed:", error);
      this.isStreaming = false;
      throw error;
    }
  }

  private async startStreaming(audioUrl: string): Promise<void> {
    try {
      console.log("[MSEAudio] Starting audio stream...");
      const response = await fetch(audioUrl);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      this.reader = response.body?.getReader() || null;
      if (!this.reader) {
        throw new Error("No response body reader available");
      }

      this.abortController = new AbortController();
      this.processAudioStream();
      this.startHealthCheck();
    } catch (error) {
      console.error("[MSEAudio] Failed to start streaming:", error);
      this.handleReconnection();
    }
  }

  private async processAudioStream(): Promise<void> {
    if (!this.reader || !this.sourceBuffer) return;

    try {
      while (this.isStreaming && !this.abortController?.signal.aborted) {
        const { done, value } = await this.reader.read();
        if (done) break;

        this.stats.bytesReceived += value.length;
        this.stats.chunksProcessed++;
        this.stats.lastChunkTime = Date.now();

        // Log first few chunks to verify data is being received
        if (this.stats.chunksProcessed <= 5) {
          console.log(
            `[MSEAudio] Received chunk ${this.stats.chunksProcessed}, size: ${value.length} bytes`
          );
        }

        // Check if audio element has error before processing
        if (this.audioElement?.error) {
          console.warn(
            "[MSEAudio] Audio element has error, stopping processing:",
            this.audioElement.error
          );
          break;
        }

        // Check audio element status before processing (like the old implementation)
        if (
          this.audioElementError ||
          (this.audioElement && this.audioElement.error)
        ) {
          console.warn("[MSEAudio] Audio element has error, skipping chunk");
          continue;
        }

        // Add to source buffer with proper checks (like the old implementation)
        if (
          this.sourceBuffer &&
          !this.sourceBuffer.updating &&
          this.mediaSource &&
          this.mediaSource.readyState === "open" &&
          this.audioElement &&
          !this.audioElementError &&
          !this.audioElement.error
        ) {
          try {
            // Convert Uint8Array to ArrayBuffer like the old implementation
            const arrayBuffer = value.buffer.slice(
              value.byteOffset,
              value.byteOffset + value.byteLength
            ) as ArrayBuffer;
            this.sourceBuffer.appendBuffer(arrayBuffer);
          } catch (error) {
            console.warn("[MSEAudio] Failed to append buffer:", error);
            // Continue processing other chunks
          }
        } else {
          // Wait for source buffer to be ready
          if (this.sourceBuffer && this.sourceBuffer.updating) {
            await new Promise((resolve) => {
              this.sourceBuffer!.addEventListener("updateend", resolve, {
                once: true,
              });
            });
          }
        }

        // Update buffered time
        if (this.audioElement) {
          const buffered = this.audioElement.buffered;
          if (buffered.length > 0) {
            this.stats.bufferedSeconds = buffered.end(buffered.length - 1);
          }

          // Log audio element state for debugging
          if (this.stats.chunksProcessed <= 5) {
            console.log(`[MSEAudio] Audio element state:`, {
              readyState: this.audioElement.readyState,
              networkState: this.audioElement.networkState,
              paused: this.audioElement.paused,
              muted: this.audioElement.muted,
              currentTime: this.audioElement.currentTime,
              duration: this.audioElement.duration,
              buffered:
                buffered.length > 0
                  ? `${buffered.start(0).toFixed(2)}s - ${buffered
                      .end(buffered.length - 1)
                      .toFixed(2)}s`
                  : "none",
            });
          }
        }
      }
    } catch (error) {
      console.error("[MSEAudio] Stream processing error:", error);
      this.handleReconnection();
    }
  }

  private startHealthCheck(): void {
    this.healthCheckInterval = window.setInterval(() => {
      if (!this.isStreaming) return;

      const healthStatus = this.checkConnectionHealth();

      if (!healthStatus.isHealthy) {
        console.warn("[MSEAudio] ⚠️ Connection health check failed:", {
          reason: healthStatus.reason,
          details: healthStatus.details,
          timestamp: new Date().toISOString(),
        });

        // Handle specific health issues
        if (healthStatus.reason === "no_data_received") {
          this.handleReconnection();
        }
      }
    }, 2000); // Check every 2 seconds like the old implementation
  }

  private checkConnectionHealth(): {
    isHealthy: boolean;
    reason?: string;
    details?: any;
  } {
    if (!this.audioElement || !this.mediaSource || !this.sourceBuffer) {
      return { isHealthy: false, reason: "missing_components" };
    }

    if (this.audioElementError || this.audioElement.error) {
      return {
        isHealthy: false,
        reason: "audio_element_error",
        details: {
          error: this.audioElement.error,
          audioElementError: this.audioElementError,
          networkState: this.audioElement.networkState,
          readyState: this.audioElement.readyState,
        },
      };
    }

    if (this.mediaSource.readyState !== "open") {
      return {
        isHealthy: false,
        reason: "media_source_not_open",
        details: { readyState: this.mediaSource.readyState },
      };
    }

    if (this.audioElement.networkState === 3) {
      // NETWORK_NO_SOURCE
      return {
        isHealthy: false,
        reason: "network_no_source",
        details: { networkState: this.audioElement.networkState },
      };
    }

    // More lenient data check - only trigger reconnection for actual errors
    const timeSinceStart = Date.now() - this.stats.startTime;
    const timeSinceLastChunk = Date.now() - this.stats.lastChunkTime;

    // Only consider it unhealthy if:
    // 1. We've been running for more than 30 seconds AND
    // 2. We haven't received any data at all AND
    // 3. We haven't received data for more than 20 seconds
    if (
      timeSinceStart > 30000 &&
      this.stats.bytesReceived === 0 &&
      timeSinceLastChunk > 20000
    ) {
      return {
        isHealthy: false,
        reason: "no_data_received",
        details: {
          timeSinceStart,
          timeSinceLastChunk,
          bytesReceived: this.stats.bytesReceived,
        },
      };
    }

    // If we have received some data, be more lenient about gaps
    if (this.stats.bytesReceived > 0 && timeSinceLastChunk > 30000) {
      // 30 seconds without data after we've received some
      return {
        isHealthy: false,
        reason: "data_starvation",
        details: {
          timeSinceLastChunk,
          bytesReceived: this.stats.bytesReceived,
        },
      };
    }

    return { isHealthy: true };
  }

  private handleReconnection(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error(
        "[MSEAudio] Max reconnection attempts reached, giving up audio connection"
      );
      this.isStreaming = false;
      return;
    }

    this.reconnectAttempts++;

    // Exponential backoff with jitter
    const baseDelay =
      this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
    const jitter = Math.random() * 1000; // Add up to 1 second of jitter
    const delay = Math.min(baseDelay + jitter, 30000); // Cap at 30 seconds

    console.log(
      `[MSEAudio] Scheduling reconnection in ${Math.round(delay)}ms (attempt ${
        this.reconnectAttempts
      }/${this.maxReconnectAttempts})`
    );

    this.reconnectTimer = window.setTimeout(() => {
      if (this.currentAudioUrl && this.isStreaming) {
        console.log(
          `[MSEAudio] Attempting reconnection (attempt ${this.reconnectAttempts})`
        );
        this.connect(this.currentAudioUrl).catch((error) => {
          console.error("[MSEAudio] Reconnection failed:", error);
          // Don't immediately retry, let the health check handle it
        });
      }
    }, delay);
  }

  // Get audio stats
  getStats() {
    return { ...this.stats };
  }

  // Pause audio
  pause(): void {
    if (this.audioElement && !this.audioElement.paused) {
      this.audioElement.pause();
    }
  }

  // Resume audio
  resume(): void {
    if (this.audioElement && this.audioElement.paused) {
      this.audioElement.play().catch(console.error);
    }
  }

  // Stop audio
  stop(): void {
    this.isStreaming = false;

    if (this.healthCheckInterval) {
      clearInterval(this.healthCheckInterval);
      this.healthCheckInterval = null;
    }

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    if (this.reader) {
      this.reader.cancel();
      this.reader = null;
    }

    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.remove();
      this.audioElement = null;
    }

    if (this.mediaSource) {
      if (this.mediaSource.readyState === "open") {
        this.mediaSource.endOfStream();
      }
      this.mediaSource = null;
    }

    this.sourceBuffer = null;
  }
}

export class H264ClientRefactored extends BaseClient {
  private decoder: VideoDecoder | null = null;
  private abortController: AbortController | null = null;
  private audioProcessor: MSEAudioProcessor | null = null;
  private controlWs: WebSocket | null = null;
  private buffer: Uint8Array = new Uint8Array(0);
  private spsData: Uint8Array | null = null;
  private ppsData: Uint8Array | null = null;
  private animationFrameId: number | undefined;
  private decodedFrames: Array<{ frame: VideoFrame; timestamp: number }> = [];
  private waitingForKeyframe: boolean = true;
  private keyframeRequestTimer: number | null = null;
  private controlRetryCount: number = 0;
  // private controlReconnectTimer: number | null = null; // Removed unused variable
  private maxControlRetries: number = 5;
  private lastCanvasDimensions: { width: number; height: number } | null = null;

  // Latency measurement - now inherited from BaseClient
  // private resizeTimeout: number | null = null; // Removed unused variable
  private resizeObserver: ResizeObserver | null = null;
  // private orientationChangeHandler: (() => void) | null = null; // Removed unused variable
  // private orientationCheckInterval: number | null = null; // Removed unused variable
  // private lastOrientation: string | null = null; // Removed unused variable
  // private lastConnectionStatus: boolean = false; // Removed unused variable

  // Connection parameters for reconnection
  private lastApiUrl: string = "";
  private lastWsUrl: string | undefined;

  constructor(container: HTMLElement, options: ClientOptions = {}) {
    super(container, options);
    this.initializeWebCodecs();
  }

  /**
   * Set the canvas container for H264 rendering
   */
  setCanvasContainer(container: HTMLElement): void {
    // Reinitialize video render service with the correct container
    this.videoRenderService = new VideoRenderService({
      container,
      statsService: this.statsService, // Use the same StatsService instance
      onStatsUpdate: (_stats: { resolution?: string; fps?: number }) => {
        // VideoRenderService updates the shared StatsService, get complete stats
        // Get complete stats from StatsService instead of just forwarding partial stats
        const completeStats = this.statsService.getCurrentStats();
        this.onStatsUpdate?.(completeStats);
      },
      onError: (error: Error) => {
        this.handleError(error, "VideoRenderService", "render");
      },
    });
  }

  /**
   * Initialize WebCodecs decoder
   */
  private initializeWebCodecs(): void {
    // console.log("[H264Client] Initializing WebCodecs decoder...");

    // Check if WebCodecs is supported
    if (typeof VideoDecoder !== "function") {
      console.error("[H264Client] WebCodecs not supported");
      this.handleError(
        new Error("WebCodecs not supported"),
        "H264Client",
        "initialize"
      );
      return;
    }

    try {
      // Create VideoDecoder
      this.decoder = new VideoDecoder({
        output: (frame) => this.onFrameDecoded(frame),
        error: (error) => {
          console.error("[H264Client] VideoDecoder error:", error);
          // Don't call handleError for decoder errors, just log them
          // This prevents the decoder from being closed on configuration errors

          // If decoder fails, try to recreate it
          console.log(
            "[H264Client] VideoDecoder failed, attempting to recreate..."
          );
          this.recreateDecoder();
        },
      });
    } catch (error) {
      console.error("[H264Client] Failed to create VideoDecoder:", error);
      this.handleError(error as Error, "H264Client", "initialize");
    }
  }

  /**
   * Establish H264 connection
   */
  protected async establishConnection(params: ConnectionParams): Promise<void> {
    const { deviceSerial, apiUrl, wsUrl } = params;
    this.lastApiUrl = apiUrl;
    this.lastWsUrl = wsUrl;

    console.log(
      `[H264Client] Establishing H264 connection to device: ${deviceSerial}`
    );

    // Ensure decoder is initialized before starting video stream
    if (!this.decoder) {
      console.log("[H264Client] Decoder not ready, initializing WebCodecs...");
      this.initializeWebCodecs();
    }

    if (!this.decoder) {
      throw new Error("Failed to initialize H264 decoder");
    }

    // Start services
    this.startServices();

    // Start HTTP video stream
    const videoUrl = `${apiUrl}/devices/${deviceSerial}/video?mode=h264&format=avc`;
    await this.startHTTP(videoUrl);

    // Connect control WebSocket
    if (wsUrl) {
      await this.connectControl(deviceSerial, apiUrl, wsUrl);
    }

    // Connect audio if enabled
    if (this.options.enableAudio) {
      try {
        const audioUrl = `${apiUrl}/devices/${deviceSerial}/audio?codec=opus&format=webm&mse=true`;
        await this.connectAudio(audioUrl);
      } catch (error) {
        console.warn(
          "[H264Client] Audio connection failed, continuing without audio:",
          error
        );
        // Don't throw error, audio is optional
        // But we should notify the user that audio is not available
        this.onConnectionStateChange?.(
          "connected",
          "H.264 stream connected (audio unavailable)"
        );
      }
    }

    // Start keyframe requests
    this.requestKeyframe();

    // Start latency measurement
    this.startPingMeasurement();

    // Start container resize observer
    this.startResizeObserver();
  }

  /**
   * Cleanup H264 connection
   */
  protected async cleanupConnection(): Promise<void> {
    console.log("[H264Client] Cleaning up H264 connection");

    // Stop services first
    this.stopServices();

    // Stop keyframe requests
    if (this.keyframeRequestTimer) {
      clearInterval(this.keyframeRequestTimer);
      this.keyframeRequestTimer = null;
    }

    // Stop animation frame
    if (this.animationFrameId) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = undefined;
    }

    // Stop resize observer
    this.stopResizeObserver();

    // Stop audio processor
    if (this.audioProcessor) {
      this.audioProcessor.stop();
      this.audioProcessor = null;
    }

    // Close control WebSocket
    if (this.controlWs) {
      this.controlWs.close();
      this.controlWs = null;
    }

    // Abort HTTP stream
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    // Close decoder
    if (this.decoder) {
      this.decoder.close();
      this.decoder = null;
    }

    // Clear frames
    this.decodedFrames.forEach(({ frame }) => frame.close());
    this.decodedFrames = [];

    // Reset video-related state
    this.buffer = new Uint8Array(0);
    this.spsData = null;
    this.ppsData = null;
    this.waitingForKeyframe = true;
    this.lastCanvasDimensions = null;

    // Clear and reset canvas
    const canvas = this.getCanvas();
    if (canvas) {
      const context = canvas.getContext("2d");
      if (context) {
        // Clear the canvas content
        context.clearRect(0, 0, canvas.width, canvas.height);
        // Reset canvas to default size
        canvas.width = 640;
        canvas.height = 480;
        // Reset canvas styling
        canvas.style.width = "100%";
        canvas.style.height = "100%";
        canvas.style.objectFit = "contain";
        canvas.style.display = "block";
        canvas.style.margin = "auto";
        canvas.style.background = "black";
      }
    }
  }

  /**
   * Check if control is connected
   */
  protected isControlConnectedInternal(): boolean {
    return this.controlWs?.readyState === WebSocket.OPEN;
  }

  /**
   * Get last API URL
   */
  protected getLastApiUrl(): string {
    return this.lastApiUrl;
  }

  /**
   * Get last WebSocket URL
   */
  protected getLastWsUrl(): string | undefined {
    return this.lastWsUrl;
  }

  /**
   * Register recovery strategies for H264
   */
  protected registerRecoveryStrategies(): void {
    this.errorHandling.registerRecoveryStrategy("H264Client", {
      canRecover: (error: Error, _context) => {
        return (
          error.message.includes("stream") ||
          error.message.includes("audio") ||
          error.message.includes("connection")
        );
      },
      recover: async (error: Error, context) => {
        console.log(
          "[H264Client] Attempting recovery...",
          error.message,
          context.component
        );
        await this.cleanupConnection();
        if (this.currentDevice && this.lastApiUrl) {
          await this.establishConnection({
            deviceSerial: this.currentDevice,
            apiUrl: this.lastApiUrl,
            wsUrl: this.lastWsUrl,
          });
        }
      },
      maxRetries: 5,
      retryDelay: 1000,
    });
  }

  /**
   * Start HTTP video stream
   */
  private async startHTTP(url: string): Promise<void> {
    console.log("[H264Client] Starting HTTP stream:", url);

    this.abortController = new AbortController();
    console.log("[H264Client] Making HTTP request to video stream...");
    const response = await fetch(url, {
      signal: this.abortController.signal,
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body reader available");
    }
    // Process stream (non-blocking)
    this.processVideoStream(reader).catch((error) => {
      console.error("[H264Client] Video stream processing error:", error);
      this.handleError(error as Error, "H264Client", "stream");
    });
  }

  /**
   * Process video stream
   */
  private async processVideoStream(
    reader: ReadableStreamDefaultReader<Uint8Array>
  ): Promise<void> {
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }

        // Append to buffer
        const newBuffer = new Uint8Array(this.buffer.length + value.length);
        newBuffer.set(this.buffer);
        newBuffer.set(value, this.buffer.length);
        this.buffer = newBuffer;

        // Process NAL units from AVC format stream
        const { processedNals, remainingBuffer } = this.parseAVC(this.buffer);
        this.buffer = remainingBuffer;

        // Process each NAL unit
        for (const nalData of processedNals) {
          this.processNALUnit(nalData);
        }
      }
    } catch (error) {
      if ((error as Error).name !== "AbortError") {
        console.error("[H264Client] Stream processing error:", error);
        this.handleError(error as Error, "H264Client", "stream");
      }
    }
  }

  /**
   * Process NAL units from buffer
   */
  /**
   * Parse AVC format NAL units (length-prefixed)
   */
  private parseAVC(data: Uint8Array): {
    processedNals: Uint8Array[];
    remainingBuffer: Uint8Array;
  } {
    const processedNals: Uint8Array[] = [];
    let offset = 0;
    let nalCount = 0;

    while (offset < data.length) {
      // Need at least 4 bytes for length prefix
      if (offset + 4 > data.length) {
        break;
      }

      // Read length prefix (big-endian)
      const length =
        (data[offset] << 24) |
        (data[offset + 1] << 16) |
        (data[offset + 2] << 8) |
        data[offset + 3];

      offset += 4;

      // Check if we have enough data for the NAL unit
      if (offset + length > data.length) {
        // Not enough data, put back the length prefix
        offset -= 4;
        break;
      }

      // Extract NAL unit
      const nalData = data.slice(offset, offset + length);
      processedNals.push(nalData);
      offset += length;

      nalCount++;
      const nalType = nalData[0] & 0x1f;
      // Only log very important events (SPS/PPS) and first one
      if (nalCount === 1 && (nalType === NALU.SPS || nalType === NALU.PPS)) {
        console.log(
          `[H264Client] Found AVC NAL unit ${nalCount}, size: ${nalData.length}, type: ${nalType}`
        );
      }
    }

    // Return remaining buffer
    const remainingBuffer = data.slice(offset);
    return { processedNals, remainingBuffer };
  }

  /**
   * Process individual NAL unit
   */
  private processNALUnit(nalUnit: Uint8Array): void {
    if (nalUnit.length === 0) return;

    const nalType = nalUnit[0] & 0x1f;

    switch (nalType) {
      case NALU.SPS:
        this.spsData = nalUnit;
        this.tryConfigureDecoder();
        break;
      case NALU.PPS:
        this.ppsData = nalUnit;
        this.tryConfigureDecoder();
        break;
      case NALU.IDR:
        if (this.spsData && this.ppsData) {
          // 收到关键帧，停止等待
          this.waitingForKeyframe = false;
          this.decodeFrame(nalUnit);
        } else {
          console.warn("[H264Client] Received IDR but missing SPS/PPS data");
        }
        break;
      default:
        if (!this.waitingForKeyframe) {
          this.decodeFrame(nalUnit);
        }
        break;
    }
  }

  /**
   * Try to configure VideoDecoder when SPS/PPS are available
   */
  private tryConfigureDecoder(): void {
    if (!this.spsData || !this.ppsData || !this.decoder) {
      return;
    }

    try {
      const description = this.createAVCDescription(this.spsData, this.ppsData);

      const config = {
        codec: "avc1.42E01E", // H.264 Baseline Profile
        description,
        hardwareAcceleration: "prefer-hardware" as const,
        optimizeForLatency: true,
      };

      this.decoder.configure(config);

      // 配置后需要等待关键帧
      this.waitingForKeyframe = true;
    } catch (error) {
      console.error("[H264Client] Failed to configure VideoDecoder:", error);
      // Don't close decoder on configuration error, just log it
    }
  }

  /**
   * Create AVC Configuration Record from SPS and PPS
   */
  private createAVCDescription(sps: Uint8Array, pps: Uint8Array): ArrayBuffer {
    // Create AVC Configuration Record
    const configLength = 11 + sps.length + pps.length;
    const config = new Uint8Array(configLength);
    let offset = 0;

    // AVC Configuration Record header
    config[offset++] = 0x01; // configurationVersion
    config[offset++] = sps[1]; // AVCProfileIndication
    config[offset++] = sps[2]; // profile_compatibility
    config[offset++] = sps[3]; // AVCLevelIndication
    config[offset++] = 0xff; // lengthSizeMinusOne (3) + reserved bits

    // SPS
    config[offset++] = 0xe1; // numOfSequenceParameterSets (1) + reserved bits
    config[offset++] = (sps.length >> 8) & 0xff; // sequenceParameterSetLength (high)
    config[offset++] = sps.length & 0xff; // sequenceParameterSetLength (low)
    config.set(sps, offset);
    offset += sps.length;

    // PPS
    config[offset++] = 0x01; // numOfPictureParameterSets
    config[offset++] = (pps.length >> 8) & 0xff; // pictureParameterSetLength (high)
    config[offset++] = pps.length & 0xff; // pictureParameterSetLength (low)
    config.set(pps, offset);

    return config.buffer;
  }

  /**
   * Decode H264 frame
   */
  private decodeFrame(nalUnit: Uint8Array): void {
    if (!this.decoder) {
      console.warn("[H264Client] Decoder not available for decoding");
      return;
    }

    // Check if decoder is configured and ready
    if (this.waitingForKeyframe) {
      console.warn("[H264Client] Waiting for keyframe, skipping decode");
      return;
    }

    // Check decoder state
    if (this.decoder.state !== "configured") {
      console.warn(
        "[H264Client] Decoder not configured, state:",
        this.decoder.state
      );
      return;
    }

    try {
      // Convert NAL unit to AVC format (add length prefix)
      const avcData = this.convertNALToAVC(nalUnit);

      const nalType = nalUnit[0] & 0x1f;
      const isIDR = nalType === NALU.IDR;

      const chunk = new EncodedVideoChunk({
        type: isIDR ? "key" : "delta",
        timestamp: performance.now() * 1000, // Use performance.now() for better timing
        data: avcData,
      });

      this.decoder.decode(chunk);
    } catch (error) {
      console.error("[H264Client] Decode error:", error);

      // If decode fails, try to recreate decoder
      if (this.decoder && this.decoder.state !== "configured") {
        console.log("[H264Client] Recreating decoder due to decode failure");
        this.recreateDecoder();
      }
    }
  }

  /**
   * Convert NAL unit to AVC format (add length prefix)
   */
  private convertNALToAVC(nalUnit: Uint8Array): ArrayBuffer {
    const lengthPrefix = new Uint8Array(4);
    const view = new DataView(lengthPrefix.buffer);
    view.setUint32(0, nalUnit.length, false); // Big-endian

    const avcData = new Uint8Array(4 + nalUnit.length);
    avcData.set(lengthPrefix, 0);
    avcData.set(nalUnit, 4);

    return avcData.buffer;
  }

  /**
   * Recreate VideoDecoder when it fails
   */
  private recreateDecoder(): void {
    // console.log("[H264Client] Recreating VideoDecoder...");

    // Close existing decoder
    if (this.decoder && this.decoder.state === "configured") {
      this.decoder.close();
    }

    // Reset keyframe waiting state
    this.waitingForKeyframe = true;

    // Create new decoder
    this.decoder = new VideoDecoder({
      output: (frame) => this.onFrameDecoded(frame),
      error: (error) => {
        console.error("[H264Client] VideoDecoder error:", error);
        // Don't call handleError for decoder errors, just log them
      },
    });

    // Reconfigure with existing SPS/PPS data
    if (this.spsData && this.ppsData) {
      this.tryConfigureDecoder();
    }
  }

  /**
   * Handle decoded frame
   */
  private onFrameDecoded(frame: VideoFrame): void {
    this.decodedFrames.push({
      frame,
      timestamp: Date.now(),
    });

    // Render frame
    this.renderFrame();

    // Update stats
    this.statsService.recordFrameDecoded();
  }

  /**
   * Render frame to canvas
   */
  private renderFrame(): void {
    if (this.decodedFrames.length === 0) return;

    const { frame } = this.decodedFrames.shift()!;

    // Use video render service (it handles resolution updates)
    this.videoRender.renderFrame(frame);
  }

  /**
   * Connect control WebSocket
   */
  private async connectControl(
    deviceSerial: string,
    apiUrl: string,
    wsUrl: string
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      // Build WebSocket URL using base class method
      const controlWsUrl = this.buildControlWebSocketUrlFromParams({
        deviceSerial,
        apiUrl,
        wsUrl,
      });

      this.controlWs = new WebSocket(controlWsUrl);

      this.controlWs.onopen = () => {
        this.controlRetryCount = 0;
        resolve();
      };

      this.controlWs.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          this.handleControlMessage(message);
        } catch (error) {
          console.error("[H264Client] Control message parse error:", error);
        }
      };

      this.controlWs.onclose = () => {
        // console.log("[H264Client] Control WebSocket closed");
        if (this.connected) {
          this.handleControlReconnection(deviceSerial, apiUrl, wsUrl);
        }
      };

      this.controlWs.onerror = (error) => {
        console.error("[H264Client] Control WebSocket error:", error);
        reject(error);
      };
    });
  }

  /**
   * Handle control reconnection
   */
  private handleControlReconnection(
    deviceSerial: string,
    apiUrl: string,
    wsUrl: string
  ): void {
    if (this.controlRetryCount >= this.maxControlRetries) {
      console.error("[H264Client] Max control reconnection attempts reached");
      return;
    }

    this.controlRetryCount++;
    console.log(
      `[H264Client] Reconnecting control WebSocket... (attempt ${this.controlRetryCount}/${this.maxControlRetries})`
    );

    // this.controlReconnectTimer = window.setTimeout(() => {
    //   this.connectControl(deviceSerial, apiUrl, wsUrl).catch(console.error);
    // }, 1000 * this.controlRetryCount);

    // Use reconnection service instead
    this.reconnectionService.startReconnection(
      () => this.connectControl(deviceSerial, apiUrl, wsUrl),
      "H264Control"
    );
  }

  /**
   * Connect audio
   */
  private async connectAudio(audioUrl: string): Promise<void> {
    try {
      this.audioProcessor = new MSEAudioProcessor(this.container);
      await this.audioProcessor.connect(audioUrl);
    } catch (error) {
      console.warn("[H264Client] Audio connection failed:", error);
      // Don't throw error, audio is optional
    }
  }

  /**
   * Handle control message
   */
  private handleControlMessage(message: ControlMessage): void {
    // Handle ping responses for latency measurement
    this.handlePingResponse(message);
  }

  /**
   * Measure ping latency - implementation for H264 client
   */
  protected measurePing(): void {
    if (this.controlWs && this.controlWs.readyState === WebSocket.OPEN) {
      const pingId = `ping_${Date.now()}_${Math.random()}`;
      this.pendingPings.set(pingId, performance.now());

      this.controlWs.send(
        JSON.stringify({
          type: "ping",
          id: pingId,
        })
      );
    } else {
      console.debug("[H264Client] Control WebSocket not ready for ping");
    }
  }

  /**
   * Start resize observer
   */
  private startResizeObserver(): void {
    if (this.resizeObserver) return;

    this.resizeObserver = new ResizeObserver(() => {
      if (this.lastCanvasDimensions) {
        this.videoRender.updateCanvasDisplaySize(
          this.lastCanvasDimensions.width,
          this.lastCanvasDimensions.height
        );
      }
    });

    this.resizeObserver.observe(this.container);
  }

  /**
   * Stop resize observer
   */
  private stopResizeObserver(): void {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }
  }

  /**
   * Request keyframe
   */
  requestKeyframe(): void {
    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      return;
    }

    const message: ControlMessage = {
      type: "reset_video",
      timestamp: Date.now(),
    };

    this.controlWs.send(JSON.stringify(message));
  }

  // Override ControlClient methods for H264-specific implementation
  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void {
    console.log(
      `[H264Client] sendKeyEvent: keycode=${keycode}, action=${action}, metaState=${
        metaState || 0
      }`
    );

    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      console.warn(
        `[H264Client] Cannot send key event: WebSocket not ready. State: ${this.controlWs?.readyState}`
      );
      return;
    }

    const message: ControlMessage = {
      type: "key",
      keycode,
      action,
      metaState: metaState || 0,
      timestamp: Date.now(),
    };

    console.log(`[H264Client] Sending control message:`, message);
    this.controlWs.send(JSON.stringify(message));
  }

  sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure?: number
  ): void {
    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      return;
    }

    const message: ControlMessage = {
      type: "touch",
      x,
      y,
      action,
      pressure: pressure || 1.0,
      timestamp: Date.now(),
    };

    this.controlWs.send(JSON.stringify(message));
  }

  sendControlAction(action: string, params?: Record<string, unknown>): void {
    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      return;
    }

    const message: ControlMessage = {
      type: action as ControlMessage["type"],
      ...params,
      timestamp: Date.now(),
    };

    this.controlWs.send(JSON.stringify(message));
  }

  sendClipboardSet(text: string, paste?: boolean): void {
    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      return;
    }

    const message: ControlMessage = {
      type: "clipboard_set",
      text,
      paste: paste || false,
      timestamp: Date.now(),
    };

    this.controlWs.send(JSON.stringify(message));
  }

  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void {
    // Get the canvas element for coordinate calculation
    const canvas = this.getCanvas();
    if (!canvas) {
      console.warn("[H264Client] No canvas available for mouse event");
      return;
    }

    // Use base class coordinate normalization
    const { x, y } = this.normalizeCoordinates(
      event.clientX,
      event.clientY,
      canvas
    );

    this.sendTouchEvent(x, y, action);
  }

  handleTouchEvent(event: TouchEvent, action: "down" | "up" | "move"): void {
    if (event.touches.length === 0) return;

    // Get the canvas element for coordinate calculation
    const canvas = this.getCanvas();
    if (!canvas) {
      console.warn("[H264Client] No canvas available for touch event");
      return;
    }

    const touch = event.touches[0];
    // Use base class coordinate normalization
    const { x, y } = this.normalizeCoordinates(
      touch.clientX,
      touch.clientY,
      canvas
    );

    this.sendTouchEvent(x, y, action);
  }

  /**
   * Get audio processor for external access
   */
  getAudioProcessor(): MSEAudioProcessor | null {
    return this.audioProcessor;
  }

  /**
   * Get canvas element for H264 rendering
   */
  getCanvas(): HTMLCanvasElement | null {
    return this.videoRender.getCanvas();
  }
}
