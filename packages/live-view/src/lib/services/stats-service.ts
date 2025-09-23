// Stats monitoring service for collecting and reporting performance metrics
import { Stats } from "../types";

export interface StatsServiceOptions {
  updateInterval?: number; // Update interval in milliseconds
  onStatsUpdate?: (stats: Stats) => void;
  enableFPS?: boolean;
  enableResolution?: boolean;
  enableLatency?: boolean;
  enableBandwidth?: boolean;
}

export interface PerformanceMetrics {
  fps?: number;
  resolution?: string;
  latency?: number;
  bandwidth?: number;
  frameDrops?: number;
  bitrate?: number;
  packetLoss?: number;
}

export class StatsService {
  private options: Required<StatsServiceOptions>;
  private updateTimer: number | null = null;
  private isActive: boolean = false;

  // FPS tracking
  private lastFramesDecoded: number = 0;
  // private lastFramesReceived: number = 0; // eslint-disable-line @typescript-eslint/no-unused-vars
  private lastStatsTime: number = 0;
  private frameCount: number = 0;
  private lastFrameTime: number = 0;

  // Resolution tracking
  private lastResolution: string | null = null;

  // Latency tracking
  private pingTimes: number[] = [];
  private maxPingHistory: number = 10;

  // Bandwidth tracking
  private bytesReceived: number = 0;
  private lastBytesReceived: number = 0;
  private lastBandwidthTime: number = 0;

  // Current stats cache for merging updates
  private currentStats: Stats = {};

  constructor(options: StatsServiceOptions = {}) {
    this.options = {
      updateInterval: options.updateInterval ?? 1000, // 1 second default
      onStatsUpdate: options.onStatsUpdate ?? (() => {}),
      enableFPS: options.enableFPS ?? true,
      enableResolution: options.enableResolution ?? true,
      enableLatency: options.enableLatency ?? true,
      enableBandwidth: options.enableBandwidth ?? false,
    };
  }

  /**
   * Start stats monitoring
   */
  start(): void {
    if (this.isActive) {
      console.log("[StatsService] Already monitoring stats");
      return;
    }

    this.isActive = true;
    this.resetCounters();
    this.currentStats = {}; // Initialize stats cache

    console.log("[StatsService] Starting stats monitoring");
    this.scheduleUpdate();
  }

  /**
   * Stop stats monitoring
   */
  stop(): void {
    if (!this.isActive) {
      return;
    }

    this.isActive = false;
    this.clearUpdateTimer();
    this.currentStats = {}; // Clear stats cache

    console.log("[StatsService] Stopped stats monitoring");
  }

  /**
   * Update FPS counter (call when a frame is decoded)
   */
  recordFrameDecoded(): void {
    if (!this.options.enableFPS) return;

    this.frameCount++;
    this.lastFramesDecoded = this.frameCount; // Update lastFramesDecoded for getCurrentStats
    const currentTime = Date.now();

    if (this.lastFrameTime === 0) {
      this.lastFrameTime = currentTime;
      this.lastStatsTime = currentTime; // Initialize lastStatsTime
      return;
    }

    // Calculate FPS every second
    const timeDiff = (currentTime - this.lastFrameTime) / 1000;
    if (timeDiff >= 1.0) {
      const fps = Math.round(this.frameCount / timeDiff);
      this.updateFPS(fps);

      this.frameCount = 0;
      this.lastFrameTime = currentTime;
      this.lastStatsTime = currentTime; // Update lastStatsTime
    }
  }

  /**
   * Update resolution (call when resolution changes)
   */
  updateResolution(width: number, height: number): void {
    if (!this.options.enableResolution) {
      console.debug("[StatsService] updateResolution - resolution disabled");
      return;
    }

    // Check for invalid dimensions
    const isValidWidth = width > 0 && Number.isFinite(width);
    const isValidHeight = height > 0 && Number.isFinite(height);
    const isValidDimensions = isValidWidth && isValidHeight;

    if (!isValidDimensions) {
      console.warn(
        "[StatsService] updateResolution - invalid dimensions, skipping update:",
        {
          width,
          height,
          isValidWidth,
          isValidHeight,
          currentLastResolution: this.lastResolution,
        }
      );
      return;
    }

    const resolution = `${width}x${height}`;

    if (resolution !== this.lastResolution) {
      console.log("[StatsService] Resolution changed:", {
        from: this.lastResolution,
        to: resolution,
        dimensions: { width, height },
      });

      this.lastResolution = resolution;
      this.currentStats.resolution = resolution;
      this.notifyStatsUpdate({ ...this.currentStats });
    }
  }

  /**
   * Record ping time for latency calculation
   */
  recordPingTime(pingTime: number): void {
    if (!this.options.enableLatency) return;

    this.pingTimes.push(pingTime);
    if (this.pingTimes.length > this.maxPingHistory) {
      this.pingTimes.shift();
    }
  }

  /**
   * Record bytes received for bandwidth calculation
   */
  recordBytesReceived(bytes: number): void {
    if (!this.options.enableBandwidth) return;

    this.bytesReceived += bytes;
  }

