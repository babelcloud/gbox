import { ControlMessage } from "../types";

export class MSEClient {
  private ws: WebSocket | null = null;
  private currentDevice: string | null = null;
  private isConnected: boolean = false;
  public isMouseDragging: boolean = false;
  private lastMouseTime: number = 0;
  private videoElement: HTMLVideoElement | null = null;

  // Reconnection state
  private isReconnecting: boolean = false;
  private reconnectAttempts: number = 0;
  private readonly maxReconnectAttempts: number = 3; // Reduced to 3 attempts for better UX
  private reconnectTimer: number | null = null;
  private lastConnectedDevice: string | null = null;
  private lastBaseApiUrl: string | null = null;
  private isManuallyDisconnected: boolean = false;
  private shouldStopReconnecting: boolean = false;

  // Buffer management for smooth low latency
  private bufferMonitorInterval: number | null = null;
  private readonly MAX_BUFFER_DELAY = 0.3; // Maximum allowed buffer delay in seconds (balanced)
  private readonly CATCHUP_THRESHOLD = 0.2; // Start catch-up when delay exceeds this threshold
  private streamStartTime: number = 0; // Track when stream started for absolute delay calculation
  private lastCatchupTime: number = 0; // Track last catch-up to prevent too frequent adjustments

  // Callbacks
  private onConnectionStateChange?: (
    state: "connecting" | "connected" | "disconnected" | "error",
    message?: string
  ) => void;
  private onError?: (error: Error) => void;
  private onStatsUpdate?: (stats: any) => void;

  // Android key codes (same as WebRTC client)
  static readonly ANDROID_KEYCODES = {
    POWER: 26,
    VOLUME_UP: 24,
    VOLUME_DOWN: 25,
    BACK: 4,
    HOME: 3,
    APP_SWITCH: 187,
    MENU: 82,
  };

  constructor(
    videoElement: HTMLVideoElement,
    options: {
      onConnectionStateChange?: (
        state: "connecting" | "connected" | "disconnected" | "error",
        message?: string
      ) => void;
      onError?: (error: Error) => void;
      onStatsUpdate?: (stats: any) => void;
    } = {}
  ) {
    this.videoElement = videoElement;
    this.onConnectionStateChange = options.onConnectionStateChange;
    this.onError = options.onError;
    this.onStatsUpdate = options.onStatsUpdate;
  }

