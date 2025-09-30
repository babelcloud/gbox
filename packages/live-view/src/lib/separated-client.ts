// Simplified H264Client based on the working old version
import { ClientOptions, ControlClient } from "./types";

// NAL Unit types
const NALU = {
  SPS: 7, // Sequence Parameter Set
  PPS: 8, // Picture Parameter Set
  IDR: 5, // IDR frame
} as const;

// Simplified MSEAudioProcessor for WebM/Opus audio
class MSEAudioProcessor {
  private mediaSource: MediaSource | null = null;
  private sourceBuffer: SourceBuffer | null = null;
  private audioElement: HTMLAudioElement | null = null;
  private reader: ReadableStreamDefaultReader<Uint8Array> | null = null;
  private abortController: AbortController | null = null;
  private isStreaming: boolean = false;

  constructor(private container: HTMLElement) {}

  async connect(audioUrl: string): Promise<void> {
    console.log("[MSEAudio] Connecting to:", audioUrl);

    // Check MSE support
    if (
      !window.MediaSource ||
      !MediaSource.isTypeSupported('audio/webm; codecs="opus"')
    ) {
      throw new Error("Browser does not support WebM/Opus MSE");
    }

    this.isStreaming = true;

    // Create audio element
    this.audioElement = document.createElement("audio");
    this.audioElement.controls = false;
    this.audioElement.style.display = "none";
    this.audioElement.autoplay = true;
    this.audioElement.muted = false; // Enable audio playback
    console.log("[MSEAudio] Created audio element with autoplay enabled");

    // Add audio event listeners for debugging
    this.audioElement.addEventListener("loadstart", () => {
      console.log("[MSEAudio] Audio load started");
    });
    this.audioElement.addEventListener("canplay", () => {
      console.log("[MSEAudio] Audio can play");
    });
    this.audioElement.addEventListener("playing", () => {
      console.log("[MSEAudio] Audio is playing");
    });
    this.audioElement.addEventListener("error", (e) => {
      console.error("[MSEAudio] Audio error:", e);
    });
    this.audioElement.addEventListener("volumechange", () => {
      console.log(
        "[MSEAudio] Volume changed:",
        this.audioElement?.volume,
        "muted:",
        this.audioElement?.muted
      );
    });

    this.container.appendChild(this.audioElement);

    // Create MediaSource
    this.mediaSource = new MediaSource();
    this.audioElement.src = URL.createObjectURL(this.mediaSource);

    // Wait for MediaSource to open
    await new Promise((resolve, reject) => {
      if (this.mediaSource) {
        this.mediaSource.addEventListener("sourceopen", resolve, {
          once: true,
        });
        this.mediaSource.addEventListener("error", reject, { once: true });
      } else {
        reject(new Error("MediaSource not available"));
      }
    });

    // Create SourceBuffer only after MediaSource is open
    if (this.mediaSource && this.mediaSource.readyState === "open") {
      this.sourceBuffer = this.mediaSource.addSourceBuffer(
        'audio/webm; codecs="opus"'
      );
    } else {
      throw new Error("MediaSource not ready for SourceBuffer creation");
    }

    // Start streaming
    await this.startStreaming(audioUrl);
  }

