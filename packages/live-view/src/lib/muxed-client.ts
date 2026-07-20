import { ControlClient, ClientOptions, ConnectionState, Stats } from "./types";

/**
 * MP4 Client for handling Fragment MP4 (fMP4) container streams with H.264 video and AAC audio
 * Uses MediaSource API for native MP4 playback with low latency
 */
export class MP4Client implements ControlClient {
  private mediaSource: MediaSource | null = null;
  private sourceBuffer: SourceBuffer | null = null;
  private videoElement: HTMLVideoElement | null = null;
  private isConnected = false;
  private streamUrl: string = "";
  private controlWs: WebSocket | null = null;
  private isControlConnectedFlag = false;
  public isMouseDragging = false;
  private selectedMimeType: string = "";
  private appendQueue: Uint8Array[] = [];
  private pending: Uint8Array = new Uint8Array(0);
  private pendingMoof: Uint8Array | null = null;
  private earlyMediaBoxes: Uint8Array[] = [];
  // Target live latency controller (seconds)
  private targetLagSec: number = 1.0;
  private minPlaybackRate: number = 0.97;
  private maxPlaybackRate: number = 1.05; // gentle range to avoid artifacts
  private hasStartedPlayback: boolean = false;
  // Coalescing appends to reduce MSE update churn
  private coalesceQueue: Uint8Array[] = [];
  private coalesceBytes: number = 0;
  private coalesceTimer: number | null = null;
  private coalesceMaxDelayMs: number = 60;
  private coalesceMaxBytes: number = 512 * 1024;
  private coalesceMaxPairs: number = 3;

  // HTTP request management
  private abortController: AbortController | null = null;

  // Callback functions
  private onConnectionStateChange?: (
    state: ConnectionState,
    message?: string
  ) => void;
  private onError?: (error: Error) => void;
  private onStatsUpdate?: (stats: Stats) => void;

  constructor(options: ClientOptions = {}) {
    this.onConnectionStateChange = options.onConnectionStateChange;
    this.onError = options.onError;
    this.onStatsUpdate = options.onStatsUpdate;
  }

  /**
   * Connect to Fragment MP4 stream
   */
  async connect(
    deviceSerial: string,
    apiUrl: string,
    wsUrl?: string,
    forceReconnect: boolean = false
  ): Promise<void> {
    console.log(
      `[MP4Client] connect called with deviceSerial: ${deviceSerial}, apiUrl: ${apiUrl}, wsUrl: ${wsUrl}, forceReconnect: ${forceReconnect}`
    );

    if (this.isConnected && !forceReconnect) {
      console.log("[MP4Client] Already connected, returning");
      return;
    }

    // If force reconnect, disconnect first
    if (this.isConnected && forceReconnect) {
      console.log("[MP4Client] Force reconnect requested, disconnecting first");

      // Cancel any ongoing streaming first
      if (this.abortController) {
        console.log(
          "[MP4Client] Aborting ongoing HTTP request before disconnect"
        );
        this.abortController.abort();
        this.abortController = null;
      }

      this.disconnect();

      // Wait longer for cleanup to complete, especially DOM element removal and HTTP request cancellation
      await new Promise((resolve) => setTimeout(resolve, 500));

      // Reset internal state to ensure clean start
      this.sourceBuffer = null;
      this.selectedMimeType = "";
      this.earlyMediaBoxes = [];
      this.pending = new Uint8Array();
      this.hasStartedPlayback = false;

      console.log(
        "[MP4Client] Force reconnect cleanup completed, starting new connection"
      );
    }

    this.streamUrl = `${apiUrl}/devices/${deviceSerial}/stream?codec=h264%2Baac&format=mp4`;
    console.log(
      `[MP4Client] Connecting to Fragment MP4 stream: ${this.streamUrl}`
    );

    try {
      // Check MediaSource support
      if (!this.checkMediaSourceSupport()) {
        throw new Error(
          "MediaSource API not supported or MP4 codec not supported"
        );
      }

      // Connect control WebSocket first (before starting streaming)
      if (wsUrl) {
        console.log(
          `[MP4Client] Connecting control WebSocket with wsUrl: ${wsUrl}`
        );
        await this.connectControl(deviceSerial, wsUrl);
        console.log("[MP4Client] Control WebSocket connection completed");
      } else {
        console.warn(
          "[MP4Client] No wsUrl provided, control WebSocket will not be connected"
        );
      }

      console.log("[MP4Client] Setting up MediaSource...");
      await this.setupMediaSource();
      console.log("[MP4Client] MediaSource setup completed");

      console.log("[MP4Client] Starting streaming...");

      // Start streaming in background (don't await it as it runs forever)
      this.startStreaming().catch((error) => {
        console.error("[MP4Client] Streaming error:", error);
        this.onError?.(error);
      });

      // Set connected state immediately after starting the stream
      this.isConnected = true;

      // Notify connection success
      console.log(
        "[MP4Client] Calling onConnectionStateChange with 'connected'"
      );
      this.onConnectionStateChange?.("connected", "MP4 stream connected");
    } catch (error) {
      console.error("[MP4Client] Failed to connect:", error);
      this.onConnectionStateChange?.("error", "MP4 connection failed");
      this.onError?.(error as Error);
      this.cleanup();
      throw error;
    }
  }

