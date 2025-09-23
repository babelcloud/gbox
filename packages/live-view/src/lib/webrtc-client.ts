// Refactored WebRTCClient extending BaseClient
import { BaseClient } from "./base-client";
import {
  ControlMessage,
  SignalingMessage,
  ConnectionParams,
  ClientOptions,
} from "./types";

export class WebRTCClientRefactored extends BaseClient {
  private ws: WebSocket | null = null;
  private pc: RTCPeerConnection | null = null;
  private dataChannel: RTCDataChannel | null = null;
  private videoElement: HTMLVideoElement | null = null;
  private audioElement: HTMLAudioElement | null = null;
  private pendingVideoStream: MediaStream | null = null;
  private lastVideoTime: number = 0;

  // Stats properties
  private statsInterval: number | null = null;
  private lastFramesDecoded = 0;
  private lastStatsTime = 0;
  private stallCheckInterval: number | null = null;

  // Ping measurement properties - now inherited from BaseClient

  // Control message queue for early messages before DataChannel is ready
  private pendingControlMessages: ControlMessage[] = [];

  // Video reset throttling to prevent server overload
  private lastKeyframeRequest: number = 0;
  private readonly KEYFRAME_THROTTLE_MS = 2000; // Minimum 2 seconds between keyframe requests

  // Connection parameters for reconnection
  private lastWsUrl: string | undefined;

  constructor(container: HTMLElement, options: ClientOptions = {}) {
    super(container, options);
  }

  /**
   * Establish WebRTC connection
   */
  protected async establishConnection(params: ConnectionParams): Promise<void> {
    const { deviceSerial, wsUrl } = params;
    this.lastWsUrl = wsUrl;

    console.log(
      `[WebRTC] Establishing WebRTC connection to device: ${deviceSerial}`
    );
    console.log(`[WebRTC] WebSocket URL: ${wsUrl}`);

    // Build WebSocket URL using base class method
    const controlWsUrl = this.buildControlWebSocketUrlFromParams(params);
    console.log(`[WebRTC] Creating WebSocket connection to: ${controlWsUrl}`);
    this.ws = new WebSocket(controlWsUrl);

    // Create WebRTC peer connection with balanced low-latency settings
    this.pc = new RTCPeerConnection({
      iceServers: [],
      bundlePolicy: "max-bundle",
      rtcpMuxPolicy: "require",
      iceCandidatePoolSize: 1, // Use small candidate pool for stability
    });

    // Create data channel for control messages
    this.dataChannel = this.pc.createDataChannel("control", {
      ordered: false, // Allow out-of-order delivery for lower latency
      maxRetransmits: 0, // No retransmissions for lower latency
    });
    this.setupDataChannelHandlers();
    console.log("[WebRTC] Created data channel: control");

    // Add transceivers
    const videoTransceiver = this.pc.addTransceiver("video", {
      direction: "recvonly",
    });
    const audioTransceiver = this.pc.addTransceiver("audio", {
      direction: "recvonly",
    });

    console.log(
      "[WebRTC] Created transceivers - Video mid:",
      videoTransceiver.mid,
      "Audio mid:",
      audioTransceiver.mid
    );

    // Setup event handlers
    this.setupWebRTCHandlers();
    this.setupWebSocketHandlers();

    // Wait for WebSocket to be open, then create offer
    await this.waitForWebSocketAndCreateOffer(deviceSerial);
  }

  /**
   * Cleanup WebRTC connection
   */
  protected async cleanupConnection(): Promise<void> {
    console.log("[WebRTC] Cleaning up WebRTC connection");

    // Close data channel
    if (this.dataChannel) {
      this.dataChannel.close();
      this.dataChannel = null;
    }

    // Close peer connection
    if (this.pc) {
      this.pc.close();
      this.pc = null;
    }

    // Close WebSocket
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    // Stop stats monitoring
    if (this.statsInterval) {
      clearInterval(this.statsInterval);
      this.statsInterval = null;
    }

    // Stop stall detection
    this.stopStallDetection();

    // Stop ping measurement
    this.stopPingMeasurement();

    // Clear pending messages
    this.pendingControlMessages = [];
  }