  private async startStreaming(audioUrl: string): Promise<void> {
    try {
      this.abortController = new AbortController();
      const response = await fetch(audioUrl, {
        signal: this.abortController.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      if (!response.body) {
        throw new Error("ReadableStream not supported");
      }

      this.reader = response.body.getReader();

      while (this.isStreaming) {
        const { done, value } = await this.reader.read();

        if (done) {
          console.log("[MSEAudio] Stream ended");
          break;
        }

        // Append data to SourceBuffer
        if (
          this.sourceBuffer &&
          !this.sourceBuffer.updating &&
          this.mediaSource &&
          this.mediaSource.readyState === "open"
        ) {
          try {
            this.sourceBuffer.appendBuffer(
              value.buffer.slice(
                value.byteOffset,
                value.byteOffset + value.byteLength
              ) as ArrayBuffer
            );
          } catch (e) {
            console.error("[MSEAudio] SourceBuffer append failed:", e);
          }
        } else {
          await new Promise((resolve) => setTimeout(resolve, 10));
        }
      }
    } catch (error) {
      if (error instanceof Error && error.name !== "AbortError") {
        console.error("[MSEAudio] Stream processing error:", error);
      }
    }
  }

  disconnect(): void {
    console.log("[MSEAudio] Disconnecting audio stream");
    this.isStreaming = false;

    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    if (this.reader) {
      this.reader.cancel().catch(() => {});
      this.reader = null;
    }

    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.remove();
      this.audioElement = null;
    }

    if (this.mediaSource) {
      try {
        if (this.mediaSource.readyState === "open") {
          this.mediaSource.endOfStream();
        }
      } catch (e) {
        console.log("[MSEAudio] MediaSource close error:", e);
      }
      this.mediaSource = null;
    }

    this.sourceBuffer = null;
  }
}

export class H264Client implements ControlClient {
  private container: HTMLElement;
  private canvas: HTMLCanvasElement | null = null;
  private context: CanvasRenderingContext2D | null = null;
  private decoder: VideoDecoder | null = null;
  private abortController: AbortController | null = null;
  private buffer: Uint8Array = new Uint8Array(0);
  private spsData: Uint8Array | null = null;
  private ppsData: Uint8Array | null = null;
  private audioProcessor: MSEAudioProcessor | null = null;
  public isConnected: boolean = false;
  private options: ClientOptions;

  // Stats tracking
  private lastFrameTime: number = 0;
  private frameCount: number = 0;
  private statsUpdateInterval: number = 0;

  // Control properties
  private controlWebSocket: WebSocket | null = null;
  private controlConnected: boolean = false;
  public isMouseDragging: boolean = false;

  // Callbacks
  private onConnectionStateChange?: (state: string, message?: string) => void;
  private onError?: (error: Error) => void;
  private onStatsUpdate?: (stats: {
    resolution?: string;
    fps?: number;
    latency?: number;
  }) => void;

  constructor(container: HTMLElement, options: ClientOptions = {}) {
    this.container = container;
    this.options = options;
    this.onConnectionStateChange = options.onConnectionStateChange as (
      state: string,
      message?: string
    ) => void;
    this.onError = options.onError;
    this.onStatsUpdate = options.onStatsUpdate;
    this.initializeWebCodecs();
  }

  /**
   * Initialize WebCodecs decoder
   */
  private initializeWebCodecs(): void {
    // Check if WebCodecs is supported
    if (typeof VideoDecoder !== "function") {
      console.error("[H264Client] WebCodecs not supported");
      this.onError?.(new Error("WebCodecs not supported"));
      return;
    }

    try {
      // Create canvas for rendering if it doesn't exist
      if (!this.canvas) {
        this.canvas = document.createElement("canvas");
        this.canvas.style.width = "100%";
        this.canvas.style.height = "100%";
        this.canvas.style.display = "block";
        this.canvas.style.objectFit = "contain";
        this.canvas.style.background = "#000";

        // Set initial dimensions to prevent layout shift
        // These will be updated when we receive the first frame
        this.canvas.width = 1920; // Default width
        this.canvas.height = 1080; // Default height

        this.container.appendChild(this.canvas);
      }

      // Get 2D context
      const context = this.canvas.getContext("2d");
      if (!context) {
        throw new Error("Failed to get 2d context from canvas");
      }
      this.context = context;

      // Create VideoDecoder
      this.decoder = new VideoDecoder({
        output: (frame) => this.onFrameDecoded(frame),
        error: (error: DOMException) => {
          console.error("[H264Client] VideoDecoder error:", error);

          // Reset decoder on error to allow recovery
          if (this.decoder && this.decoder.state === "closed") {
            console.log(
              "[H264Client] Decoder closed due to error, resetting..."
            );
            this.decoder = null;
            this.spsData = null;
            this.ppsData = null;

            // Reinitialize decoder after a short delay
            setTimeout(() => {
              this.initializeWebCodecs();
            }, 100);
          }

          this.onError?.(new Error(`VideoDecoder error: ${error.message}`));
        },
      });
    } catch (error) {
      console.error("[H264Client] WebCodecs initialization failed:", error);
      this.onError?.(new Error("WebCodecs initialization failed"));
    }
  }