  /**
   * Disconnect from MP4 stream
   */
  disconnect(): void {
    if (!this.isConnected) {
      return;
    }

    // Cancel any ongoing HTTP requests first
    if (this.abortController) {
      console.log("[MP4Client] Aborting ongoing HTTP request");
      this.abortController.abort();
      this.abortController = null;
    }

    // Close control WebSocket first
    if (this.controlWs) {
      try {
        this.controlWs.close();
      } catch (error) {
        console.warn("[MP4Client] Error closing WebSocket:", error);
      }
      this.controlWs = null;
      this.isControlConnectedFlag = false;
    }

    // Clean up other resources
    this.cleanup();

    // Update connection state
    this.isConnected = false;
    this.onConnectionStateChange?.("disconnected", "MP4 stream disconnected");
  }

  /**
   * Check if MediaSource API supports MP4 with H.264 and AAC
   */
  private checkMediaSourceSupport(): boolean {
    if (!("MediaSource" in window)) {
      console.error("[MP4Client] MediaSource API not supported");
      return false;
    }

    const codecOptions = [
      'video/mp4; codecs="avc1.42E01E,mp4a.40.2"',
      'video/mp4; codecs="avc1.640028,mp4a.40.2"',
      'video/mp4; codecs="avc1.4D401F,mp4a.40.2"',
    ];

    for (const mimeType of codecOptions) {
      if (MediaSource.isTypeSupported(mimeType)) {
        return true;
      }
    }

    console.error("[MP4Client] No supported MP4/WebM codec found");
    return false;
  }

  /**
   * Setup MediaSource and video element
   */
  private async setupMediaSource(): Promise<void> {
    // Create MediaSource
    this.mediaSource = new MediaSource();

    // Create video element
    this.videoElement = document.createElement("video");
    this.videoElement.src = URL.createObjectURL(this.mediaSource);
    this.videoElement.autoplay = true;
    this.videoElement.muted = true; // Start muted to bypass autoplay restrictions
    this.videoElement.playsInline = true; // Enable inline playback on mobile
    this.videoElement.controls = false; // Hide controls for clean UI
    this.videoElement.loop = false; // Don't loop the stream
    this.videoElement.preload = "auto"; // Preload video data
    this.videoElement.style.width = "100%";
    this.videoElement.style.height = "100%";
    this.videoElement.style.objectFit = "contain";
    this.videoElement.style.display = "block";
    this.videoElement.style.margin = "auto";

    this.videoElement.addEventListener("loadedmetadata", () => {
      if (this.videoElement) {
        // Keep muted initially to ensure autoplay works
        console.log("[MP4Client] Video metadata loaded, attempting to play");
        this.videoElement.play().catch((error) => {
          console.warn("[MP4Client] Initial play failed:", error);
          // Try again after a short delay
          setTimeout(() => {
            if (this.videoElement) {
              this.videoElement.play().catch((retryError) => {
                console.warn("[MP4Client] Retry play failed:", retryError);
              });
            }
          }, 100);
        });
        // Update stats when metadata is loaded
        this.updateStats();
      }
      window.dispatchEvent(new Event("resize"));
    });

    this.videoElement.addEventListener("canplay", () => {
      if (this.videoElement) {
        console.log("[MP4Client] Video can play, ensuring playback");
        this.videoElement.play().catch((error) => {
          console.warn("[MP4Client] Canplay play failed:", error);
        });
      }
      this.hasStartedPlayback = true;
    });

    this.videoElement.addEventListener("canplaythrough", () => {
      if (this.videoElement) {
        console.log("[MP4Client] Video can play through, ensuring playback");
        this.videoElement.play().catch((error) => {
          console.warn("[MP4Client] Canplaythrough play failed:", error);
        });
      }
    });

    this.videoElement.addEventListener("waiting", () => {
      this.handleVideoStall();
    });

    this.videoElement.addEventListener("resize", () => {
      window.dispatchEvent(new Event("resize"));
    });

    // Add user interaction handler to enable audio
    this.addUserInteractionHandler();

    // Add video element to the page
    await this.addVideoElementToPage();

    // Wait for MediaSource to be ready (defer SourceBuffer creation until init parsed)
    await this.waitForMediaSourceReady();
  }

  /**
   * Add user interaction handler to enable audio
   */
  private addUserInteractionHandler(): void {
    let hasUserInteracted = false;
    
    const enableAudio = () => {
      if (hasUserInteracted || !this.videoElement) return;
      hasUserInteracted = true;
      
      console.log("[MP4Client] User interaction detected, enabling audio");
      this.videoElement.muted = false;
      
      // Remove event listeners after first interaction
      document.removeEventListener("click", enableAudio);
      document.removeEventListener("touchstart", enableAudio);
      document.removeEventListener("keydown", enableAudio);
    };
    
    // Listen for user interactions
    document.addEventListener("click", enableAudio, { once: true });
    document.addEventListener("touchstart", enableAudio, { once: true });
    document.addEventListener("keydown", enableAudio, { once: true });
  }