  /**
   * Check if control is connected
   */
  protected isControlConnectedInternal(): boolean {
    return (
      this.ws?.readyState === WebSocket.OPEN &&
      this.dataChannel?.readyState === "open"
    );
  }

  /**
   * Get last API URL (not used for WebRTC)
   */
  protected getLastApiUrl(): string {
    return "";
  }

  /**
   * Get last WebSocket URL
   */
  protected getLastWsUrl(): string | undefined {
    return this.lastWsUrl;
  }

  /**
   * Register recovery strategies for WebRTC
   */
  protected registerRecoveryStrategies(): void {
    this.errorHandling.registerRecoveryStrategy("WebRTCClient", {
      canRecover: (error, _context) => {
        return (
          error.message.includes("WebRTC") ||
          error.message.includes("connection") ||
          error.message.includes("WebSocket")
        );
      },
      recover: async (error, context) => {
        console.log(
          "[WebRTC] Attempting recovery...",
          error.message,
          context.component
        );
        await this.cleanupConnection();
        if (this.currentDevice && this.lastWsUrl) {
          await this.establishConnection({
            deviceSerial: this.currentDevice,
            apiUrl: "",
            wsUrl: this.lastWsUrl,
          });
        }
      },
      maxRetries: 3,
      retryDelay: 2000,
    });
  }

  /**
   * Setup WebRTC event handlers
   */
  private setupWebRTCHandlers(): void {
    if (!this.pc) return;

    this.pc.ontrack = (event) => {
      console.log("[WebRTC] Track received:", event.track.kind);

      if (event.track.kind === "video") {
        // Enable the track directly like the old implementation
        event.track.enabled = true;
        this.setupVideoTrack(event.streams[0]);
      } else if (event.track.kind === "audio") {
        this.setupAudioTrack(event.streams[0]);
      }
    };

    this.pc.onicecandidate = (event) => {
      if (event.candidate && this.ws) {
        this.ws.send(
          JSON.stringify({
            type: "ice-candidate",
            candidate: event.candidate,
          })
        );
      }
    };

    this.pc.ondatachannel = (event) => {
      console.log("[WebRTC] Data channel received");
      this.dataChannel = event.channel;
      this.setupDataChannelHandlers();
    };

    this.pc.onconnectionstatechange = () => {
      console.log("[WebRTC] Connection state:", this.pc?.connectionState);
      if (this.pc?.connectionState === "failed") {
        this.handleError(
          new Error("WebRTC connection failed"),
          "WebRTCClient",
          "connection"
        );
      }
    };
  }

  /**
   * Setup WebSocket event handlers
   */
  private setupWebSocketHandlers(): void {
    if (!this.ws) return;

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleSignalingMessage(message);
    };

    this.ws.onclose = () => {
      console.log("[WebRTC] WebSocket disconnected");
      if (this.connected) {
        this.startReconnection();
      }
    };