  /**
   * Connect to H.264 AVC format stream
   */
  public async connect(
    deviceSerial: string,
    apiUrl: string = "/api",
    wsUrl?: string
  ): Promise<void> {
    const url = `${apiUrl}/devices/${deviceSerial}/video?mode=h264&format=avc`;

    // Reset connection without removing canvas/video elements
    await this.resetConnection();

    // Connect to control WebSocket first
    if (wsUrl) {
      await this.connectControl(deviceSerial, wsUrl);
    }

    // Always reset decoder for new stream to avoid conflicts
    if (this.decoder) {
      try {
        this.decoder.close();
      } catch (error) {
        console.warn("[H264Client] Error closing existing decoder:", error);
      }
      this.decoder = null;
      this.spsData = null;
      this.ppsData = null;

      // Wait a bit for decoder to fully close before initializing new one
      await new Promise((resolve) => setTimeout(resolve, 50));
    }

    // Initialize new decoder
    this.initializeWebCodecs();

    if (!this.decoder) {
      throw new Error("WebCodecs decoder not ready");
    }

    // Notify connecting state
    this.onConnectionStateChange?.(
      "connecting",
      "Connecting to H.264 stream..."
    );

    try {
      // Start HTTP stream and wait for it to establish connection
      await this.startHTTP(url);

      // Connect audio if enabled (run after video is established)
      if (this.options.enableAudio) {
        try {
          const audioUrl = `${apiUrl}/devices/${deviceSerial}/audio?codec=${
            this.options.audioCodec || "opus"
          }&format=webm`;
          await this.connectAudio(audioUrl);
          console.log("[H264Client] Audio connected successfully");
        } catch (error) {
          console.error(
            "[H264Client] Audio connection failed, continuing without audio:",
            error
          );
        }
      }

      // Connection is now fully established
      console.log("[H264Client] Connection established successfully");
      console.log("[H264Client] isConnected state:", this.isConnected);
      console.log("[H264Client] Control connected:", this.isControlConnected());

      // Ensure isConnected is true after successful connection
      if (!this.isConnected) {
        console.warn(
          "[H264Client] isConnected was false after connection, setting to true"
        );
        this.isConnected = true;
      }
    } catch (error) {
      console.error("[H264Client] Connection failed:", error);
      this.isConnected = false;
      this.onConnectionStateChange?.("error", "H.264 connection failed");
      this.onError?.(error as Error);
      throw error;
    }
  }

  /**
   * Start HTTP stream
   */
  private async startHTTP(url: string): Promise<void> {
    this.abortController = new AbortController();

    const response = await fetch(url, {
      signal: this.abortController.signal,
    });

    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body reader available");
    }

    // HTTP stream is now successfully connected
    this.isConnected = true;
    this.onConnectionStateChange?.("connected", "H.264 stream connected");

    // Start stream processing in background (don't wait for it to complete)
    this.processStreamData(reader).catch((error) => {
      if (error instanceof Error && error.name !== "AbortError") {
        console.error("[H264Client] Stream processing error:", error);
        this.onError?.(error);
      }
    });
  }

  private async processStreamData(
    reader: ReadableStreamDefaultReader<Uint8Array>
  ): Promise<void> {
    let chunkCount = 0;
    let totalBytes = 0;

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          console.log(
            "[H264Client] Stream ended, total chunks:",
            chunkCount,
            "total bytes:",
            totalBytes
          );
          break;
        }