  /**
   * Process WebRTC stats
   */
  async processWebRTCStats(pc: RTCPeerConnection): Promise<PerformanceMetrics> {
    const metrics: PerformanceMetrics = {};

    try {
      const stats = await pc.getStats();

      stats.forEach((report: any) => {
        if (
          report.type === "inbound-rtp" &&
          (report.mediaType === "video" || report.kind === "video")
        ) {
          // FPS calculation
          if (this.options.enableFPS && report.framesDecoded) {
            const currentTime = Date.now();
            const currentFramesDecoded = report.framesDecoded || 0;

            if (this.lastFramesDecoded > 0 && this.lastStatsTime > 0) {
              const timeDiff = (currentTime - this.lastStatsTime) / 1000;
              const framesDiff = currentFramesDecoded - this.lastFramesDecoded;
              if (timeDiff > 0 && framesDiff >= 0) {
                metrics.fps = Math.round(framesDiff / timeDiff);
              }
            }

            this.lastFramesDecoded = currentFramesDecoded;
            this.lastStatsTime = currentTime;
          }

          // Resolution
          if (this.options.enableResolution) {
            const width = report.frameWidth || 0;
            const height = report.frameHeight || 0;
            if (width && height) {
              metrics.resolution = `${width}x${height}`;
            }
          }

          // Bandwidth
          if (this.options.enableBandwidth && report.bytesReceived) {
            const currentTime = Date.now();
            const currentBytes = report.bytesReceived;

            if (this.lastBytesReceived > 0 && this.lastBandwidthTime > 0) {
              const timeDiff = (currentTime - this.lastBandwidthTime) / 1000;
              const bytesDiff = currentBytes - this.lastBytesReceived;
              if (timeDiff > 0) {
                metrics.bandwidth = Math.round(
                  (bytesDiff * 8) / timeDiff / 1000
                ); // kbps
              }
            }

            this.lastBytesReceived = currentBytes;
            this.lastBandwidthTime = currentTime;
          }
        }

        // Latency from candidate-pair
        if (
          this.options.enableLatency &&
          report.type === "candidate-pair" &&
          report.state === "succeeded" &&
          report.currentRoundTripTime
        ) {
          metrics.latency = Math.round(report.currentRoundTripTime * 1000);
        }
      });

      // Use ping-pong latency if available
      if (this.options.enableLatency && this.pingTimes.length > 0) {
        const avgPing =
          this.pingTimes.reduce((a, b) => a + b, 0) / this.pingTimes.length;
        metrics.latency = Math.round(avgPing);
      }
    } catch (error) {
      console.error("[StatsService] Error processing WebRTC stats:", error);
    }

    return metrics;
  }

  /**
   * Process H264 stats
   */
  processH264Stats(canvas: HTMLCanvasElement): PerformanceMetrics {
    const metrics: PerformanceMetrics = {};

    if (this.options.enableResolution) {
      const width = canvas.width;
      const height = canvas.height;
      if (width && height) {
        metrics.resolution = `${width}x${height}`;
      }
    }

    return metrics;
  }

  /**
   * Get current stats
   */
  getCurrentStats(): Stats {
    const stats: Stats = {};

    if (this.options.enableFPS && this.frameCount > 0) {
      // Calculate current FPS based on recent frame count
      const currentTime = Date.now();
      if (this.lastFrameTime > 0) {
        const timeDiff = (currentTime - this.lastFrameTime) / 1000;
        if (timeDiff > 0) {
          stats.fps = Math.round(this.frameCount / timeDiff);
        }
      }
    }

    if (this.options.enableResolution && this.lastResolution) {
      stats.resolution = this.lastResolution;
    }

    if (this.options.enableLatency && this.pingTimes.length > 0) {
      const avgPing =
        this.pingTimes.reduce((a, b) => a + b, 0) / this.pingTimes.length;
      stats.latency = Math.round(avgPing);
    }

    return stats;
  }

  /**
   * Reset all counters
   */
  reset(): void {
    this.resetCounters();
    this.lastResolution = null;
    this.pingTimes = [];
    this.bytesReceived = 0;
  }

  /**
   * Update service options
   */
  updateOptions(newOptions: Partial<StatsServiceOptions>): void {
    this.options = { ...this.options, ...newOptions };
  }

  /**
   * Check if monitoring is active
   */
  get active(): boolean {
    return this.isActive;
  }

  // Private methods

  private resetCounters(): void {
    this.lastFramesDecoded = 0;
    // this.lastFramesReceived = 0; // Property removed
    this.lastStatsTime = 0;
    this.frameCount = 0;
    this.lastFrameTime = 0;
    this.lastBytesReceived = 0;
    this.lastBandwidthTime = 0;
  }

  private scheduleUpdate(): void {
    if (!this.isActive) return;

    this.updateTimer = window.setTimeout(() => {
      this.performUpdate();
      this.scheduleUpdate();
    }, this.options.updateInterval);
  }

  private performUpdate(): void {
    if (!this.isActive) return;

    const stats = this.getCurrentStats();
    if (Object.keys(stats).length > 0) {
      // Update the cache with all current stats
      this.currentStats = { ...stats };
      this.notifyStatsUpdate(stats);
    }
  }

  private updateFPS(fps: number): void {
    // Merge with existing stats, only update the changed FPS value
    this.currentStats.fps = fps;
    this.notifyStatsUpdate({ ...this.currentStats });
  }

  private notifyStatsUpdate(stats: Stats): void {
    this.options.onStatsUpdate(stats);
  }

  private clearUpdateTimer(): void {
    if (this.updateTimer) {
      clearTimeout(this.updateTimer);
      this.updateTimer = null;
    }
  }
}