  /**
   * Add video element to the page
   */
  private async addVideoElementToPage(): Promise<void> {
    if (!this.videoElement) {
      throw new Error("Video element not created");
    }

    const videoContainer = this.findVideoContainer();
    if (videoContainer) {
      videoContainer.innerHTML = "";
      videoContainer.appendChild(this.videoElement);
    } else {
      document.body.appendChild(this.videoElement);
    }
  }

  /**
   * Find the appropriate video container
   */
  private findVideoContainer(): HTMLElement | null {
    // Try different selectors to find the video container
    const selectors = [
      "#video-mp4-container", // Stable id from AndroidLiveView for muxed mode
      ".video-container",
      "#video-canvas-container",
      "#video-canvas",
      ".video-wrapper",
      ".video-main-area",
    ];

    for (const selector of selectors) {
      const element = document.querySelector(selector) as HTMLElement;
      if (element) {
        return element;
      }
    }

    return null;
  }

  /**
   * Wait for MediaSource to be ready and create SourceBuffer
   */
  private waitForMediaSourceReady(): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      if (!this.mediaSource) {
        reject(new Error("MediaSource not created"));
        return;
      }

      this.mediaSource.addEventListener("sourceopen", () => {
        try {
          (this.mediaSource as MediaSource & { duration: number }).duration =
            Infinity;
        } catch {
          // Ignore errors setting duration
        }
        resolve();
      });

      this.mediaSource.addEventListener("error", (e) => {
        console.error("[MP4Client] MediaSource error:", e);
        reject(new Error(`MediaSource error: ${e}`));
      });