  async connect(
    deviceSerial: string,
    baseApiUrl: string = "/api"
  ): Promise<void> {
    if (this.isConnected) {
      console.log("[Streaming] Already connected");
      return;
    }

    console.log("[Streaming] Starting connection to", deviceSerial);
    this.isManuallyDisconnected = false; // Reset manual disconnect flag
    this.shouldStopReconnecting = false; // Reset stop reconnecting flag
    this.reconnectAttempts = 0; // Reset reconnection attempts
    this.lastConnectedDevice = deviceSerial; // Store for reconnection
    this.lastBaseApiUrl = baseApiUrl; // Store base API URL for reconnection
    this.onConnectionStateChange?.("connecting", "Connecting to device...");

    try {
      // Step 1: Connect to streaming API
      const response = await fetch(
        `${baseApiUrl}/stream/${deviceSerial}/connect`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            mode: "stream",
            config: {
              video_codec: "h264",
              audio_codec: "aac",
              enable_audio: true,
            },
          }),
        }
      );

      if (!response.ok) {
        let errorMessage = `Connection failed: ${response.status}`;
        if (response.status === 404) {
          errorMessage = "Device not found or not available";
        } else if (response.status === 500) {
          errorMessage = "Server error, please try again";
        } else if (response.status === 503) {
          errorMessage = "Service temporarily unavailable";
        }
        throw new Error(errorMessage);
      }

      const info = await response.json();
      console.log("[Streaming] Connected:", info);

      // Step 2: Set up video stream
      this.currentDevice = deviceSerial;
      this.lastConnectedDevice = deviceSerial;

      // Step 3: Set up control WebSocket first
      await this.setupControlWebSocket(deviceSerial);

      if (this.videoElement) {
        console.log(
          "[Streaming] Video element available, scheduling video stream setup..."
        );
        // Setup video stream immediately - no artificial delay
        console.log("[Streaming] Setting up video stream immediately...");
        this.setupVideoStream(deviceSerial, baseApiUrl);
      } else {
        console.log(
          "[Streaming] No video element available, skipping video stream setup"
        );
      }
    } catch (error) {
      console.error("[Streaming] Connection failed:", error);
      this.onConnectionStateChange?.("error", `Connection failed: ${error}`);
      this.onError?.(error as Error);

      // Start reconnection if not manually disconnected
      if (!this.isManuallyDisconnected && !this.shouldStopReconnecting) {
        this.scheduleReconnect(deviceSerial);
      }

      throw error;
    }
  }

  private async setupControlWebSocket(deviceSerial: string): Promise<void> {
    const wsUrl = `ws://${window.location.host}/api/stream/control/${deviceSerial}`;

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log("[Streaming] Control WebSocket connected");
    };

    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        console.log("[Streaming] Control message:", message);

        if (message.type === "pong") {
          // Handle pong responses
        }
      } catch (error) {
        console.error("[Streaming] Failed to parse control message:", error);
      }
    };

    this.ws.onerror = (error) => {
      console.error("[Streaming] Control WebSocket error:", error);
      // Don't trigger reconnection for WebSocket errors, let the main connection handle it
    };

    this.ws.onclose = () => {
      console.log("[Streaming] Control WebSocket closed");
      // Only trigger reconnection if not manually disconnected
      if (
        !this.isManuallyDisconnected &&
        !this.shouldStopReconnecting &&
        this.currentDevice
      ) {
        console.log(
          "[Streaming] WebSocket closed unexpectedly, will attempt reconnection"
        );
        this.scheduleReconnect(this.currentDevice);
      }
    };
  }

  private setupVideoStream(
    deviceSerial: string,
    baseApiUrl: string = "/api"
  ): void {
    console.log(
      "[Streaming] setupVideoStream called with deviceSerial:",
      deviceSerial,
      "baseApiUrl:",
      baseApiUrl
    );

    if (!this.videoElement) {
      console.log(
        "[Streaming] No video element in setupVideoStream, returning"
      );
      return;
    }

    console.log("[Streaming] Video element found, starting streaming modes...");

    // Try different streaming modes for optimal latency
    this.tryStreamingModes(deviceSerial, baseApiUrl).catch((error) => {
      console.error("[Streaming] Error in streaming modes:", error);
      this.onError?.(error);

      // Don't trigger reconnection for streaming mode failures
      // Only reconnect for connection-level failures
      console.log("[Streaming] Streaming mode failed, not reconnecting");
    });
  }

  private async tryStreamingModes(
    deviceSerial: string,
    baseApiUrl: string = "/api"
  ): Promise<void> {
    console.log(
      "[Streaming] tryStreamingModes called with deviceSerial:",
      deviceSerial,
      "baseApiUrl:",
      baseApiUrl
    );

    if (!this.videoElement) {
      console.log("[Streaming] No video element, streaming modes failed");
      return;
    }

    // Use MSE (fMP4) for best quality and lowest latency
    console.log("[Streaming] Attempting MSE stream...");
    if (await this.tryMSEStream(deviceSerial, baseApiUrl)) {
      console.log("[Streaming] Using MSE for best quality and lowest latency");
      return;
    }

    console.error("[Streaming] MSE streaming failed");

    // Mode 3: Try fragmented MP4 (no keyframe wait)
    if (this.tryFragmentedMP4(deviceSerial)) {
      console.log("[Streaming] Using fragmented MP4 for low latency");
      return;
    }

    // Mode 4: Fallback to regular MP4
    this.tryRegularMP4(deviceSerial);
  }

  private cleanupMSE(): void {
    // Stop buffer monitoring
    this.stopBufferMonitoring();

    if (this.videoElement) {
      // Clean up MediaSource
      if ((this.videoElement as any)._mediaSource) {
        const mediaSource = (this.videoElement as any)._mediaSource;
        if (mediaSource.readyState === "open") {
          try {
            mediaSource.endOfStream();
          } catch (e) {
            // Ignore errors when ending stream
          }
        }
        (this.videoElement as any)._mediaSource = null;
      }

      // Clean up video element
      if (this.videoElement.src) {
        URL.revokeObjectURL(this.videoElement.src);
        this.videoElement.src = "";
      }
    }
  }

  private async tryMSEStream(
    deviceSerial: string,
    baseApiUrl: string = "/api"
  ): Promise<boolean> {
    console.log(
      "[Streaming] tryMSEStream called with deviceSerial:",
      deviceSerial,
      "baseApiUrl:",
      baseApiUrl
    );

    if (!this.videoElement) {
      console.log("[Streaming] No video element, MSE stream failed");
      return false;
    }

    // Check MSE support
    if (
      !window.MediaSource ||
      !MediaSource.isTypeSupported('video/mp4; codecs="avc1.42E01E"')
    ) {
      console.log("[Streaming] MSE not supported, trying other modes");
      return false;
    }

    console.log("[Streaming] MSE is supported, proceeding with setup...");

    try {
      // Clean up any existing MediaSource completely
      if (this.videoElement.src) {
        URL.revokeObjectURL(this.videoElement.src);
        this.videoElement.src = "";
      }

      // Wait a bit to ensure cleanup is complete
      await new Promise((resolve) => setTimeout(resolve, 200));

      const mediaSource = new MediaSource();
      this.videoElement.src = URL.createObjectURL(mediaSource);

      // Store MediaSource reference for cleanup
      (this.videoElement as any)._mediaSource = mediaSource;

      mediaSource.addEventListener("sourceopen", async () => {
        console.log("[Streaming] MSE source opened");
        // MediaSource is ready, proceed immediately
        try {
          console.log("[Streaming] Starting MSE stream setup...");
          await this.setupMSEStream(mediaSource, deviceSerial, baseApiUrl);
          console.log("[Streaming] MSE stream setup completed successfully");
        } catch (error) {
          console.error("[Streaming] MSE setup failed:", error);
          this.cleanupMSE();
          this.onError?.(new Error("MSE setup failed"));
        }
      });

      mediaSource.addEventListener("error", (e) => {
        console.error("[Streaming] MSE MediaSource error:", e);
        this.cleanupMSE();
        this.onError?.(new Error("MSE MediaSource error"));
      });

      this.videoElement.addEventListener("canplay", () => {
        console.log("[Streaming] MSE video can play");
        this.isConnected = true;
        this.onConnectionStateChange?.("connected", "Connected via MSE");
        // Don't set stream start time here, wait for actual play
        // Start buffer monitoring for low latency
        this.startBufferMonitoring();

        // Gentle catch-up on first play
        setTimeout(() => {
          this.performImmediateCatchup();
        }, 500);
      });

      // Set stream start time when video actually starts playing
      this.videoElement.addEventListener("play", () => {
        this.streamStartTime = Date.now();
        console.log("[Streaming] Video started playing, set stream start time");
      });

      this.videoElement.addEventListener("error", (e) => {
        console.error("[Streaming] MSE video error:", e);
        console.error("[Streaming] Video error details:", {
          error: e,
          target: e.target,
          currentTarget: e.currentTarget,
          type: e.type,
          videoError: this.videoElement?.error
            ? {
                code: this.videoElement.error.code,
                message: this.videoElement.error.message,
                MEDIA_ERR_ABORTED: this.videoElement.error.MEDIA_ERR_ABORTED,
                MEDIA_ERR_NETWORK: this.videoElement.error.MEDIA_ERR_NETWORK,
                MEDIA_ERR_DECODE: this.videoElement.error.MEDIA_ERR_DECODE,
                MEDIA_ERR_SRC_NOT_SUPPORTED:
                  this.videoElement.error.MEDIA_ERR_SRC_NOT_SUPPORTED,
              }
            : null,
          videoNetworkState: this.videoElement?.networkState,
          videoReadyState: this.videoElement?.readyState,
          videoSrc: this.videoElement?.src,
          videoCurrentTime: this.videoElement?.currentTime,
          videoDuration: this.videoElement?.duration,
          videoPaused: this.videoElement?.paused,
          videoEnded: this.videoElement?.ended,
        });

        // Log individual error details for easier debugging
        if (this.videoElement?.error) {
          console.error(
            "[Streaming] Video error code:",
            this.videoElement.error.code
          );
          console.error(
            "[Streaming] Video error message:",
            this.videoElement.error.message
          );
          console.error(
            "[Streaming] Video network state:",
            this.videoElement.networkState
          );
          console.error(
            "[Streaming] Video ready state:",
            this.videoElement.readyState
          );
        }
        this.cleanupMSE();
        this.onError?.(new Error("MSE video stream error"));
      });

      return true;
    } catch (error) {
      console.error("[Streaming] MSE setup failed:", error);
      return false;
    }
  }

  private async setupMSEStream(
    mediaSource: MediaSource,
    deviceSerial: string,
    baseApiUrl: string = "/api"
  ): Promise<void> {
    try {
      // Wait for MediaSource to be ready
      if (mediaSource.readyState !== "open") {
        console.log(
          "[Streaming] Waiting for MediaSource to be ready...",
          "state",
          mediaSource.readyState
        );
        await new Promise((resolve, reject) => {
          const timeout = setTimeout(() => {
            reject(new Error("MediaSource ready timeout"));
          }, 10000); // Increased timeout

          const checkReady = () => {
            if (mediaSource.readyState === "open") {
              clearTimeout(timeout);
              resolve(undefined);
            } else if (mediaSource.readyState === "closed") {
              clearTimeout(timeout);
              reject(new Error("MediaSource closed unexpectedly"));
            } else {
              setTimeout(checkReady, 100); // Increased interval
            }
          };
          checkReady();
        });
      }

      // Check if MediaSource already has source buffers and clear them
      if (mediaSource.sourceBuffers.length > 0) {
        console.log(
          "[Streaming] MediaSource already has source buffers, clearing..."
        );

        // Wait for any ongoing updates to complete
        const updatePromises = Array.from(mediaSource.sourceBuffers).map(
          (sb) => {
            if (sb.updating) {
              return new Promise<void>((resolve) => {
                sb.addEventListener("updateend", () => resolve(), {
                  once: true,
                });
              });
            }
            return Promise.resolve();
          }
        );

        await Promise.all(updatePromises);

        // Remove all source buffers
        try {
          while (mediaSource.sourceBuffers.length > 0) {
            const sourceBuffer = mediaSource.sourceBuffers[0];
            mediaSource.removeSourceBuffer(sourceBuffer);
          }
        } catch (error) {
          console.error("[Streaming] Error clearing source buffers:", error);
          // If we can't clear, create a new MediaSource
          this.cleanupMSE();
          this.tryFragmentedMP4(deviceSerial);
          return;
        }
      }

      let sourceBuffer;
      try {
        sourceBuffer = mediaSource.addSourceBuffer(
          'video/mp4; codecs="avc1.42E01E"'
        );
      } catch (error) {
        if (error instanceof Error && error.name === "QuotaExceededError") {
          console.error(
            "[Streaming] MSE SourceBuffer quota exceeded, falling back to Fragmented MP4"
          );
          // Clean up MediaSource before fallback
          this.cleanupMSE();
          // Fallback to Fragmented MP4
          this.tryFragmentedMP4(deviceSerial);
          return;
        }
        throw error;
      }

      // Fetch H.264 stream for MSE
      console.log(
        "[Streaming] Fetching H.264 stream from:",
        `http://localhost:29888/api/stream/video/${deviceSerial}?mode=mse`
      );

      try {
        const response = await fetch(
          `http://localhost:29888/api/stream/video/${deviceSerial}?mode=mse`
        );

        console.log(
          "[Streaming] MSE response status:",
          response.status,
          response.statusText
        );
        console.log(
          "[Streaming] MSE response headers:",
          Object.fromEntries(response.headers.entries())
        );

        if (!response.ok) {
          throw new Error(
            `MSE stream failed: ${response.status} ${response.statusText}`
          );
        }

        const reader = response.body?.getReader();

        if (!reader) {
          throw new Error("No response body reader available");
        }

        console.log(
          "[Streaming] H.264 stream reader created, starting to read data..."
        );

        let buffer = new Uint8Array(0);
        let isFirstChunk = true;
        let hasReceivedData = false;
        let spsPpsSent = false;

        while (true) {
          const { done, value } = await reader.read();
          if (done) {
            console.log("[Streaming] H.264 stream reader finished");
            break;
          }

          console.log(
            "[Streaming] Received H.264 data chunk:",
            value.length,
            "bytes"
          );

          // Append to buffer
          const newBuffer = new Uint8Array(buffer.length + value.length);
          newBuffer.set(buffer);
          newBuffer.set(value, buffer.length);
          buffer = newBuffer;

          // Wait for source buffer to be ready
          while (sourceBuffer.updating) {
            await new Promise((resolve) => {
              sourceBuffer.addEventListener("updateend", resolve, {
                once: true,
              });
            });
          }

          try {
            // Check if sourceBuffer is still valid and MediaSource is still open
            if (!sourceBuffer) {
              console.log("[Streaming] SourceBuffer invalid, stopping append");
              return;
            }

            if (mediaSource.readyState !== "open") {
              console.log(
                "[Streaming] MediaSource not open, waiting...",
                "state",
                mediaSource.readyState
              );
              // Wait a bit longer for MediaSource to be ready
              await new Promise((resolve) => setTimeout(resolve, 200));
              // Check again after waiting
              if ((mediaSource.readyState as string) !== "open") {
                console.log(
                  "[Streaming] MediaSource still not open after waiting, skipping this chunk",
                  "state",
                  mediaSource.readyState
                );
                return;
              }
            }

            // Process first chunk immediately
            if (isFirstChunk) {
              isFirstChunk = false;
            }

            // Append MP4 fragment data to SourceBuffer
            if (buffer.length > 0) {
              console.log(
                "[Streaming] Appending MP4 fragment to SourceBuffer:",
                buffer.length,
                "bytes"
              );

              try {
                sourceBuffer.appendBuffer(buffer);
                hasReceivedData = true;
                console.log(
                  "[Streaming] Successfully appended MP4 fragment to SourceBuffer"
                );
                buffer = new Uint8Array(0); // Clear buffer after successful append
              } catch (e) {
                console.error("[Streaming] Failed to append MP4 fragment:", e);
                // Continue with next chunk
              }
            }
          } catch (e) {
            console.error("[Streaming] MSE append error:", e);
            if (e instanceof Error && e.name === "InvalidStateError") {
              if (e.message.includes("removed")) {
                console.log(
                  "[Streaming] SourceBuffer removed, stopping append"
                );
                return;
              }
              if (mediaSource.readyState === "closed") {
                console.log("[Streaming] MediaSource closed, stopping append");
                return;
              }
              console.log("[Streaming] InvalidStateError, waiting...", e);
              // Wait longer for InvalidStateError
              await new Promise((resolve) => setTimeout(resolve, 200));
            } else if (e instanceof Error && e.name === "QuotaExceededError") {
              console.error(
                "[Streaming] SourceBuffer quota exceeded, falling back to Fragmented MP4"
              );
              this.cleanupMSE();
              this.tryFragmentedMP4(deviceSerial);
              return;
            } else {
              console.log("[Streaming] Buffer append failed, waiting...", e);
              // Wait longer for other errors
              await new Promise((resolve) => setTimeout(resolve, 100));
            }
          }
        }

        // Wait for final updates to complete
        while (sourceBuffer.updating) {
          await new Promise((resolve) => {
            sourceBuffer.addEventListener("updateend", resolve, { once: true });
          });
        }

        // Only end stream if we received data and MediaSource is still open
        if (hasReceivedData && mediaSource.readyState === "open") {
          try {
            mediaSource.endOfStream();
          } catch (e) {
            console.warn("[Streaming] Failed to end MediaSource stream:", e);
          }
        }
      } catch (fetchError) {
        console.error("[Streaming] MSE fetch error:", fetchError);
        throw fetchError;
      }
    } catch (error) {
      console.error("[Streaming] MSE stream error:", error);
      // Clean up MediaSource before fallback
      this.cleanupMSE();
      // Try fallback to Fragmented MP4
      console.log(
        "[Streaming] Falling back to Fragmented MP4 due to MSE error"
      );
      this.tryFragmentedMP4(deviceSerial);
    }
  }

  private tryWebMStream(deviceSerial: string): boolean {
    if (!this.videoElement) return false;

    try {
      // Check if WebM is supported
      if (!this.videoElement.canPlayType('video/webm; codecs="vp8"')) {
        console.log("[Streaming] WebM not supported, trying other modes");
        return false;
      }

      // Set up WebM stream with aggressive low-latency settings
      this.videoElement.src = `/api/stream/video/${deviceSerial}?mode=webm`;

      // Set aggressive low-latency attributes
      this.videoElement.preload = "none";
      this.videoElement.autoplay = true;
      this.videoElement.muted = false; // Enable audio playback
      this.videoElement.playsInline = true;

      // Set low-latency buffer settings (webkitVideoDecodedByteCount is read-only)
      // Just ensure the video element is ready for low-latency playback

      this.videoElement.load();

      this.videoElement.addEventListener("canplay", () => {
        console.log("[Streaming] WebM can play");
        this.isConnected = true;
        this.onConnectionStateChange?.(
          "connected",
          "Connected via WebM streaming"
        );

        // Start playing immediately for low latency
        this.videoElement?.play().catch((e) => {
          console.warn("[Streaming] WebM autoplay failed:", e);
        });
      });

      this.videoElement.addEventListener("error", (e) => {
        console.error("[Streaming] WebM error:", e);
        this.onError?.(new Error("WebM stream error"));
      });

      return true;
    } catch (error) {
      console.error("[Streaming] WebM setup failed:", error);
      return false;
    }
  }

  private tryFragmentedMP4(deviceSerial: string): boolean {
    if (!this.videoElement) return false;

    try {
      // Set up fragmented MP4 stream
      this.videoElement.src = `/api/stream/video/${deviceSerial}?mode=fmp4`;
      this.videoElement.load();

      this.videoElement.addEventListener("canplay", () => {
        console.log("[Streaming] Fragmented MP4 can play");
        this.isConnected = true;
        this.onConnectionStateChange?.(
          "connected",
          "Connected via Fragmented MP4"
        );
      });

      this.videoElement.addEventListener("error", (e) => {
        console.error("[Streaming] Fragmented MP4 error:", e);
        this.onError?.(new Error("Fragmented MP4 stream error"));
      });

      return true;
    } catch (error) {
      console.error("[Streaming] Fragmented MP4 setup failed:", error);
      return false;
    }
  }

  private tryRegularMP4(deviceSerial: string): void {
    if (!this.videoElement) return;

    // Set up regular MP4 stream (fallback)
    this.videoElement.src = `/api/stream/video/${deviceSerial}?mode=ffmpeg`;
    this.videoElement.load();

    this.videoElement.addEventListener("loadstart", () => {
      console.log("[Streaming] Regular MP4 stream started");
    });

    this.videoElement.addEventListener("error", (e) => {
      console.error("[Streaming] Regular MP4 stream error:", e);
      this.onError?.(new Error("Regular MP4 stream error"));
    });

    this.videoElement.addEventListener("canplay", () => {
      console.log("[Streaming] Regular MP4 can play");
      this.isConnected = true;
      this.onConnectionStateChange?.("connected", "Connected via Regular MP4");

      // Update stats
      this.onStatsUpdate?.({
        resolution: `${this.videoElement!.videoWidth}x${
          this.videoElement!.videoHeight
        }`,
        fps: 0, // TODO: Calculate FPS
        latency: 0, // TODO: Calculate latency
      });
    });

    this.videoElement.addEventListener("loadedmetadata", () => {
      console.log("[Streaming] Regular MP4 metadata loaded");
      // Set connected state when metadata is available
      this.isConnected = true;
      this.onConnectionStateChange?.("connected", "Connected via Regular MP4");
    });
  }

  async disconnect(): Promise<void> {
    if (!this.isConnected || !this.currentDevice) {
      return;
    }

    console.log("[Streaming] Disconnecting from", this.currentDevice);
    this.isManuallyDisconnected = true; // Mark as manually disconnected
    this.shouldStopReconnecting = true; // Stop any ongoing reconnection attempts
    this.onConnectionStateChange?.("disconnected", "Disconnecting...");

    try {
      // Stop video stream and clean up MediaSource
      if (this.videoElement) {
        // Clean up MediaSource if it exists
        if (
          this.videoElement.src &&
          this.videoElement.src.startsWith("blob:")
        ) {
          URL.revokeObjectURL(this.videoElement.src);
        }
        this.videoElement.src = "";
        this.videoElement.load();
      }

      // Close control WebSocket
      if (this.ws) {
        this.ws.close();
        this.ws = null;
      }

      // Call disconnect API
      await fetch(`/api/stream/${this.currentDevice}/disconnect`, {
        method: "POST",
      });

      this.currentDevice = null;
      this.isConnected = false;
      this.onConnectionStateChange?.("disconnected", "Disconnected");
    } catch (error) {
      console.error("[Streaming] Disconnect failed:", error);
      this.onError?.(error as Error);
    }
  }

  sendControlMessage(message: ControlMessage | any): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn("[Streaming] Control WebSocket not ready");
      return;
    }

    try {
      this.ws.send(JSON.stringify(message));
    } catch (error) {
      console.error("[Streaming] Failed to send control message:", error);
    }
  }

  // Touch event methods (similar to WebRTC client)
  sendTouchEvent(
    action: string,
    x: number,
    y: number,
    pressure: number = 1.0,
    pointerId: number = 0
  ): void {
    // Ignore touches outside video area
    if (x < 0 || x > 1 || y < 0 || y > 1) {
      console.log(
        `[Streaming] Touch outside video area ignored: x=${x.toFixed(
          3
        )}, y=${y.toFixed(3)}`
      );
      return;
    }

    const message: ControlMessage = {
      type: "touch",
      action,
      x,
      y,
      pressure,
      pointerId,
    };

    this.sendControlMessage(message);
  }

  sendKeyEvent(keycode: number, action: string, metaState: number = 0): void {
    const message: ControlMessage = {
      type: "key",
      action,
      keycode,
      metaState,
    };

    this.sendControlMessage(message);
  }

  sendScrollEvent(
    x: number,
    y: number,
    hScroll: number,
    vScroll: number
  ): void {
    const message: ControlMessage = {
      type: "scroll",
      x,
      y,
      hScroll,
      vScroll,
    };

    this.sendControlMessage(message);
  }

  requestKeyframe(): void {
    // Temporarily disabled to prevent Video capture reset
    console.log(
      "[Streaming] requestKeyframe called but disabled to prevent Video capture reset"
    );
    return;

    const message: ControlMessage = {
      type: "reset_video",
    };

    this.sendControlMessage(message);
  }

  sendClipboardText(text: string, paste: boolean = false): void {
    const message: ControlMessage = {
      type: "clipboard_set",
      text,
      paste,
    };

    this.sendControlMessage(message);
  }

  // Mouse event handlers (compatible with useMouseHandler)
  handleMouseEvent(event: MouseEvent, action: string): void {
    if (!this.videoElement || !this.isConnected) return;

    const rect = this.videoElement.getBoundingClientRect();
    const x = (event.clientX - rect.left) / rect.width;
    const y = (event.clientY - rect.top) / rect.height;

    // Convert mouse action to touch action
    const touchAction =
      action === "down" ? "down" : action === "up" ? "up" : "move";

    this.sendTouchEvent(touchAction, x, y, 1.0, 0);

    // Update dragging state
    if (action === "down") {
      this.isMouseDragging = true;
    } else if (action === "up") {
      this.isMouseDragging = false;
    }
  }

  handleTouchEvent(event: TouchEvent, action: string): void {
    if (!this.videoElement || !this.isConnected) return;

    const rect = this.videoElement.getBoundingClientRect();
    const touch = event.touches[0] || event.changedTouches[0];

    if (!touch) return;

    const x = (touch.clientX - rect.left) / rect.width;
    const y = (touch.clientY - rect.top) / rect.height;
    const pressure = touch.force || 1.0;
    const pointerId = touch.identifier || 0;

    this.sendTouchEvent(action, x, y, pressure, pointerId);
  }

  private scheduleReconnect(deviceSerial: string): void {
    if (
      this.isReconnecting ||
      this.shouldStopReconnecting ||
      this.isManuallyDisconnected
    ) {
      return;
    }

    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log(
        "[Streaming] Max reconnection attempts reached, stopping reconnection"
      );
      this.onConnectionStateChange?.(
        "error",
        "Max reconnection attempts reached"
      );
      this.shouldStopReconnecting = true;
      return;
    }

    this.isReconnecting = true;
    this.reconnectAttempts++;

    // Exponential backoff with jitter to avoid thundering herd
    const baseDelay = 1000 * Math.pow(2, this.reconnectAttempts - 1);
    const jitter = Math.random() * 1000; // Add up to 1 second of jitter
    const delay = Math.min(baseDelay + jitter, 30000); // Max 30 seconds

    console.log(
      `[Streaming] Scheduling reconnection attempt ${this.reconnectAttempts}/${
        this.maxReconnectAttempts
      } in ${Math.round(delay)}ms`
    );

    this.reconnectTimer = window.setTimeout(async () => {
      if (this.shouldStopReconnecting || this.isManuallyDisconnected) {
        this.isReconnecting = false;
        return;
      }

      try {
        console.log(
          `[Streaming] Attempting reconnection ${this.reconnectAttempts}/${this.maxReconnectAttempts}`
        );
        this.onConnectionStateChange?.(
          "connecting",
          `Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`
        );

        await this.connect(deviceSerial, this.lastBaseApiUrl || "/api");
        this.reconnectAttempts = 0; // Reset on successful connection
        this.isReconnecting = false;
      } catch (error) {
        console.error(
          `[Streaming] Reconnection attempt ${this.reconnectAttempts} failed:`,
          error
        );
        this.isReconnecting = false;

        // Check if this is a 404 error (device not found) and stop retrying
        if (error instanceof Error && error.message.includes("404")) {
          console.log(
            "[Streaming] Device not found (404), stopping reconnection attempts"
          );
          this.shouldStopReconnecting = true;
          this.onConnectionStateChange?.(
            "error",
            "Device not found or not available"
          );
          return;
        }

        // Schedule next attempt only if we haven't exceeded max attempts
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.scheduleReconnect(deviceSerial);
        }
      }
    }, delay);
  }

  private startBufferMonitoring(): void {
    if (this.bufferMonitorInterval) {
      clearInterval(this.bufferMonitorInterval);
    }

    this.bufferMonitorInterval = window.setInterval(() => {
      this.monitorBufferDelay();
    }, 200); // Check every 200ms for smooth catch-up
  }

  private stopBufferMonitoring(): void {
    if (this.bufferMonitorInterval) {
      clearInterval(this.bufferMonitorInterval);
      this.bufferMonitorInterval = null;
    }
  }

  private performImmediateCatchup(): void {
    if (!this.videoElement || this.videoElement.paused) {
      return;
    }

    try {
      const buffered = this.videoElement.buffered;
      if (buffered.length > 0) {
        const bufferedEnd = buffered.end(buffered.length - 1);
        const currentTime = this.videoElement.currentTime;
        const delay = bufferedEnd - currentTime;

        if (delay > 0.05) {
          // If there's any delay at all
          console.log(
            `[Streaming] Performing immediate catch-up: ${delay.toFixed(
              2
            )}s delay, jumping to ${bufferedEnd.toFixed(2)}s`
          );
          this.videoElement.currentTime = bufferedEnd - 0.01; // Jump to very end
        }
      }
    } catch (error) {
      console.warn("[Streaming] Error performing immediate catch-up:", error);
    }
  }

  private monitorBufferDelay(): void {
    if (!this.videoElement || this.videoElement.paused) {
      return;
    }

    try {
      const buffered = this.videoElement.buffered;
      if (buffered.length > 0) {
        const bufferedEnd = buffered.end(buffered.length - 1);
        const delay = bufferedEnd - this.videoElement.currentTime;

        // Aggressive catch-up is the most reliable way to reduce latency
        if (delay > 0.2) {
          // If buffer is more than 200ms
          console.log(
            `[Streaming] High buffer delay (${delay.toFixed(
              2
            )}s). Catching up...`
          );
          this.videoElement.currentTime = bufferedEnd - 0.01; // Jump to the end
        }

        // Keep playback rate constant for a smoother experience
        this.videoElement.playbackRate = 1.0;
      }
    } catch (error) {
      console.warn("[Streaming] Error monitoring buffer delay:", error);
    }
  }

  cleanup(): void {
    this.shouldStopReconnecting = true;

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    // Stop buffer monitoring
    this.stopBufferMonitoring();

    this.disconnect();
  }
}
