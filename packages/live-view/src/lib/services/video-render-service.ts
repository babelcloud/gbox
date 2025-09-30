// Video rendering service for handling canvas operations and video display
import { StatsService } from "./stats-service";

export interface VideoRenderServiceOptions {
  container: HTMLElement;
  onStatsUpdate?: (stats: { resolution?: string; fps?: number }) => void;
  onError?: (error: Error) => void;
  enableStats?: boolean;
  enableResizeObserver?: boolean;
  enableOrientationCheck?: boolean;
  aspectRatioMode?: "contain" | "cover" | "fill" | "scale-down";
  backgroundColor?: string;
  statsService?: StatsService | null; // Optional external stats service
}

export interface CanvasDimensions {
  width: number;
  height: number;
}

export interface VideoFrame {
  displayWidth: number;
  displayHeight: number;
  close(): void;
}

export class VideoRenderService {
  private options: Required<VideoRenderServiceOptions>;
  private canvas: HTMLCanvasElement | null = null;
  private context: CanvasRenderingContext2D | null = null;
  private videoElement: HTMLVideoElement | null = null;
  private statsService: StatsService | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private orientationCheckInterval: number | null = null;
  private lastOrientation: string | null = null;
  private lastCanvasDimensions: CanvasDimensions | null = null;
  private lastResolution: string | null = null;
  private isActive: boolean = false;

  constructor(options: VideoRenderServiceOptions) {
    this.options = {
      onStatsUpdate: options.onStatsUpdate ?? (() => {}),
      onError: options.onError ?? (() => {}),
      enableStats: options.enableStats ?? true,
      enableResizeObserver: options.enableResizeObserver ?? true,
      enableOrientationCheck: options.enableOrientationCheck ?? true,
      aspectRatioMode: options.aspectRatioMode ?? "contain",
      backgroundColor: options.backgroundColor ?? "black",
      statsService: options.statsService || null,
      ...options,
    };

    this.initializeCanvas();
    this.setupStatsService();
  }

  /**
   * Initialize canvas element
   */
  private initializeCanvas(): void {
    // Create canvas element (like old code)
    this.canvas = document.createElement("canvas");
    this.canvas.style.display = "block";
    this.canvas.style.width = "100%";
    this.canvas.style.height = "100%";
    this.canvas.style.objectFit = "contain";
    this.canvas.style.background = this.options.backgroundColor;
    this.canvas.style.margin = "auto";

    // Get 2D context
    this.context = this.canvas.getContext("2d");
    if (!this.context) {
      throw new Error("Failed to get 2D rendering context");
    }

    // Don't append to container automatically - let the client decide where to place it
    // this.options.container.appendChild(this.canvas);

    // Setup resize observer
    if (this.options.enableResizeObserver) {
      this.setupResizeObserver();
    }

    // Setup orientation check
    if (this.options.enableOrientationCheck) {
      this.setupOrientationCheck();
    }
  }

  /**
   * Draw frame centered on canvas
   */
  private drawFrameCentered(frame: VideoFrame): void {
    if (!this.context || !this.canvas) return;

    const canvasWidth = this.canvas.width;
    const canvasHeight = this.canvas.height;
    const frameWidth = frame.displayWidth;
    const frameHeight = frame.displayHeight;

    // Calculate aspect ratios
    const canvasAspect = canvasWidth / canvasHeight;
    const frameAspect = frameWidth / frameHeight;

    let drawWidth: number;
    let drawHeight: number;

    if (this.options.aspectRatioMode === "contain") {
      // Scale to fit within canvas while maintaining aspect ratio
      if (frameAspect > canvasAspect) {
        // Frame is wider, fit to width
        drawWidth = canvasWidth;
        drawHeight = canvasWidth / frameAspect;
      } else {
        // Frame is taller, fit to height
        drawHeight = canvasHeight;
        drawWidth = canvasHeight * frameAspect;
      }
    } else if (this.options.aspectRatioMode === "cover") {
      // Scale to cover entire canvas while maintaining aspect ratio
      if (frameAspect > canvasAspect) {
        // Frame is wider, fit to height
        drawHeight = canvasHeight;
        drawWidth = canvasHeight * frameAspect;
      } else {
        // Frame is taller, fit to width
        drawWidth = canvasWidth;
        drawHeight = canvasWidth / frameAspect;
      }
    } else {
      // Fill mode - stretch to fit canvas
      drawWidth = canvasWidth;
      drawHeight = canvasHeight;
    }

    // Center the drawing
    const drawX = (canvasWidth - drawWidth) / 2;
    const drawY = (canvasHeight - drawHeight) / 2;

    // Clear canvas with background color
    this.context.fillStyle = this.options.backgroundColor;
    this.context.fillRect(0, 0, canvasWidth, canvasHeight);

    // Draw frame centered
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    this.context.drawImage(frame as any, drawX, drawY, drawWidth, drawHeight);
  }