      // Timeout after 5 seconds
      setTimeout(() => {
        reject(new Error("MediaSource setup timeout"));
      }, 5000);
    });
  }

  /**
   * Start streaming MP4 data
   */
  private async startStreaming(): Promise<void> {
    if (!this.mediaSource) {
      throw new Error("MediaSource not ready");
    }

    // Check MediaSource state
    if (this.mediaSource.readyState !== "open") {
      throw new Error(
        `MediaSource not open, state: ${this.mediaSource.readyState}`
      );
    }

    // Create new AbortController for this request
    this.abortController = new AbortController();

    const response = await fetch(this.streamUrl, {
      signal: this.abortController.signal,
    });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body reader available");
    }

    const initBoxes: Uint8Array[] = [];
    let haveInit = false;

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }

        if (!this.mediaSource || this.mediaSource.readyState !== "open") {
          break;
        }

        // If SourceBuffer not yet created, parse for complete ftyp+moov init
        if (!this.sourceBuffer) {
          this.pending = MP4Client.concat(this.pending, value);
          const boxes = this.extractCompleteBoxesFromPending();
          for (const box of boxes) {
            const t = MP4Client.readType(box, 4);
            if (t === "ftyp" || t === "moov") {
              initBoxes.push(box);
            } else if (t === "moof" || t === "mdat") {
              // Stash media boxes seen before SourceBuffer is created
              this.earlyMediaBoxes.push(box);
            }
          }
          // when we have both ftyp and moov, create SB with precise codecs and append init only
          if (
            !haveInit &&
            initBoxes.some((b) => MP4Client.readType(b, 4) === "ftyp") &&
            initBoxes.some((b) => MP4Client.readType(b, 4) === "moov")
          ) {
            const ftyp = initBoxes.find(
              (b) => MP4Client.readType(b, 4) === "ftyp"
            );
            const moov = initBoxes.find(
              (b) => MP4Client.readType(b, 4) === "moov"
            );
            if (!ftyp || !moov) continue;
            const init = new Uint8Array(ftyp.length + moov.length);
            init.set(ftyp, 0);
            init.set(moov, ftyp.length);
            const parsedAvc1 = MP4Client.extractAvc1FromInit(init);
            if (!parsedAvc1) {
              continue;
            }
            if (!this.selectedMimeType) {
              this.selectedMimeType = `video/mp4; codecs="${parsedAvc1},mp4a.40.2"`;
            }

            // Check if MediaSource is still open before adding SourceBuffer
            if (!this.mediaSource || this.mediaSource.readyState !== "open") {
              console.warn(
                "[MP4Client] MediaSource not ready for SourceBuffer creation"
              );
              continue;
            }

            try {
              console.log(
                "[MP4Client] Creating SourceBuffer with MIME type:",
                this.selectedMimeType
              );
              console.log(
                "[MP4Client] MediaSource state:",
                this.mediaSource.readyState
              );
              console.log(
                "[MP4Client] MIME type supported:",
                MediaSource.isTypeSupported(this.selectedMimeType)
              );

              const buffer = this.mediaSource.addSourceBuffer(
                this.selectedMimeType
              );
              // Use segments mode for better control over timestamps
              try {
                (buffer as unknown as { mode?: string }).mode = "segments";
              } catch {
                // Ignore if mode setting fails
              }
              this.sourceBuffer = buffer;
              this.sourceBuffer.addEventListener("updateend", () =>
                this.onUpdateEnd()
              );
              this.sourceBuffer.addEventListener("error", (ev) => {
                console.error("[MP4Client] SourceBuffer error", ev);
              });
              // append init only
              this.enqueueAppend(init);
              haveInit = true;

              // Drain any early media boxes captured before SB creation
              if (this.earlyMediaBoxes.length > 0) {
                // Try to pair moof+mdat in order
                let i = 0;
                while (i < this.earlyMediaBoxes.length) {
                  const cur = this.earlyMediaBoxes[i];
                  const curType = MP4Client.readType(cur, 4);
                  if (
                    curType === "moof" &&
                    i + 1 < this.earlyMediaBoxes.length
                  ) {
                    const next = this.earlyMediaBoxes[i + 1];
                    const nextType = MP4Client.readType(next, 4);
                    if (nextType === "mdat") {
                      const merged = new Uint8Array(cur.length + next.length);
                      merged.set(cur, 0);
                      merged.set(next, cur.length);
                      this.enqueueAppend(merged);
                      i += 2;
                      continue;
                    }
                  }
                  // Fallback: if mdat and we had pendingMoof captured earlier
                  if (curType === "mdat" && this.pendingMoof) {
                    const merged = new Uint8Array(
                      this.pendingMoof.length + cur.length
                    );
                    merged.set(this.pendingMoof, 0);
                    merged.set(cur, this.pendingMoof.length);
                    this.enqueueAppend(merged);
                    this.pendingMoof = null;
                    i += 1;
                    continue;
                  }
                  // If a standalone moof without following mdat, keep as pending
                  if (curType === "moof") {
                    this.pendingMoof = cur;
                  }
                  i += 1;
                }
                this.earlyMediaBoxes = [];
              }
            } catch (e) {
              console.error("[MP4Client] addSourceBuffer failed", e, {
                mimeType: this.selectedMimeType,
                mediaSourceReadyState: this.mediaSource?.readyState,
                hasMediaSource: !!this.mediaSource,
              });
              throw e;
            }
          }
          continue;
        }

        this.pending = MP4Client.concat(this.pending, value);
        const boxes = this.extractCompleteBoxesFromPending();
        // Only append valid sequences: init boxes directly; moof must be paired with following mdat
        for (const box of boxes) {
          const type = MP4Client.readType(box, 4);
          if (type === "ftyp" || type === "moov" || type === "sidx") {
            this.enqueueAppend(box);
            continue;
          }
          if (type === "moof") {
            // hold until we get the subsequent mdat (can arrive later)
            this.pendingMoof = box;
            continue;
          }
          if (type === "mdat") {
            if (this.pendingMoof) {
              const merged = new Uint8Array(
                this.pendingMoof.length + box.length
              );
              merged.set(this.pendingMoof, 0);
              merged.set(box, this.pendingMoof.length);
              this.enqueueAppend(merged);
              this.pendingMoof = null;
            } else {
              // mdat without moof is not useful, skip
            }
            continue;
          }
          // Unknown box types are ignored
        }
      }

      // End of stream
      if (this.sourceBuffer && this.sourceBuffer.updating) {
        await new Promise<void>((resolve) => {
          const sb = this.sourceBuffer as SourceBuffer;
          const onEnd = () => {
            sb.removeEventListener("updateend", onEnd);
            resolve();
          };
          sb.addEventListener("updateend", onEnd, { once: true });
        });
      }

      if (this.mediaSource && this.mediaSource.readyState === "open") {
        this.mediaSource.endOfStream();
      }
    } catch (error) {
      // Don't treat AbortError as a real error - it's expected when disconnecting
      if (error instanceof Error && error.name === "AbortError") {
        console.log("[MP4Client] Stream aborted (expected during disconnect)");
        return;
      }
      console.error("[MP4Client] Streaming error:", error);
      throw error;
    }
  }

  private onUpdateEnd() {
    if (!this.sourceBuffer) return;
    // Trim buffered ranges to avoid QUOTA_EXCEEDED
    try {
      this.maybeTrimBuffer();
    } catch (e) {
      console.warn("[MP4Client] trim buffer failed", e);
    }
    this.monitorBufferHealth();
    if (this.sourceBuffer && !this.sourceBuffer.updating) {
      // If we have pending chunks, coalesce small moof+mdat pairs to reduce update frequency
      while (
        this.appendQueue.length > 0 &&
        this.coalesceQueue.length < this.coalesceMaxPairs
      ) {
        const next = this.appendQueue.shift();
        if (!next) break;
        this.coalesceQueue.push(next);
        this.coalesceBytes += next.byteLength;
        if (this.coalesceBytes >= this.coalesceMaxBytes) break;
      }

      if (this.coalesceQueue.length > 0) {
        const flush = () => {
          if (!this.sourceBuffer || this.sourceBuffer.updating) return;
          const merged = MP4Client.concat(
            new Uint8Array(0),
            this.coalesceQueue.reduce(
              (acc, cur) => MP4Client.concat(acc, cur),
              new Uint8Array(0)
            )
          );
          this.coalesceQueue = [];
          this.coalesceBytes = 0;
          this.coalesceTimer = null;
          try {
            this.sourceBuffer.appendBuffer(merged as unknown as BufferSource);
            if (this.videoElement && this.videoElement.paused) {
              console.log("[MP4Client] Video paused, attempting to play after buffer append");
              this.videoElement.play().catch((error) => {
                console.warn("[MP4Client] Play after buffer append failed:", error);
              });
            }
          } catch (e) {
            console.error("[MP4Client] appendBuffer (coalesced) failed", e);
            if (this.videoElement) {
              console.log("[MP4Client] Attempting to play after append error");
              this.videoElement.play().catch((error) => {
                console.warn("[MP4Client] Play after append error failed:", error);
              });
            }
          }
        };

        if (this.coalesceTimer) {
          clearTimeout(this.coalesceTimer);
          this.coalesceTimer = null;
        }
        // Flush immediately if large enough, else wait briefly to coalesce pairs
        if (
          this.coalesceBytes >= this.coalesceMaxBytes ||
          this.coalesceQueue.length >= this.coalesceMaxPairs
        ) {
          flush();
        } else {
          this.coalesceTimer = setTimeout(flush, this.coalesceMaxDelayMs);
        }
      }
    }
  }

  private enqueueAppend(chunk: Uint8Array) {
    if (!this.sourceBuffer) return;
    if (
      this.sourceBuffer &&
      !this.sourceBuffer.updating &&
      this.appendQueue.length === 0
    ) {
      try {
        this.sourceBuffer.appendBuffer(chunk as unknown as BufferSource);
        // Try to play after appending data
        if (this.videoElement && this.videoElement.paused) {
          console.log("[MP4Client] Video paused, attempting to play after immediate append");
          this.videoElement.play().catch((error) => {
            console.warn("[MP4Client] Play after immediate append failed:", error);
          });
        }
        return;
      } catch (e) {
        console.warn("[MP4Client] append immediate failed, queueing", e);
      }
    }
    this.appendQueue.push(chunk);
  }

  private maybeTrimBuffer() {
    if (!this.sourceBuffer || !this.videoElement) return;
    const sb = this.sourceBuffer as SourceBuffer;
    const v = this.videoElement as HTMLVideoElement;
    if (sb.buffered.length === 0) return;

    if (sb.buffered.length > 1) {
      return;
    }

    const start = sb.buffered.start(0);
    const end = sb.buffered.end(sb.buffered.length - 1);
    const maxWindow = 30;

    if (end - start > maxWindow && !sb.updating && v.currentTime > start + 10) {
      const removeEnd = Math.max(start, v.currentTime - 8);
      if (removeEnd > start) {
        try {
          sb.remove(start, removeEnd);
        } catch (e) {
          console.warn("[MP4Client] sb.remove failed", e);
        }
      }
    }
  }

  private monitorBufferHealth() {
    if (!this.videoElement || !this.sourceBuffer) return;

    const sb = this.sourceBuffer;
    const v = this.videoElement;

    if (sb.buffered.length > 0 && !v.paused) {
      const currentTime = v.currentTime;
      const latestTime = sb.buffered.end(sb.buffered.length - 1);
      const bufferAhead = latestTime - currentTime;

      // Gentle playbackRate control around targetLagSec
      const error = bufferAhead - this.targetLagSec;
      const deadband = 0.15;
      if (Math.abs(error) > deadband) {
        let rate = 1.0;
        if (error > 0) {
          // too much buffer: speed up slightly
          rate = Math.min(
            this.maxPlaybackRate,
            1.0 + Math.min(0.05, error * 0.04)
          );
        } else {
          // too little buffer: allow tiny slowdown only before first playback
          const allowSlow = !this.hasStartedPlayback;
          const targetMin = allowSlow ? this.minPlaybackRate : 1.0;
          rate = Math.max(targetMin, 1.0 + Math.max(-0.02, error * 0.02));
        }
        if (Math.abs(v.playbackRate - rate) > 0.005) {
          v.playbackRate = rate;
        }
      } else if (v.playbackRate !== 1.0) {
        v.playbackRate = 1.0;
      }

      // Preventive micro back-seek when extremely close to edge and data incoming
      if (
        bufferAhead < 0.08 &&
        bufferAhead > 0 &&
        this.appendQueue.length > 0
      ) {
        const seekTo = Math.max(sb.buffered.start(0), currentTime - 0.03);
        if (seekTo < latestTime) {
          v.currentTime = seekTo;
        }
      }
    }
  }

  private handleVideoStall() {
    if (!this.videoElement || !this.sourceBuffer) return;

    const sb = this.sourceBuffer;
    const v = this.videoElement;

    // Check if we have buffered data ahead
    if (sb.buffered.length > 0) {
      const currentTime = v.currentTime;
      const latestTime = sb.buffered.end(sb.buffered.length - 1);
      const lag = latestTime - currentTime;

      if (lag > 0.1) {
        const seekTo = currentTime + 0.05;
        if (seekTo < latestTime) {
          v.currentTime = seekTo;
        }
      }
    }

    if (v.paused) {
      v.play().catch(() => {});
    }
  }

  // Pulls as many complete top-level boxes as possible from this.pending
  private extractCompleteBoxesFromPending(): Uint8Array[] {
    const out: Uint8Array[] = [];
    let offset = 0;
    while (this.pending.length - offset >= 8) {
      const size32 = MP4Client.readU32(this.pending, offset);
      const type = MP4Client.readType(this.pending, offset + 4);
      let header = 8;
      let totalSize = size32 >>> 0;

      if (size32 === 1) {
        // largesize (64-bit) follows
        if (this.pending.length - offset < 16) break; // need more
        const size64 = MP4Client.readU64(this.pending, offset + 8);
        if (size64 < 16n) break; // invalid
        header = 16;
        // clamp to Number safely (fragments are small here)
        totalSize = Number(size64);
      } else if (size32 === 0) {
        // box extends to end of file/stream; in live stream we can't know yet
        // wait for more data
        break;
      } else if (size32 < 8) {
        // invalid size
        break;
      }

      if (offset + totalSize > this.pending.length) break; // incomplete

      // Special-case: sanity check for very small mdat (should have payload)
      if (type === "mdat" && totalSize <= header) {
        // need more bytes
        break;
      }

      const box = this.pending.subarray(offset, offset + totalSize);
      out.push(box);
      offset += totalSize;
    }
    if (offset > 0) {
      this.pending = this.pending.subarray(offset);
    }
    return out;
  }

  private static extractAvc1FromInit(init: Uint8Array): string | "" {
    // Locate moov
    const moov = MP4Client.findBoxDeep(init, ["moov"]);
    if (!moov) return "";
    // Iterate traks under moov, pick the one with hdlr.handler_type == 'vide'
    const traks = MP4Client.findChildren(moov, "trak");
    for (const trak of traks) {
      const mdia = MP4Client.findBoxDeep(trak, ["mdia"]);
      if (!mdia) continue;
      const hdlr = MP4Client.findBoxDeep(mdia, ["hdlr"]);
      if (!hdlr || hdlr.length < 12) continue;
      // hdlr payload: version(1) flags(3) pre_defined(4) handler_type(4)
      const handlerType = MP4Client.readType(hdlr, 8);
      if (handlerType !== "vide") continue;
      const stsd = MP4Client.findBoxDeep(trak, [
        "mdia",
        "minf",
        "stbl",
        "stsd",
      ]);
      if (!stsd || stsd.length < 8) continue;
      let off = 8; // skip version/flags + entry_count
      while (off + 8 <= stsd.length) {
        const size = MP4Client.readU32(stsd, off);
        const type = MP4Client.readType(stsd, off + 4);
        if (size < 8 || off + size > stsd.length) break;
        if (type === "avc1" || type === "avc3") {
          const avc1Box = stsd.subarray(off, off + size);
          // VisualSampleEntry has 78 bytes of fields after header before child boxes
          const avcC =
            MP4Client.findChildBoxFrom(avc1Box, "avcC", 8 + 78) ||
            MP4Client.findChildBox(avc1Box, "avcC");
          if (avcC && avcC.length >= 7 && avcC[0] === 1) {
            const profile = avcC[1];
            const compat = avcC[2];
            const level = avcC[3];
            return `avc1.${MP4Client.hex2(profile)}${MP4Client.hex2(
              compat
            )}${MP4Client.hex2(level)}`;
          }
        }
        off += size;
      }
    }
    return "";
  }

  private static findBoxDeep(
    buf: Uint8Array,
    path: string[]
  ): Uint8Array | null {
    if (path.length === 0) return null;
    const target = path[0];
    let o = 0;
    while (o + 8 <= buf.length) {
      const sz = MP4Client.readU32(buf, o);
      const tp = MP4Client.readType(buf, o + 4);
      if (sz < 8 || o + sz > buf.length) break;
      if (tp === target) {
        if (path.length === 1) {
          return buf.subarray(o + 8, o + sz);
        }
        const child = buf.subarray(o + 8, o + sz);
        return MP4Client.findBoxDeep(child, path.slice(1));
      }
      o += sz;
    }
    return null;
  }

  private static findChildBox(
    buf: Uint8Array,
    target: string
  ): Uint8Array | null {
    // buf is a single mp4 box payload (including header at start)
    // We need to scan its children: skip the 8-byte header
    if (buf.length < 8) return null;
    let o = 8;
    while (o + 8 <= buf.length) {
      const sz = MP4Client.readU32(buf, o);
      const tp = MP4Client.readType(buf, o + 4);
      if (sz < 8 || o + sz > buf.length) break;
      if (tp === target) {
        return buf.subarray(o + 8, o + sz);
      }
      o += sz;
    }
    return null;
  }

  // Find child box payload starting from specific offset (relative to start of buf)
  private static findChildBoxFrom(
    buf: Uint8Array,
    target: string,
    start: number
  ): Uint8Array | null {
    if (buf.length < start + 8) return null;
    let o = start;
    while (o + 8 <= buf.length) {
      const sz = MP4Client.readU32(buf, o);
      const tp = MP4Client.readType(buf, o + 4);
      if (sz < 8 || o + sz > buf.length) break;
      if (tp === target) {
        return buf.subarray(o + 8, o + sz);
      }
      o += sz;
    }
    return null;
  }

  private static findChildren(buf: Uint8Array, target: string): Uint8Array[] {
    const out: Uint8Array[] = [];
    if (buf.length < 8) return out;
    let o = 0;
    while (o + 8 <= buf.length) {
      const sz = MP4Client.readU32(buf, o);
      const tp = MP4Client.readType(buf, o + 4);
      if (sz < 8 || o + sz > buf.length) break;
      if (tp === target) {
        // return child payload (skip 8-byte header)
        out.push(buf.subarray(o + 8, o + sz));
      }
      o += sz;
    }
    return out;
  }

  private static readU32(buf: Uint8Array, off: number): number {
    return (
      ((buf[off] << 24) |
        (buf[off + 1] << 16) |
        (buf[off + 2] << 8) |
        buf[off + 3]) >>>
      0
    );
  }

  private static readU64(buf: Uint8Array, off: number): bigint {
    const hi = BigInt(MP4Client.readU32(buf, off));
    const lo = BigInt(MP4Client.readU32(buf, off + 4));
    return (hi << 32n) | lo;
  }

  private static readType(buf: Uint8Array, off: number): string {
    return String.fromCharCode(
      buf[off],
      buf[off + 1],
      buf[off + 2],
      buf[off + 3]
    );
  }

  private static hex2(v: number): string {
    return (v & 0xff).toString(16).toUpperCase().padStart(2, "0");
  }

  private static concat(a: Uint8Array, b: Uint8Array): Uint8Array {
    const out = new Uint8Array(a.length + b.length);
    out.set(a, 0);
    out.set(b, a.length);
    return out;
  }

  /**
   * Clean up resources
   */
  private cleanup(): void {
    // Cancel any ongoing HTTP requests
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    // Clear SourceBuffer reference first
    this.sourceBuffer = null;

    // End MediaSource stream if it's still open
    if (this.mediaSource && this.mediaSource.readyState === "open") {
      try {
        this.mediaSource.endOfStream();
      } catch (error) {
        console.warn("[MP4Client] Error ending MediaSource:", error);
      }
    }

    // Clear MediaSource reference
    this.mediaSource = null;

    // Remove video element
    if (this.videoElement && this.videoElement.parentNode) {
      try {
        this.videoElement.parentNode.removeChild(this.videoElement);
      } catch (error) {
        console.warn("[MP4Client] Error removing video element:", error);
      }
    }

    // Revoke object URL
    if (this.videoElement && this.videoElement.src) {
      try {
        URL.revokeObjectURL(this.videoElement.src);
      } catch (error) {
        console.warn("[MP4Client] Error revoking object URL:", error);
      }
    }

    // Reset references
    this.videoElement = null;
  }

  /**
   * Get connection status
   */
  isStreaming(): boolean {
    return this.isConnected;
  }

  /**
   * Get video element (for external access if needed)
   */
  getVideoElement(): HTMLVideoElement | null {
    return this.videoElement;
  }

  /**
   * Update stats
   */
  private updateStats(): void {
    if (!this.videoElement) return;

    const width = this.videoElement.videoWidth;
    const height = this.videoElement.videoHeight;
    const resolution = width && height ? `${width}x${height}` : "";

    this.onStatsUpdate?.({
      resolution,
      fps: 0, // MP4 streams don't provide real-time FPS
      latency: 0, // MP4 streams don't provide real-time latency
    });
  }

  // ControlClient interface implementation

  /**
   * Connect control WebSocket
   */
  private async connectControl(
    deviceSerial: string,
    wsUrl: string
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const controlWsUrl = `${wsUrl}/api/devices/${deviceSerial}/control`;
      console.log(
        `[MP4Client] Connecting to control WebSocket: ${controlWsUrl}`
      );
      this.controlWs = new WebSocket(controlWsUrl);

      // set timeout to avoid WebSocket connection failure
      setInterval(() => {
        resolve();
      }, 1000);

      this.controlWs.onopen = () => {
        console.log("[MP4Client] Control WebSocket connected successfully");
        this.isControlConnectedFlag = true;
        resolve();
      };

      this.controlWs.onerror = (error) => {
        console.error("[MP4Client] Control WebSocket error:", error);
        reject(error);
      };

      this.controlWs.onclose = () => {
        console.log("[MP4Client] Control WebSocket closed");
        this.isControlConnectedFlag = false;
      };
    });
  }

  /**
   * Check if control is connected
   */
  isControlConnected(): boolean {
    const isConnected = this.isControlConnectedFlag;
    console.log(`[MP4Client] isControlConnected check:`, {
      isControlConnectedFlag: this.isControlConnectedFlag,
      wsState: this.controlWs?.readyState,
      result: isConnected,
    });
    return isConnected;
  }

  /**
   * Send key event
   */
  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void {
    if (!this.isControlConnected() || !this.controlWs) {
      console.warn("[MP4Client] Control not connected, cannot send key event");
      return;
    }

    const message = {
      type: "key",
      keycode,
      action,
      metaState: metaState || 0,
    };

    console.log(`[MP4Client] Sending key event:`, message);
    this.controlWs.send(JSON.stringify(message));
  }

  /**
   * Send touch event
   */
  sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure?: number
  ): void {
    console.log(`[MP4Client] sendTouchEvent called:`, {
      x,
      y,
      action,
      pressure,
    });

    if (!this.isControlConnected() || !this.controlWs) {
      console.warn(
        "[MP4Client] Control not connected, cannot send touch event"
      );
      return;
    }

    const message = {
      type: "touch",
      x,
      y,
      action,
      pressure: pressure || 1.0,
    };

    console.log(`[MP4Client] Sending touch event:`, message);
    this.controlWs.send(JSON.stringify(message));
  }

  /**
   * Send control action
   */
  sendControlAction(action: string, params?: unknown): void {
    console.log(`[MP4Client] sendControlAction called:`, { action, params });

    if (!this.isControlConnected() || !this.controlWs) {
      console.warn(
        "[MP4Client] Control not connected, cannot send control action"
      );
      return;
    }

    const message = {
      type: "control",
      action,
      params: params || {},
    };

    console.log(`[MP4Client] Sending control action:`, message);
    this.controlWs.send(JSON.stringify(message));
  }

  /**
   * Send clipboard set
   */
  sendClipboardSet(text: string, paste?: boolean): void {
    if (!this.isControlConnected() || !this.controlWs) {
      console.warn("[MP4Client] Control not connected, cannot send clipboard");
      return;
    }

    const message = {
      type: "clipboard",
      action: "set",
      text,
      paste: paste || false,
    };

    this.controlWs.send(JSON.stringify(message));
  }

  /**
   * Request keyframe
   */
  requestKeyframe(): void {
    // MP4 streams are typically continuous, no need to request keyframes
  }

  /**
   * Handle mouse event
   */
  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void {
    console.log(`[MP4Client] handleMouseEvent called:`, {
      action,
      hasVideoElement: !!this.videoElement,
    });

    if (!this.videoElement) {
      console.warn("[MP4Client] No video element, cannot handle mouse event");
      return;
    }

    const rect = this.videoElement.getBoundingClientRect();
    // Send normalized coordinates (0-1) like H264Client and WebRTCClient
    const x = (event.clientX - rect.left) / rect.width;
    const y = (event.clientY - rect.top) / rect.height;

    console.log(`[MP4Client] Mouse event coordinates:`, {
      clientX: event.clientX,
      clientY: event.clientY,
      rectLeft: rect.left,
      rectTop: rect.top,
      rectWidth: rect.width,
      rectHeight: rect.height,
      normalizedX: x,
      normalizedY: y,
    });

    if (action === "down") {
      this.isMouseDragging = true;
      console.log(`[MP4Client] Mouse drag started, isMouseDragging = true`);
    } else if (action === "up") {
      this.isMouseDragging = false;
      console.log(`[MP4Client] Mouse drag ended, isMouseDragging = false`);
    }

    this.sendTouchEvent(x, y, action);
  }

  /**
   * Handle touch event
   */
  handleTouchEvent(event: TouchEvent, action: "down" | "up" | "move"): void {
    console.log(`[MP4Client] handleTouchEvent called:`, {
      action,
      hasVideoElement: !!this.videoElement,
      touchesLength: event.touches.length,
    });

    if (!this.videoElement || event.touches.length === 0) {
      console.warn(
        "[MP4Client] No video element or no touches, cannot handle touch event"
      );
      return;
    }

    const touch = event.touches[0];
    const rect = this.videoElement.getBoundingClientRect();
    // Send normalized coordinates (0-1) like H264Client and WebRTCClient
    const x = (touch.clientX - rect.left) / rect.width;
    const y = (touch.clientY - rect.top) / rect.height;

    console.log(`[MP4Client] Touch event coordinates:`, {
      clientX: touch.clientX,
      clientY: touch.clientY,
      rectLeft: rect.left,
      rectTop: rect.top,
      rectWidth: rect.width,
      rectHeight: rect.height,
      normalizedX: x,
      normalizedY: y,
    });

    if (action === "down") {
      this.isMouseDragging = true;
      console.log(`[MP4Client] Touch drag started, isMouseDragging = true`);
    } else if (action === "up") {
      this.isMouseDragging = false;
      console.log(`[MP4Client] Touch drag ended, isMouseDragging = false`);
    }

    this.sendTouchEvent(x, y, action);
  }
}