        if (value && value.length) {
          chunkCount++;
          totalBytes += value.length;

          if (chunkCount <= 5) {
            console.log(
              "[H264Client] Received chunk",
              chunkCount,
              "size:",
              value.length,
              "total bytes:",
              totalBytes
            );
          }

          // Append new data to buffer with size limit
          const maxBufferSize = 5 * 1024 * 1024; // 5MB limit
          const newBufferSize = this.buffer.length + value.length;

          if (newBufferSize > maxBufferSize) {
            console.warn(
              "[H264Client] Buffer size limit exceeded, clearing buffer"
            );
            this.buffer = new Uint8Array(0);
          }

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
      }
    } catch (error) {
      console.error("[H264Client] Stream processing error:", error);
      throw error;
    }
  }

  /**
   * Parse H.264 AVC format stream and extract NAL units
   */
  private parseAVC(data: Uint8Array): {
    processedNals: Uint8Array[];
    remainingBuffer: Uint8Array;
  } {
    const processedNals: Uint8Array[] = [];
    let offset = 0;
    const maxIterations = 1000; // Prevent infinite loops
    let iterations = 0;

    while (offset < data.length - 4 && iterations < maxIterations) {
      iterations++;

      // Read length prefix (4 bytes, big-endian)
      const length =
        (data[offset] << 24) |
        (data[offset + 1] << 16) |
        (data[offset + 2] << 8) |
        data[offset + 3];

      offset += 4;

      // Validate length to prevent infinite loops
      if (length <= 0 || length > 1024 * 1024) {
        // Max 1MB per NAL unit
        console.warn("[H264Client] Invalid NAL unit length:", length);
        break;
      }

      // Check if we have enough data for the NAL unit
      if (offset + length > data.length) {
        offset -= 4; // Put back the length prefix
        break;
      }

      // Extract NAL unit
      const nalData = data.slice(offset, offset + length);
      if (nalData.length > 0) {
        processedNals.push(nalData);
      }

      offset += length;
    }

    if (iterations >= maxIterations) {
      console.warn(
        "[H264Client] parseAVC reached max iterations, possible infinite loop"
      );
    }

    return {
      processedNals,
      remainingBuffer: data.slice(offset),
    };
  }

  /**
   * Process individual NAL unit
   */
  private processNALUnit(nalData: Uint8Array): void {
    if (nalData.length === 0) return;

    const nalType = nalData[0] & 0x1f;

    // Handle SPS
    if (nalType === NALU.SPS) {
      this.spsData = nalData;
      this.tryConfigureDecoder();
      return;
    }

    // Handle PPS
    if (nalType === NALU.PPS) {
      this.ppsData = nalData;
      this.tryConfigureDecoder();
      return;
    }

    // Only decode if we have SPS and PPS
    if (!this.spsData || !this.ppsData) {
      console.log(
        "[H264Client] Skipping NAL unit type",
        nalType,
        "- waiting for SPS/PPS"
      );
      return;
    }

    // Decode frame
    this.decodeFrame(nalData);
  }

  /**
   * Try to configure decoder when we have both SPS and PPS
   */
  private tryConfigureDecoder(): void {
    if (!this.spsData || !this.ppsData || !this.decoder) {
      return;
    }

    try {
      // Create AVC description
      const description = this.createAVCDescription(this.spsData, this.ppsData);

      const config: VideoDecoderConfig = {
        codec: "avc1.42E01E", // H.264 Baseline Profile
        optimizeForLatency: true,
        description,
        hardwareAcceleration: "prefer-hardware" as HardwareAcceleration,
      };

      // Configure decoder
      this.decoder.configure(config);
    } catch (error) {
      console.error("[H264Client] Decoder configuration failed:", error);
    }
  }

  /**
   * Create AVC description for VideoDecoderConfig (avcC format)
   */
  private createAVCDescription(
    spsData: Uint8Array,
    ppsData: Uint8Array
  ): ArrayBuffer {
    // Create avcC (AVC Configuration Record) format
    const spsLength = new Uint8Array(2);
    const spsView = new DataView(spsLength.buffer);
    spsView.setUint16(0, spsData.length, false); // Big-endian

    const ppsLength = new Uint8Array(2);
    const ppsView = new DataView(ppsLength.buffer);
    ppsView.setUint16(0, ppsData.length, false); // Big-endian

    // avcC format: [version][profile][compatibility][level][lengthSizeMinusOne][numSPS][SPS...][numPPS][PPS...]
    const avcC = new Uint8Array(
      6 + 2 + spsData.length + 1 + 2 + ppsData.length
    );
    let offset = 0;

    // avcC header
    avcC[offset++] = 0x01; // configurationVersion
    avcC[offset++] = spsData[1]; // AVCProfileIndication (from SPS)
    avcC[offset++] = spsData[2]; // profile_compatibility (from SPS)
    avcC[offset++] = spsData[3]; // AVCLevelIndication (from SPS)
    avcC[offset++] = 0xff; // lengthSizeMinusOne (0xFF = 4-byte length)
    avcC[offset++] = 0xe1; // numOfSequenceParameterSets (0xE1 = 1 SPS)

    // SPS
    avcC.set(spsLength, offset);
    offset += 2;
    avcC.set(spsData, offset);
    offset += spsData.length;

    // PPS
    avcC[offset++] = 0x01; // numOfPictureParameterSets (1 PPS)
    avcC.set(ppsLength, offset);
    offset += 2;
    avcC.set(ppsData, offset);

    return avcC.buffer;
  }

  /**
   * Decode H.264 frame
   */
  private decodeFrame(nalData: Uint8Array): void {
    if (!this.decoder || this.decoder.state !== "configured") {
      // If decoder is closed, try to reinitialize
      if (this.decoder && this.decoder.state === "closed") {
        this.decoder = null;
        this.spsData = null;
        this.ppsData = null;
        this.initializeWebCodecs();
      }
      return;
    }

    const nalType = nalData[0] & 0x1f;
    const isIDR = nalType === NALU.IDR;

    try {
      // Convert NAL unit to AVC format (add length prefix)
      const avcData = this.convertNALToAVC(nalData);

      // Use performance.now() for better timing accuracy
      const timestamp = performance.now() * 1000; // Convert to microseconds

      // Create EncodedVideoChunk
      const chunk = new EncodedVideoChunk({
        type: isIDR ? "key" : "delta",
        timestamp: timestamp,
        data: avcData,
      });

      // Debug: Log frame decoding (disabled for production)
      // console.log("[H264Client] Decoding frame:", {
      //   nalType,
      //   isIDR,
      //   nalDataLength: nalData.length,
      //   avcDataLength: avcData.byteLength,
      //   timestamp,
      //   chunkType: isIDR ? "key" : "delta",
      // });

      // Decode the chunk
      this.decoder.decode(chunk);
    } catch (error) {
      console.error("[H264Client] Failed to decode frame:", error);
    }
  }

  /**
   * Convert NAL unit to AVC format (add length prefix)
   */
  private convertNALToAVC(nalUnit: Uint8Array): ArrayBuffer {
    // Create 4-byte length prefix (big-endian)
    const lengthPrefix = new Uint8Array(4);
    const view = new DataView(lengthPrefix.buffer);
    view.setUint32(0, nalUnit.length, false); // Big-endian

    // Combine length prefix + NAL unit data
    const avcData = new Uint8Array(4 + nalUnit.length);
    avcData.set(lengthPrefix, 0);
    avcData.set(nalUnit, 4);

    return avcData.buffer;
  }

  /**
   * Handle decoded frame
   */
  private onFrameDecoded(frame: VideoFrame): void {
    if (!this.context || !this.canvas) {
      console.log(
        "[H264Client] Cannot render frame - missing context or canvas"
      );
      return;
    }

    try {
      // Update frame count for FPS calculation
      this.frameCount++;

      // Update canvas size to match frame
      if (
        this.canvas.width !== frame.displayWidth ||
        this.canvas.height !== frame.displayHeight
      ) {
        console.log("[H264Client] Resizing canvas to match frame:", {
          frameWidth: frame.displayWidth,
          frameHeight: frame.displayHeight,
        });

        // Check if this is the first real frame (not default dimensions)
        const isFirstRealFrame =
          this.canvas.width === 1920 && this.canvas.height === 1080;

        // Update canvas dimensions
        this.canvas.width = frame.displayWidth;
        this.canvas.height = frame.displayHeight;

        // Only trigger resize event for first frame to avoid flicker
        if (isFirstRealFrame) {
          window.dispatchEvent(new Event("resize"));
        }
      }

      // Calculate and update stats every second
      const now = performance.now();
      if (!this.lastFrameTime) {
        this.lastFrameTime = now;
        this.frameCount = 1; // Start counting from 1
        return; // Skip stats update on first frame
      }

      const timeDiff = now - this.lastFrameTime;

      // Update stats every second
      if (timeDiff >= 1000) {
        const fps = Math.round((this.frameCount * 1000) / timeDiff);

        // Calculate latency (approximate)
        // Latency is the time from when the frame was captured to when it's displayed
        const frameTimestamp = frame.timestamp / 1000; // Convert from microseconds to milliseconds
        const latency = Math.round(now - frameTimestamp);

        // Update resolution
        const resolution = `${frame.displayWidth}x${frame.displayHeight}`;

        // Update stats
        this.onStatsUpdate?.({
          fps,
          latency: Math.max(0, latency), // Ensure latency is not negative
          resolution,
        });

        // Reset counters
        this.lastFrameTime = now;
        this.frameCount = 0;
      }

      // Draw frame to canvas
      this.context.drawImage(frame, 0, 0);
    } catch (error) {
      console.error("[H264Client] Failed to render frame:", error);
    } finally {
      frame.close();
    }
  }

  /**
   * Connect to audio stream using MSEAudioProcessor
   */
  private async connectAudio(audioUrl: string): Promise<void> {
    console.log("[H264Client] Connecting to audio stream:", audioUrl);

    try {
      // Create MSEAudioProcessor for WebM/Opus audio
      this.audioProcessor = new MSEAudioProcessor(this.container);
      await this.audioProcessor.connect(audioUrl);
      console.log("[H264Client] Audio connected successfully");
    } catch (error) {
      console.error("[H264Client] Failed to connect audio:", error);
      throw error;
    }
  }

  /**
   * Reset connection without removing UI elements
   */
  public async resetConnection(): Promise<void> {
    // Notify disconnecting state
    this.isConnected = false;
    this.onConnectionStateChange?.("disconnected", "H.264 stream disconnected");

    // Cancel HTTP request first
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    // Close decoder
    if (this.decoder) {
      this.decoder.close();
      this.decoder = null;
    }

    // Disconnect audio processor
    if (this.audioProcessor) {
      this.audioProcessor.disconnect();
      this.audioProcessor = null;
    }

    // Disconnect control WebSocket
    if (this.controlWebSocket) {
      this.controlWebSocket.close();
      this.controlWebSocket = null;
      this.controlConnected = false;
    }

    // Reset state variables but keep canvas and context
    this.buffer = new Uint8Array(0);
    this.spsData = null;
    this.ppsData = null;

    // Reset stats tracking
    this.lastFrameTime = 0;
    this.frameCount = 0;
    if (this.statsUpdateInterval) {
      clearInterval(this.statsUpdateInterval);
      this.statsUpdateInterval = 0;
    }
  }

  /**
   * Disconnect and cleanup - removes all UI elements
   * Only call this when switching streaming modes or destroying the client
   */
  public async disconnect(): Promise<void> {
    console.log("[H264Client] Disconnecting and cleaning up resources...");

    // First reset the connection
    await this.resetConnection();

    // Then remove UI elements
    if (this.canvas) {
      if (this.canvas.parentNode) {
        this.canvas.parentNode.removeChild(this.canvas);
      }
      this.canvas = null;
    }

    this.context = null;
  }

  // ControlClient interface implementation
  public async connectControl(
    deviceSerial: string,
    wsUrl: string
  ): Promise<void> {
    try {
      // Build control WebSocket URL using the same logic as BaseClient
      // Convert http:// to ws:// or https:// to wss://
      let controlWsUrl;
      if (wsUrl.startsWith("ws://") || wsUrl.startsWith("wss://")) {
        // Already a WebSocket URL, just append the path
        controlWsUrl = `${wsUrl}/api/devices/${deviceSerial}/control`;
      } else {
        // Convert HTTP to WebSocket and append path
        controlWsUrl = `${wsUrl}/api/devices/${deviceSerial}/control`.replace(
          /^http/,
          "ws"
        );
      }
      console.log(`[H264Client] Control WebSocket URL: ${controlWsUrl}`);

      this.controlWebSocket = new WebSocket(controlWsUrl);

      this.controlWebSocket.onopen = () => {
        console.log("[H264Client] Control WebSocket connected");
        this.controlConnected = true;
        console.log("[H264Client] Control connection state updated:", {
          controlConnected: this.controlConnected,
        });
      };

      this.controlWebSocket.onclose = () => {
        console.log("[H264Client] Control WebSocket disconnected");
        this.controlConnected = false;
      };

      this.controlWebSocket.onerror = (error) => {
        console.error("[H264Client] Control WebSocket error:", error);
        this.controlConnected = false;
      };
    } catch (error) {
      console.error("[H264Client] Failed to connect control WebSocket:", error);
      throw error;
    }
  }

  public isControlConnected(): boolean {
    return this.controlConnected;
  }

  public sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void {
    if (!this.isControlConnected || !this.controlWebSocket) {
      console.warn("[H264Client] Control not connected, cannot send key event");
      return;
    }

    const message = {
      type: "key",
      keycode,
      action,
      metaState: metaState || 0,
    };

    this.controlWebSocket.send(JSON.stringify(message));
  }

  public sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure?: number
  ): void {
    if (!this.isControlConnected || !this.controlWebSocket) {
      return;
    }

    const message = {
      type: "touch",
      x,
      y,
      action,
      pressure: pressure || 1.0,
    };

    this.controlWebSocket.send(JSON.stringify(message));
  }

  public sendControlAction(
    action: string,
    params?: Record<string, unknown>
  ): void {
    if (!this.isControlConnected || !this.controlWebSocket) {
      console.warn(
        "[H264Client] Control not connected, cannot send control action"
      );
      return;
    }

    const message = {
      type: "control",
      action,
      params: params || {},
    };

    this.controlWebSocket.send(JSON.stringify(message));
  }

  public sendClipboardSet(text: string, paste?: boolean): void {
    if (!this.isControlConnected || !this.controlWebSocket) {
      console.warn("[H264Client] Control not connected, cannot send clipboard");
      return;
    }

    const message = {
      type: "clipboard",
      text,
      paste: paste || false,
    };

    this.controlWebSocket.send(JSON.stringify(message));
  }

  public requestKeyframe(): void {
    // For H264 client, we don't need to request keyframes as they come naturally
    // This is a no-op for H264 streaming
  }

  public handleMouseEvent(
    event: MouseEvent,
    action: "down" | "up" | "move"
  ): void {
    if (!this.canvas) return;

    const rect = this.canvas.getBoundingClientRect();
    // Send normalized coordinates (0-1) like WebRTC mode
    const x = (event.clientX - rect.left) / rect.width;
    const y = (event.clientY - rect.top) / rect.height;

    if (action === "down") {
      this.isMouseDragging = true;
    } else if (action === "up") {
      this.isMouseDragging = false;
    }

    this.sendTouchEvent(x, y, action);
  }

  public handleTouchEvent(
    event: TouchEvent,
    action: "down" | "up" | "move"
  ): void {
    if (!this.canvas || event.touches.length === 0) return;

    const rect = this.canvas.getBoundingClientRect();
    const touch = event.touches[0];
    // Send normalized coordinates (0-1) like WebRTC mode
    const x = (touch.clientX - rect.left) / rect.width;
    const y = (touch.clientY - rect.top) / rect.height;

    if (action === "down") {
      this.isMouseDragging = true;
    } else if (action === "up") {
      this.isMouseDragging = false;
    }

    this.sendTouchEvent(x, y, action);
  }
}