  /**
   * Setup stats service
   */
  private setupStatsService(): void {
    if (!this.options.enableStats) return;

    // Use external stats service if provided, otherwise create new one
    if (this.options.statsService) {
      this.statsService = this.options.statsService;
    } else {
      this.statsService = new StatsService({
        onStatsUpdate: (stats) => {
          this.options.onStatsUpdate(stats);
        },
        enableFPS: true,
        enableResolution: true,
        enableLatency: false,
        enableBandwidth: false,
      });
    }
  }

  /**
   * Setup resize observer for container size changes
   */
  private setupResizeObserver(): void {
    if (!this.canvas) return;

    this.resizeObserver = new ResizeObserver(() => {
      this.handleContainerResize();
    });

    this.resizeObserver.observe(this.options.container);
  }

  /**
   * Setup orientation check for device rotation
   */
  private setupOrientationCheck(): void {
    this.orientationCheckInterval = window.setInterval(() => {
      this.checkOrientationChange();
    }, 500);
  }

  /**
   * Handle container resize
   */
  private handleContainerResize(): void {
    if (!this.canvas || !this.lastCanvasDimensions) return;

    this.updateCanvasDisplaySize(
      this.lastCanvasDimensions.width,
      this.lastCanvasDimensions.height
    );
  }

  /**
   * Check for orientation changes
   */
  private checkOrientationChange(): void {
    if (!this.canvas) return;

    const currentOrientation = this.getOrientation();
    if (currentOrientation !== this.lastOrientation) {
      console.log("[VideoRenderService] Orientation changed:", {
        from: this.lastOrientation,
        to: currentOrientation,
      });

      this.lastOrientation = currentOrientation;
      this.handleOrientationChange();
    }
  }

  /**
   * Get current orientation
   */
  private getOrientation(): string {
    if (window.screen && window.screen.orientation) {
      return window.screen.orientation.type;
    }
    return window.innerWidth > window.innerHeight ? "landscape" : "portrait";
  }

  /**
   * Handle orientation change
   */
  private handleOrientationChange(): void {
    if (!this.canvas || !this.lastCanvasDimensions) return;

    // Force canvas redraw after orientation change
    this.updateCanvasDisplaySize(
      this.lastCanvasDimensions.width,
      this.lastCanvasDimensions.height,
      false // No transition for orientation changes
    );
  }