    this.ws.onerror = (error) => {
      console.error("[WebRTC] WebSocket error:", error);
      this.handleError(
        new Error("WebSocket error"),
        "WebRTCClient",
        "websocket"
      );
    };
  }

  /**
   * Setup data channel handlers
   */
  private setupDataChannelHandlers(): void {
    if (!this.dataChannel) return;

    this.dataChannel.onopen = () => {
      console.log("[WebRTC] Data channel opened");
      this.sendPendingControlMessages();
    };

    this.dataChannel.onclose = () => {
      console.log("[WebRTC] Data channel closed");
    };

    this.dataChannel.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);

        // Handle ping responses for latency measurement
        this.handlePingResponse(message);
      } catch (e) {
        // Not JSON
      }
    };

    this.dataChannel.onerror = (error) => {
      console.error("[WebRTC] Data channel error:", error);
      this.handleError(
        new Error("Data channel error"),
        "WebRTCClient",
        "datachannel"
      );
    };
  }

  /**
   * Setup video track
   */
  private setupVideoTrack(stream: MediaStream): void {
    console.log("[WebRTC] setupVideoTrack called with stream:", stream);
    console.log("[WebRTC] Stream video tracks:", stream.getVideoTracks());
    console.log("[WebRTC] Stream audio tracks:", stream.getAudioTracks());

    // Look for video element in the parent container (videoWrapper)
    const parentContainer = this.container.parentElement;
    const existingVideo =
      parentContainer?.querySelector("video") ||
      this.container.querySelector("video");

    console.log("[WebRTC] Container:", this.container);
    console.log("[WebRTC] Parent container:", parentContainer);
    console.log("[WebRTC] Found video element:", existingVideo);

    if (existingVideo) {
      console.log("[WebRTC] Video track received, setting up playback");

      // Enable the video track
      if (stream.getVideoTracks().length > 0) {
        const videoTrack = stream.getVideoTracks()[0];
        console.log("[WebRTC] Video track details:", {
          id: videoTrack.id,
          kind: videoTrack.kind,
          enabled: videoTrack.enabled,
          readyState: videoTrack.readyState,
          muted: videoTrack.muted,
        });
        videoTrack.enabled = true;

        // Note: videoTrack.muted is read-only, we can't change it directly
        if (videoTrack.muted) {
          console.log(
            "[WebRTC] Video track is muted - this may cause video not to display"
          );
        }
      }

      // Basic video element setup
      existingVideo.autoplay = true;
      existingVideo.muted = false;
      existingVideo.playsInline = true;
      existingVideo.controls = false;
      existingVideo.preload = "auto";
      existingVideo.srcObject = stream;

      // Basic styling
      existingVideo.style.objectFit = "contain";
      existingVideo.style.background = "black";

      console.log("[WebRTC] Video srcObject set");

      existingVideo.onloadedmetadata = () => {
        if (!existingVideo) return;
        const width = existingVideo.videoWidth;
        const height = existingVideo.videoHeight;
        console.log("[WebRTC] Video metadata loaded:", `${width}x${height}`);
        console.log(
          "[WebRTC] Video element readyState:",
          existingVideo.readyState
        );
        console.log(
          "[WebRTC] Video element networkState:",
          existingVideo.networkState
        );
        console.log(
          "[WebRTC] Video element srcObject:",
          existingVideo.srcObject
        );
        if (width && height) {
          this.onStatsUpdate?.({ resolution: `${width}x${height}` });
        }
      };

      existingVideo.onloadstart = () => {
        console.log("[WebRTC] Video load started");
      };

      existingVideo.oncanplay = () => {
        console.log("[WebRTC] Video can play");
      };

      existingVideo.onerror = (error) => {
        console.error("[WebRTC] Video error:", error);
      };

      existingVideo.onplaying = () => {
        console.log("[WebRTC] Video started playing");
        // Reset stall detection when video starts playing
        this.lastVideoTime = existingVideo?.currentTime || 0;
      };

      // Try to play the video
      existingVideo.play().catch((error) => {
        console.error("[WebRTC] Failed to play video:", error);
      });

      this.videoElement = existingVideo;
      this.pendingVideoStream = null;
      console.log("[WebRTC] Using existing video element from React component");

      // Set connection state
      this.onConnectionStateChange?.("connected", undefined);
      this.isConnected = true;
      this.startStats();
    } else {
      console.error("[WebRTC] No video element found in container");
    }
  }

  /**
   * Setup audio track
   */
  private setupAudioTrack(stream: MediaStream): void {
    console.log("[WebRTC] Audio track received, stream:", stream);
    console.log("[WebRTC] Audio stream tracks:", stream.getAudioTracks());
    console.log("[WebRTC] Audio stream ID:", stream.id);
    console.log("[WebRTC] Audio stream active:", stream.active);

    // Create a separate audio element for audio playback (like the old implementation)
    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.srcObject = null;
      this.audioElement.remove();
      this.audioElement = null;
    }

    this.audioElement = document.createElement("audio");
    this.audioElement.autoplay = true;
    (this.audioElement as any).playsInline = true;
    this.audioElement.controls = false;
    this.audioElement.preload = "none";
    this.audioElement.srcObject = stream;

    if (stream.getAudioTracks().length > 0) {
      stream.getAudioTracks()[0].enabled = true;
    }

    document.body.appendChild(this.audioElement);

    // Optimize audio for low latency
    if ("setSinkId" in this.audioElement) {
      // Use default audio device for lowest latency
      (this.audioElement as any).setSinkId("default").catch(() => {
        // Ignore if setSinkId fails
      });
    }

    this.audioElement.play().catch((e) => {
      console.error("Audio playback failed:", e);
      this.onError?.(
        new Error("Audio playback failed, click page to enable audio")
      );
    });

    console.log("[WebRTC] Audio element created and started");
    console.log(
      "[WebRTC] Audio element srcObject:",
      this.audioElement.srcObject
    );
    console.log(
      "[WebRTC] Audio element readyState:",
      this.audioElement.readyState
    );
  }

  /**
   * Wait for WebSocket to be open and create offer
   */
  private async waitForWebSocketAndCreateOffer(
    deviceSerial: string
  ): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      if (!this.ws) {
        reject(new Error("WebSocket not initialized"));
        return;
      }

      const timeout = setTimeout(() => {
        reject(new Error("WebSocket connection timeout"));
      }, 5000);

      this.ws.onopen = async () => {
        clearTimeout(timeout);
        console.log("[WebRTC] WebSocket connected, creating offer");

        try {
          // Create and send offer
          const offer = await this.pc!.createOffer();
          console.log(
            "[WebRTC] Offer SDP preview:",
            offer.sdp?.substring(0, 200) + "..."
          );
          await this.pc!.setLocalDescription(offer);

          // Send offer with deviceSerial and proper structure
          this.ws!.send(
            JSON.stringify({
              type: "offer",
              deviceSerial: deviceSerial,
              offer: {
                sdp: offer.sdp,
              },
            })
          );

          console.log("[WebRTC] Offer sent to server");
          resolve();
        } catch (error) {
          reject(error);
        }
      };

      this.ws.onerror = (error) => {
        clearTimeout(timeout);
        console.error("[WebRTC] WebSocket connection error:", error);
        reject(new Error("WebSocket connection error"));
      };
    });
  }

  /**
   * Handle signaling messages
   */
  private handleSignalingMessage(message: SignalingMessage): void {
    switch (message.type) {
      case "offer":
        this.handleOffer(message.sdp!);
        break;
      case "answer":
        this.handleAnswer(message);
        break;
      case "ice-candidate":
        this.handleIceCandidate(message.candidate!);
        break;
      case "error":
        this.handleError(
          new Error(message.error!),
          "WebRTCClient",
          "signaling"
        );
        break;
    }
  }

  /**
   * Handle WebRTC offer
   */
  private async handleOffer(sdp: string): Promise<void> {
    if (!this.pc) return;

    try {
      await this.pc.setRemoteDescription({ type: "offer", sdp });
      const answer = await this.pc.createAnswer();
      await this.pc.setLocalDescription(answer);

      if (this.ws) {
        this.ws.send(
          JSON.stringify({
            type: "answer",
            sdp: answer.sdp,
          })
        );
      }
    } catch (error) {
      this.handleError(error as Error, "WebRTCClient", "offer");
    }
  }

  /**
   * Handle WebRTC answer
   */
  private async handleAnswer(message: {
    sdp?: string;
    answer?: { sdp: string };
  }): Promise<void> {
    if (!this.pc) return;

    try {
      // Handle both formats: direct sdp or nested in answer object
      const sdp = message.sdp || message.answer?.sdp;
      if (!sdp) {
        console.error(
          "[WebRTC] Answer missing SDP field, full message:",
          JSON.stringify(message)
        );
        return;
      }

      console.log(
        "[WebRTC] Answer SDP preview:",
        sdp.substring(0, 200) + "..."
      );

      await this.pc.setRemoteDescription({ type: "answer", sdp });
      console.log("[WebRTC] Remote answer set successfully");
    } catch (error) {
      this.handleError(error as Error, "WebRTCClient", "answer");
    }
  }

  /**
   * Handle ICE candidate
   */
  private async handleIceCandidate(candidate: RTCIceCandidate): Promise<void> {
    if (!this.pc) return;

    try {
      await this.pc.addIceCandidate(candidate);
    } catch (error) {
      this.handleError(error as Error, "WebRTCClient", "ice-candidate");
    }
  }

  /**
   * Send pending control messages
   */
  private sendPendingControlMessages(): void {
    if (!this.dataChannel || this.dataChannel.readyState !== "open") return;

    while (this.pendingControlMessages.length > 0) {
      const message = this.pendingControlMessages.shift();
      if (message) {
        this.dataChannel.send(JSON.stringify(message));
      }
    }
  }

  // Override ControlClient methods for WebRTC-specific implementation
  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void {
    const message: ControlMessage = {
      type: "key",
      keycode,
      action,
      metaState: metaState || 0,
      timestamp: Date.now(),
    };
    this.sendControlMessage(message);
  }

  sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure?: number
  ): void {
    const message: ControlMessage = {
      type: "touch",
      x,
      y,
      action,
      pressure: pressure || 1.0,
      timestamp: Date.now(),
    };
    this.sendControlMessage(message);
  }

  sendControlAction(action: string, params?: Record<string, unknown>): void {
    const message: ControlMessage = {
      type: action as ControlMessage["type"],
      ...params,
      timestamp: Date.now(),
    };
    this.sendControlMessage(message);
  }

  sendClipboardSet(text: string, paste?: boolean): void {
    const message: ControlMessage = {
      type: "clipboard_set",
      text,
      paste: paste || false,
      timestamp: Date.now(),
    };
    this.sendControlMessage(message);
  }

  requestKeyframe(): void {
    const now = Date.now();
    if (now - this.lastKeyframeRequest < this.KEYFRAME_THROTTLE_MS) {
      return;
    }

    this.lastKeyframeRequest = now;
    const message: ControlMessage = {
      type: "reset_video",
      timestamp: now,
    };
    this.sendControlMessage(message);
  }

  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void {
    // Convert mouse event to touch event for Android
    const { x, y } = this.normalizeCoordinates(
      event.clientX,
      event.clientY,
      event.target as HTMLElement
    );

    this.sendTouchEvent(x, y, action);
  }

  handleTouchEvent(event: TouchEvent, action: "down" | "up" | "move"): void {
    if (event.touches.length === 0) return;

    const touch = event.touches[0];
    const { x, y } = this.normalizeCoordinates(
      touch.clientX,
      touch.clientY,
      event.target as HTMLElement
    );

    this.sendTouchEvent(x, y, action);
  }

  /**
   * Send control message via data channel
   */
  private sendControlMessage(message: ControlMessage): void {
    if (!this.dataChannel || this.dataChannel.readyState !== "open") {
      // Queue message for later
      this.pendingControlMessages.push(message);
      return;
    }

    try {
      this.dataChannel.send(JSON.stringify(message));
    } catch (error) {
      this.handleError(error as Error, "WebRTCClient", "sendControlMessage");
    }
  }

  /**
   * Get video element for external access
   */
  getVideoElement(): HTMLVideoElement | null {
    return this.videoElement;
  }

  /**
   * Setup video element when it becomes available
   * This method can be called by React component when video element is ready
   */
  setupVideoElementWhenReady(): void {
    if (this.pendingVideoStream) {
      this.setupVideoTrack(this.pendingVideoStream);
    }
  }

  /**
   * Start stats monitoring
   */
  private startStats(): void {
    if (this.statsInterval) {
      clearInterval(this.statsInterval);
    }

    // Update stats every second
    this.statsInterval = window.setInterval(() => {
      this.updateStats();
    }, 1000);

    // Start stall detection
    this.startStallDetection();
    // Start ping measurement for accurate latency
    this.startPingMeasurement();
  }

  /**
   * Start stall detection
   */
  private startStallDetection(): void {
    if (this.stallCheckInterval) {
      clearInterval(this.stallCheckInterval);
    }

    this.lastVideoTime = this.videoElement?.currentTime || 0;

    // Check for stalls every 2 seconds
    this.stallCheckInterval = window.setInterval(() => {
      this.checkForVideoStall();
    }, 2000);
  }

  /**
   * Stop stall detection
   */
  private stopStallDetection(): void {
    if (this.stallCheckInterval) {
      clearInterval(this.stallCheckInterval);
      this.stallCheckInterval = null;
    }
  }

  /**
   * Check for video stalls and request keyframe if needed
   */
  private checkForVideoStall(): void {
    if (!this.videoElement || this.videoElement.paused) return;

    const currentTime = this.videoElement.currentTime;
    const timeDiff = currentTime - this.lastVideoTime;

    // If video time hasn't advanced by at least 0.1 seconds in 2 seconds, consider it stalled
    if (timeDiff < 0.1) {
      console.log("[WebRTC] Video appears stalled, requesting keyframe");
      this.requestKeyframe();
    }

    this.lastVideoTime = currentTime;
  }

  // Ping measurement methods are now inherited from BaseClient

  /**
   * Measure ping latency - implementation for WebRTC client
   */
  protected measurePing(): void {
    if (!this.dataChannel || this.dataChannel.readyState !== "open") {
      return;
    }

    const pingStart = performance.now();
    const pingId = Math.random().toString(36).substring(2, 11);

    // Send ping message
    this.dataChannel.send(
      JSON.stringify({
        type: "ping",
        id: pingId,
        timestamp: pingStart,
      })
    );

    // Store ping start time
    this.pendingPings.set(pingId, pingStart);

    // Clean up old pings after 5 seconds
    setTimeout(() => {
      this.pendingPings.delete(pingId);
    }, 5000);
  }

  /**
   * Update stats using WebRTC stats API
   */
  private async updateStats(): Promise<void> {
    if (!this.pc) return;

    try {
      const stats = await this.pc.getStats();
      let fps = 0;
      let resolution = "";
      let webrtcLatency = 0;

      stats.forEach((report: any) => {
        if (
          report.type === "inbound-rtp" &&
          (report.mediaType === "video" || report.kind === "video")
        ) {
          const width = report.frameWidth || 0;
          const height = report.frameHeight || 0;

          // Use direct framesPerSecond if available (most reliable)
          if (report.framesPerSecond) {
            fps = Math.round(report.framesPerSecond);
          }
          // Fallback: calculate FPS from frames decoded difference
          else if (report.framesDecoded) {
            const currentTime = Date.now();
            const currentFramesDecoded = report.framesDecoded || 0;

            if (this.lastFramesDecoded > 0 && this.lastStatsTime > 0) {
              const timeDiff = (currentTime - this.lastStatsTime) / 1000; // in seconds
              const framesDiff = currentFramesDecoded - this.lastFramesDecoded;
              if (timeDiff > 0 && framesDiff >= 0) {
                fps = Math.round(framesDiff / timeDiff);
              }
            }

            this.lastFramesDecoded = currentFramesDecoded;
            this.lastStatsTime = currentTime;
          }

          if (width && height) {
            resolution = `${width}x${height}`;
          }
        }

        // Get latency from candidate-pair stats (as fallback)
        if (
          report.type === "candidate-pair" &&
          report.state === "succeeded" &&
          report.currentRoundTripTime
        ) {
          webrtcLatency = Math.round(report.currentRoundTripTime * 1000); // Convert to ms
        }
      });

      // Use ping-pong latency if available, otherwise use WebRTC latency
      const latency = this.getAverageLatency() || webrtcLatency;

      this.onStatsUpdate?.({ fps, resolution, latency });
    } catch (err) {
      console.warn("Failed to get WebRTC stats:", err);
    }
  }

  /**
   * Get audio element for external access
   */
  getAudioElement(): HTMLAudioElement | null {
    return this.audioElement;
  }
}
