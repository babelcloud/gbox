// Types
interface H264ClientOptions {
  onConnectionStateChange?: (
    state: "connecting" | "connected" | "disconnected" | "error",
    message?: string
  ) => void;
  onError?: (error: Error) => void;
  onStatsUpdate?: (stats: any) => void;
}

// NAL Unit types
const NALU = {
  SPS: 7, // Sequence Parameter Set
  PPS: 8, // Picture Parameter Set
  IDR: 5, // IDR frame
} as const;

export class H264Client {
  private container: HTMLElement;
  private canvas: HTMLCanvasElement | null = null;
  private context: CanvasRenderingContext2D | null = null;
  private decoder: VideoDecoder | null = null;
  private abortController: AbortController | null = null;
  private opts: H264ClientOptions;
  private buffer: Uint8Array = new Uint8Array(0);
  private spsData: Uint8Array | null = null;
  private ppsData: Uint8Array | null = null;
  private animationFrameId: number | undefined;
  private decodedFrames: Array<{ frame: VideoFrame; timestamp: number }> = [];

  constructor(container: HTMLElement, opts: H264ClientOptions = {}) {
    this.container = container;
    this.opts = opts;
    this.initializeWebCodecs();
  }

  // Initialize WebCodecs decoder
  private initializeWebCodecs(): void {
    console.log("[H264Client] Initializing WebCodecs decoder...");

    // Check if WebCodecs is supported
    if (typeof VideoDecoder !== "function") {
      console.error("[H264Client] WebCodecs not supported");
      this.opts.onError?.(new Error("WebCodecs not supported"));
      return;
    }

    try {
      // Create canvas for rendering
      this.canvas = document.createElement("canvas");
      this.canvas.style.width = "100%";
      this.canvas.style.height = "100%";
      this.canvas.style.display = "block";
      this.container.appendChild(this.canvas);

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
          this.opts.onError?.(
            new Error(`VideoDecoder error: ${error.message}`)
          );
        },
      });

      console.log("[H264Client] WebCodecs decoder initialized successfully");
    } catch (error) {
      console.error("[H264Client] WebCodecs initialization failed:", error);
      this.opts.onError?.(new Error("WebCodecs initialization failed"));
    }
  }

  // Connect to H.264 AVC format stream
  public async connect(
    deviceSerial: string,
    apiUrl: string = "/api"
  ): Promise<void> {
    const url = `${apiUrl}/stream/video/${deviceSerial}?mode=h264&format=avc`;
    console.log("[H264Client] Connecting to H.264 AVC stream:", url);

    // Reinitialize WebCodecs if decoder is not ready (e.g., after disconnect)
    if (!this.decoder) {
      console.log(
        "[H264Client] Decoder not ready, reinitializing WebCodecs..."
      );
      this.initializeWebCodecs();
    }

    if (!this.decoder) {
      throw new Error("WebCodecs decoder not ready");
    }

    // Notify connecting state
    this.opts.onConnectionStateChange?.(
      "connecting",
      "Connecting to H.264 stream..."
    );

    try {
      await this.startHTTP(url);
      // Notify connected state
      this.opts.onConnectionStateChange?.(
        "connected",
        "H.264 stream connected"
      );
    } catch (error) {
      console.error("[H264Client] Connection failed:", error);
      this.opts.onConnectionStateChange?.("error", "H.264 connection failed");
      this.opts.onError?.(error as Error);
    }
  }

  // Start HTTP stream
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

    // Process stream data in async function
    (async () => {
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;

          if (value && value.length) {
            // Append new data to buffer
            const newBuffer = new Uint8Array(this.buffer.length + value.length);
            newBuffer.set(this.buffer);
            newBuffer.set(value, this.buffer.length);
            this.buffer = newBuffer;

            // Process NAL units from AVC format stream
            const { processedNals, remainingBuffer } = this.parseAVC(
              this.buffer
            );
            this.buffer = remainingBuffer;

            // Process each NAL unit
            for (const nalData of processedNals) {
              this.processNALUnit(nalData);
            }
          }
        }
      } catch (error) {
        // Only log error if it's not an abort error (which is expected when disconnecting)
        if (error instanceof Error && error.name !== "AbortError") {
          console.error("[H264Client] Stream processing error:", error);
        }
      }
    })();
  }

  // Parse H.264 AVC format stream and extract NAL units
  private parseAVC(data: Uint8Array): {
    processedNals: Uint8Array[];
    remainingBuffer: Uint8Array;
  } {
    const processedNals: Uint8Array[] = [];
    let offset = 0;

    while (offset < data.length - 4) {
      // Read length prefix (4 bytes, big-endian)
      const length =
        (data[offset] << 24) |
        (data[offset + 1] << 16) |
        (data[offset + 2] << 8) |
        data[offset + 3];

      offset += 4;

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

    return {
      processedNals,
      remainingBuffer: data.slice(offset),
    };
  }

  // Process individual NAL unit
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
      return;
    }

    // Decode frame
    this.decodeFrame(nalData);
  }

  // Try to configure decoder when we have both SPS and PPS
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

  // Create AVC description for VideoDecoderConfig (avcC format)
  private createAVCDescription(
    spsData: Uint8Array,
    ppsData: Uint8Array
  ): ArrayBuffer {
    // Create avcC (AVC Configuration Record) format
    // Reference: ISO/IEC 14496-15:2010 section 5.2.4.1

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

  // Decode H.264 frame
  private decodeFrame(nalData: Uint8Array): void {
    if (!this.decoder || this.decoder.state !== "configured") {
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

      // Decode the chunk
      this.decoder.decode(chunk);
    } catch (error) {
      console.error("[H264Client] Failed to decode frame:", error);

      // If decode fails, try to recreate decoder
      if (this.decoder && this.decoder.state !== "configured") {
        this.recreateDecoder();
      }
    }
  }

  // Convert NAL unit to AVC format (add length prefix)
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

  // Recreate decoder when it's closed
  private recreateDecoder(): void {
    // Close existing decoder if it exists
    if (this.decoder && this.decoder.state === "configured") {
      this.decoder.close();
    }

    // Create new decoder
    this.decoder = new VideoDecoder({
      output: (frame) => this.onFrameDecoded(frame),
      error: (error: DOMException) => {
        console.error("[H264Client] VideoDecoder error:", error);
        this.opts.onError?.(new Error(`VideoDecoder error: ${error.message}`));
      },
    });

    // Reconfigure with existing SPS/PPS data
    if (this.spsData && this.ppsData) {
      this.tryConfigureDecoder();
    }
  }

  // Handle decoded frame
  private onFrameDecoded(frame: VideoFrame): void {
    if (!this.context || !this.canvas) return;

    try {
      // Update canvas size to match frame
      if (
        this.canvas.width !== frame.displayWidth ||
        this.canvas.height !== frame.displayHeight
      ) {
        this.canvas.width = frame.displayWidth;
        this.canvas.height = frame.displayHeight;
      }

      // Draw frame to canvas
      this.context.drawImage(frame, 0, 0);
    } catch (error) {
      console.error("[H264Client] Failed to render frame:", error);
    } finally {
      frame.close();
    }
  }

  // Disconnect and cleanup
  public disconnect(): void {
    console.log("[H264Client] Disconnecting and cleaning up resources...");

    // Notify disconnecting state
    this.opts.onConnectionStateChange?.(
      "disconnected",
      "H.264 stream disconnected"
    );

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

    // Clear animation frame
    if (this.animationFrameId) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = undefined;
    }

    // Close all pending frames
    for (const { frame } of this.decodedFrames) {
      frame.close();
    }
    this.decodedFrames = [];

    // Clear canvas
    if (this.canvas && this.canvas.parentNode) {
      this.canvas.parentNode.removeChild(this.canvas);
      this.canvas = null;
    }

    this.context = null;
    this.buffer = new Uint8Array(0);
    this.spsData = null;
    this.ppsData = null;

    console.log("[H264Client] Disconnect completed");
  }

  // Alias for disconnect (for compatibility)
  public cleanup(): void {
    this.disconnect();
  }
}