  /**
   * Update canvas display size to maintain aspect ratio
   */
  updateCanvasDisplaySize(
    videoWidth: number,
    videoHeight: number,
    useTransition: boolean = true
  ): void {
    if (!this.canvas) return;

    const container = this.options.container;
    const containerRect = container.getBoundingClientRect();
    const containerWidth = containerRect.width;
    const containerHeight = containerRect.height;

    if (containerWidth === 0 || containerHeight === 0) {
      console.warn("[VideoRenderService] Container has zero dimensions");
      return;
    }

    // Calculate aspect ratios
    const videoAspectRatio = videoWidth / videoHeight;
    const containerAspectRatio = containerWidth / containerHeight;

    let displayWidth: number;
    let displayHeight: number;

    // Calculate display dimensions based on aspect ratio mode
    switch (this.options.aspectRatioMode) {
      case "contain":
        if (videoAspectRatio > containerAspectRatio) {
          displayWidth = containerWidth;
          displayHeight = containerWidth / videoAspectRatio;
        } else {
          displayHeight = containerHeight;
          displayWidth = containerHeight * videoAspectRatio;
        }
        break;
      case "cover":
        if (videoAspectRatio > containerAspectRatio) {
          displayHeight = containerHeight;
          displayWidth = containerHeight * videoAspectRatio;
        } else {
          displayWidth = containerWidth;
          displayHeight = containerWidth / videoAspectRatio;
        }
        break;
      case "fill":
        displayWidth = containerWidth;
        displayHeight = containerHeight;
        break;
      case "scale-down": {
        const containWidth =
          videoAspectRatio > containerAspectRatio
            ? containerWidth
            : containerHeight * videoAspectRatio;
        const containHeight =
          videoAspectRatio > containerAspectRatio
            ? containerWidth / videoAspectRatio
            : containerHeight;
        displayWidth = Math.min(containWidth, containerWidth);
        displayHeight = Math.min(containHeight, containerHeight);
        break;
      }
      default:
        displayWidth = containerWidth;
        displayHeight = containerHeight;
    }

    // Center the canvas
    const offsetX = (containerWidth - displayWidth) / 2;
    const offsetY = (containerHeight - displayHeight) / 2;

    // Apply styles
    this.canvas.style.width = `${displayWidth}px`;
    this.canvas.style.height = `${displayHeight}px`;
    this.canvas.style.marginLeft = `${offsetX}px`;
    this.canvas.style.marginTop = `${offsetY}px`;

    // Add transition if requested
    if (useTransition) {
      this.canvas.style.transition = "all 0.3s ease-in-out";
    } else {
      this.canvas.style.transition = "none";
    }

    // Store dimensions for future reference
    this.lastCanvasDimensions = { width: videoWidth, height: videoHeight };

    // Update stats service with new resolution
    this.statsService?.updateResolution(videoWidth, videoHeight);
  }

  /**
   * Render a video frame to canvas (for H264)
   */
  renderFrame(frame: VideoFrame): void {
    if (!this.context || !this.canvas) {
      console.warn("[VideoRenderService] Canvas not available for rendering");
      return;
    }

    try {
      // Check for resolution changes
      const currentResolution = `${frame.displayWidth}x${frame.displayHeight}`;
      const isResolutionChange = currentResolution !== this.lastResolution;
      const isSignificantChange = this.isSignificantResolutionChange(
        frame.displayWidth,
        frame.displayHeight
      );

      // Always update resolution if we have valid dimensions
      if (frame.displayWidth > 0 && frame.displayHeight > 0) {
        if (isResolutionChange) {
          console.log("[VideoRenderService] Resolution changed:", {
            from: this.lastResolution,
            to: currentResolution,
            isSignificant: isSignificantChange,
          });

          // Update canvas pixel dimensions
          this.canvas.width = frame.displayWidth;
          this.canvas.height = frame.displayHeight;

          // Update display size
          this.updateCanvasDisplaySize(
            frame.displayWidth,
            frame.displayHeight,
            !isSignificantChange // No transition for significant changes
          );
        }

        // Always update stats service with current resolution (even if no change)
        this.lastResolution = currentResolution;
        this.statsService?.updateResolution(
          frame.displayWidth,
          frame.displayHeight
        );
      } else {
        console.warn("[VideoRenderService] Invalid frame dimensions:", {
          displayWidth: frame.displayWidth,
          displayHeight: frame.displayHeight,
          currentResolution,
          lastResolution: this.lastResolution,
        });
      }

      // Draw frame to canvas with proper centering
      this.drawFrameCentered(frame);

      // Record frame for stats
      this.statsService?.recordFrameDecoded();
    } catch (error) {
      console.error("[VideoRenderService] Failed to render frame:", error);
      this.options.onError(error as Error);
    } finally {
      frame.close();
    }
  }

  /**
   * Setup video element for WebRTC playback
   */
  setupVideoElement(): HTMLVideoElement {
    if (this.videoElement) {
      return this.videoElement;
    }

    this.videoElement = document.createElement("video");
    this.videoElement.autoplay = true;
    this.videoElement.muted = false;
    this.videoElement.playsInline = true;
    this.videoElement.controls = false;
    this.videoElement.preload = "auto";
    this.videoElement.style.width = "100%";
    this.videoElement.style.height = "100%";
    this.videoElement.style.objectFit = this.options.aspectRatioMode;
    this.videoElement.style.background = this.options.backgroundColor;

    // Setup event handlers
    this.videoElement.onloadedmetadata = () => {
      if (!this.videoElement) return;
      const width = this.videoElement.videoWidth;
      const height = this.videoElement.videoHeight;
      console.log(
        "[VideoRenderService] Video metadata loaded:",
        `${width}x${height}`
      );

      if (width && height) {
        this.updateCanvasDisplaySize(width, height);
        this.statsService?.updateResolution(width, height);
        this.options.onStatsUpdate({ resolution: `${width}x${height}` });
      }
    };

    this.videoElement.onplaying = () => {
      console.log("[VideoRenderService] Video started playing");
    };

    // Append to container
    this.options.container.appendChild(this.videoElement);

    return this.videoElement;
  }

  /**
   * Set video source for WebRTC
   */
  setVideoSource(stream: MediaStream): void {
    if (!this.videoElement) {
      this.setupVideoElement();
    }

    if (this.videoElement) {
      this.videoElement.srcObject = stream;
    }
  }

  /**
   * Check if resolution change is significant
   */
  private isSignificantResolutionChange(
    newWidth: number,
    newHeight: number
  ): boolean {
    if (!this.lastCanvasDimensions) return true;

    const { width: oldWidth, height: oldHeight } = this.lastCanvasDimensions;
    const widthChange = Math.abs(newWidth - oldWidth) / oldWidth;
    const heightChange = Math.abs(newHeight - oldHeight) / oldHeight;

    // Consider it significant if change is more than 10%
    return widthChange > 0.1 || heightChange > 0.1;
  }

  /**
   * Start rendering service
   */
  start(): void {
    if (this.isActive) {
      console.log("[VideoRenderService] Already active");
      return;
    }

    this.isActive = true;
    this.statsService?.start();
    console.log("[VideoRenderService] Started");
  }

  /**
   * Stop rendering service
   */
  stop(): void {
    if (!this.isActive) return;

    this.isActive = false;
    this.statsService?.stop();
    this.clearObservers();
    console.log("[VideoRenderService] Stopped");
  }

  /**
   * Clear all observers and intervals
   */
  private clearObservers(): void {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }

    if (this.orientationCheckInterval) {
      clearInterval(this.orientationCheckInterval);
      this.orientationCheckInterval = null;
    }
  }

  /**
   * Get canvas element
   */
  getCanvas(): HTMLCanvasElement | null {
    return this.canvas;
  }

  /**
   * Get video element
   */
  getVideoElement(): HTMLVideoElement | null {
    return this.videoElement;
  }

  /**
   * Get current resolution
   */
  getCurrentResolution(): string | null {
    return this.lastResolution;
  }

  /**
   * Update service options
   */
  updateOptions(newOptions: Partial<VideoRenderServiceOptions>): void {
    this.options = { ...this.options, ...newOptions };
  }

  /**
   * Check if service is active
   */
  get active(): boolean {
    return this.isActive;
  }

  /**
   * Cleanup and destroy service
   */
  destroy(): void {
    this.stop();

    if (this.canvas && this.canvas.parentNode) {
      this.canvas.parentNode.removeChild(this.canvas);
    }

    if (this.videoElement && this.videoElement.parentNode) {
      this.videoElement.parentNode.removeChild(this.videoElement);
    }

    this.canvas = null;
    this.context = null;
    this.videoElement = null;
    this.statsService = null;
  }
}
