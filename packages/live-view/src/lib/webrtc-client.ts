import { ControlMessage, SignalingMessage } from "../types";

export class WebRTCClient {
  private ws: WebSocket | null = null;
  private pc: RTCPeerConnection | null = null;
  private dataChannel: RTCDataChannel | null = null;
  private currentDevice: string | null = null;
  private isConnected: boolean = false;
  private statsInterval: number | null = null;
  public isMouseDragging: boolean = false;
  private lastMouseTime: number = 0;
  private videoElement: HTMLVideoElement | null = null;
  private audioElement: HTMLAudioElement | null = null;

  // Reconnection state
  private isReconnecting: boolean = false;
  private reconnectAttempts: number = 0;
  private readonly maxReconnectAttempts: number = 30; // Increase for backend restarts
  private reconnectTimer: number | null = null;
  private lastConnectedDevice: string | null = null;

  // Callbacks
  private onConnectionStateChange?: (
    state: "connecting" | "connected" | "disconnected" | "error",
    message?: string
  ) => void;
  private onError?: (error: Error) => void;
  private onStatsUpdate?: (stats: any) => void;

  // Android key codes
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

  async connect(deviceSerial: string, wsUrl: string): Promise<void> {
    console.log(`[WebRTC] Connecting to device: ${deviceSerial}`);
    console.log(`[WebRTC] WebSocket URL: ${wsUrl}`);

    // Always disconnect first to ensure clean state
    if (this.isConnected || this.pc || this.ws) {
      console.log("[WebRTC] Cleaning up existing connection");
      await this.disconnect();
      // Wait for cleanup to complete
      await new Promise((resolve) => setTimeout(resolve, 500));
    }

    this.currentDevice = deviceSerial;
    this.lastConnectedDevice = deviceSerial;
    this.isReconnecting = false;
    this.reconnectAttempts = 0;
    this.onConnectionStateChange?.("connecting", "正在连接设备...");

    try {
      console.log("[WebRTC] Starting WebRTC connection establishment");
      await this.establishWebRTCConnection(deviceSerial, wsUrl);
    } catch (error) {
      console.error("[WebRTC] Connection failed:", error);

      // Check if it's a connection closed error that we can retry
      const errorMsg = (error as Error).message;
      if (
        errorMsg.includes("connection closed") ||
        errorMsg.includes("InvalidStateError")
      ) {
        console.log(
          "[WebRTC] Connection closed error, will retry automatically"
        );
        this.onConnectionStateChange?.(
          "disconnected",
          "连接已关闭，正在重连..."
        );
        return; // Don't throw error, let automatic reconnection handle it
      }

      this.onError?.(error as Error);
      this.onConnectionStateChange?.("error", "连接失败");
      throw error;
    }
  }

  private async establishWebRTCConnection(
    deviceSerial: string,
    wsUrl: string
  ): Promise<void> {
    const fullWsUrl = `${wsUrl}?device=${deviceSerial}`;
    console.log(`[WebRTC] Creating WebSocket connection to: ${fullWsUrl}`);
    this.ws = new WebSocket(fullWsUrl);

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
    this.setupDataChannel();
    console.log("[WebRTC] Created data channel: control");

    // Add transceivers
    const videoTransceiver = this.pc.addTransceiver("video", {
      direction: "recvonly",
    });
    const audioTransceiver = this.pc.addTransceiver("audio", {
      direction: "recvonly",
    });

    // Set reasonable low latency hints (not ultra-aggressive)
    if ("playoutDelayHint" in videoTransceiver.receiver) {
      (videoTransceiver.receiver as any).playoutDelayHint = 0.1; // 100ms instead of 0
    }
    if ("playoutDelayHint" in audioTransceiver.receiver) {
      (audioTransceiver.receiver as any).playoutDelayHint = 0.1; // 100ms instead of 0
    }

    this.setupWebRTCHandlers();
    this.setupWebSocketHandlers();

    // Wait for WebSocket to be open, then create offer
    await new Promise<void>((resolve, reject) => {
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

  private setupWebRTCHandlers(): void {
    if (!this.pc) return;

    this.pc.ontrack = (event) => {
      console.log(
        "[WebRTC] Track received:",
        event.track.kind,
        "Track ID:",
        event.track.id
      );
      if (event.track.kind === "video" && this.videoElement) {
        console.log("[WebRTC] Video track received, setting up playback");
        event.track.enabled = true;
        
        // Basic video element setup
        this.videoElement.autoplay = true;
        this.videoElement.muted = false;
        this.videoElement.playsInline = true;
        this.videoElement.controls = false;
        this.videoElement.preload = "auto";
        this.videoElement.srcObject = event.streams[0];

        // Basic styling
        this.videoElement.style.objectFit = "contain";
        this.videoElement.style.background = "black";
        
        console.log("[WebRTC] Video srcObject set");

        this.videoElement.onloadedmetadata = () => {
          if (!this.videoElement) return;
          const width = this.videoElement.videoWidth;
          const height = this.videoElement.videoHeight;
          console.log("[WebRTC] Video metadata loaded:", `${width}x${height}`);
          if (width && height) {
            this.onStatsUpdate?.({ resolution: `${width}x${height}` });
          }
        };

        this.videoElement.onplaying = () => {
          // Reset stall detection when video starts playing
          this.lastVideoTime = this.videoElement?.currentTime || 0;
        };

        this.onConnectionStateChange?.("connected", undefined);
        this.isConnected = true;
        this.startStats();
      } else if (event.track.kind === "audio") {
        console.log("[WebRTC] Audio track received");
        this.setupAudioPlayback(event.track, event.streams[0]);
      }
    };

    this.pc.onicecandidate = (event) => {
      if (event.candidate && this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(
          JSON.stringify({
            type: "ice-candidate",
            deviceSerial: this.currentDevice,
            candidate: event.candidate,
          })
        );
      }
    };

    this.pc.ondatachannel = (event) => {
      console.log(
        "[WebRTC] Data channel received from server:",
        event.channel.label
      );
      // Only use server's data channel if we don't have one
      if (!this.dataChannel) {
        this.dataChannel = event.channel;
        this.setupDataChannel();
      }
    };

    this.pc.onconnectionstatechange = () => {
      if (!this.pc) return;
      console.log("[WebRTC] Connection state:", this.pc.connectionState);
      if (
        this.pc.connectionState === "failed" ||
        this.pc.connectionState === "disconnected"
      ) {
        this.isConnected = false;
        // Don't show error immediately, try to reconnect first
        if (this.lastConnectedDevice && !this.isReconnecting) {
          console.log("[WebRTC] Connection lost, starting reconnection...");
          this.onConnectionStateChange?.(
            "connecting",
            "Connection lost, reconnecting..."
          );
          this.startReconnection();
        } else if (!this.lastConnectedDevice) {
          this.onConnectionStateChange?.("error", "Connection lost");
        }
      } else if (this.pc.connectionState === "connected") {
        console.log("[WebRTC] Peer connection established successfully");
      }
    };
  }

  private setupAudioPlayback(
    track: MediaStreamTrack,
    stream: MediaStream
  ): void {
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
    this.audioElement.srcObject = stream || new MediaStream([track]);
    track.enabled = true;
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
      this.onError?.(new Error("音频播放失败，点击页面启用音频"));
    });
  }

  private setupWebSocketHandlers(): void {
    if (!this.ws) return;

    this.ws.onmessage = async (event) => {
      const message: SignalingMessage = JSON.parse(event.data);
      await this.handleSignalingMessage(message);
    };

    this.ws.onclose = () => {
      // WebSocket closed - expected after WebRTC connection established
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      this.onError?.(new Error("WebSocket connection error"));
    };
  }

  private async handleSignalingMessage(
    message: SignalingMessage
  ): Promise<void> {
    if (!this.pc) return;

    console.log("[WebRTC] Received signaling message:", message.type);

    switch (message.type) {
      case "offer":
        console.log("[WebRTC] Setting remote offer");
        await this.pc.setRemoteDescription(
          new RTCSessionDescription({
            type: "offer",
            sdp: message.sdp!,
          })
        );
        const answer = await this.pc.createAnswer();
        await this.pc.setLocalDescription(answer);
        console.log("[WebRTC] Sending answer");
        this.sendSignalingMessage({
          type: "answer",
          sdp: answer.sdp,
        });
        break;

      case "answer":
        console.log("[WebRTC] Setting remote answer");
        console.log("[WebRTC] Message keys:", Object.keys(message));
        console.log("[WebRTC] Message.sdp exists:", !!message.sdp);
        console.log(
          "[WebRTC] Message.answer exists:",
          !!(message as any).answer
        );

        // Handle both formats: direct sdp or nested in answer object
        const sdp = message.sdp || (message as any).answer?.sdp;
        if (!sdp) {
          console.error(
            "[WebRTC] Answer missing SDP field, full message:",
            JSON.stringify(message)
          );
          break;
        }
        await this.pc.setRemoteDescription(
          new RTCSessionDescription({
            type: "answer",
            sdp: sdp,
          })
        );
        console.log("[WebRTC] Remote answer set successfully");
        break;

      case "ice-candidate":
        if (message.candidate) {
          console.log("[WebRTC] Adding ICE candidate");
          await this.pc.addIceCandidate(new RTCIceCandidate(message.candidate));
        }
        break;

      case "error":
        console.error("[WebRTC] Server error:", message.error);
        const errorMsg = message.error || "Unknown server error";

        // Handle specific connection errors that need reconnection
        if (
          errorMsg.includes("connection closed") ||
          errorMsg.includes("InvalidStateError") ||
          errorMsg.includes("InvalidModificationError") ||
          errorMsg.includes("invalid proposed signaling state")
        ) {
          console.log(
            "[WebRTC] Connection error detected, will attempt reconnection:",
            errorMsg
          );
          this.isConnected = false;

          // Clean up current connection before reconnecting
          if (this.pc) {
            try {
              this.pc.close();
            } catch (e) {
              console.warn(
                "[WebRTC] Error closing peer connection during error recovery:",
                e
              );
            }
            this.pc = null;
          }

          // Use the standard reconnection mechanism
          if (
            this.currentDevice &&
            this.lastConnectedDevice &&
            !this.isReconnecting
          ) {
            this.onConnectionStateChange?.(
              "connecting",
              "Connection error, reconnecting..."
            );

            // Wait a bit before reconnecting to allow server to clean up
            setTimeout(() => {
              this.startReconnection();
            }, 500);
          }

          // Don't trigger error callback for recoverable errors
          return;
        }

        this.onError?.(new Error(errorMsg));
        this.onConnectionStateChange?.("error", errorMsg);
        break;

      default:
        console.log("[WebRTC] Unknown message type:", message.type);
    }
  }

  private sendSignalingMessage(message: SignalingMessage): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }

  private setupDataChannel(): void {
    if (!this.dataChannel) return;

    this.dataChannel.onopen = () => {
      console.log("[WebRTC] Data channel opened");
      setTimeout(() => this.requestKeyframe(), 500);
    };

    this.dataChannel.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        
        // Handle ping responses for latency measurement
        if (message.type === "pong" && message.id && this.pendingPings?.has(message.id)) {
          const pingStart = this.pendingPings.get(message.id);
          if (pingStart) {
            const latency = performance.now() - pingStart;

            // Store ping time for averaging
            if (!this.pingTimes) this.pingTimes = [];
            this.pingTimes.push(latency);

            // Keep only last 5 ping times
            if (this.pingTimes.length > 5) {
              this.pingTimes.shift();
            }

            // Update latency display with average
            const avgLatency = this.pingTimes.reduce((a, b) => a + b, 0) / this.pingTimes.length;
            this.onStatsUpdate?.({ latency: Math.round(avgLatency) });

            this.pendingPings.delete(message.id);
          }
        }
      } catch (e) {
        // Not JSON
      }
    };
  }

  // Ping measurement properties
  private pingTimes: number[] = [];
  private pingInterval: number | null = null;
  private pendingPings: Map<string, number> | null = null;

  private startPingMeasurement(): void {
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

  private measurePing(): void {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') {
      return;
    }

    const pingStart = performance.now();
    const pingId = Math.random().toString(36).substring(2, 11);

    // Send ping message
    this.dataChannel.send(JSON.stringify({
      type: 'ping',
      id: pingId,
      timestamp: pingStart
    }));

    // Store ping start time
    if (!this.pendingPings) {
      this.pendingPings = new Map();
    }
    this.pendingPings.set(pingId, pingStart);

    // Clean up old pings after 5 seconds
    setTimeout(() => {
      this.pendingPings?.delete(pingId);
    }, 5000);
  }

  private stopPingMeasurement(): void {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
    this.pendingPings?.clear();
  }

  sendControlMessage(message: ControlMessage): void {
    if (!this.dataChannel) {
      console.warn("[WebRTC] Data channel not available");
      return;
    }

    if (this.dataChannel.readyState !== "open") {
      console.warn(
        "[WebRTC] Data channel not open, state:",
        this.dataChannel.readyState
      );
      return;
    }

    // Check if peer connection is still valid
    if (
      !this.pc ||
      this.pc.connectionState === "closed" ||
      this.pc.connectionState === "failed"
    ) {
      console.warn("[WebRTC] Peer connection not ready for control message", {
        connectionState: this.pc?.connectionState,
      });
      return;
    }

    const msgWithTimestamp = {
      ...message,
      timestamp: Date.now(),
    };

    // Only log non-movement control messages
    if (message.type !== "touch" || message.action !== "move") {
      console.log("[WebRTC] Sending control message:", msgWithTimestamp);
    }

    // Handle clipboard messages with binary data specially
    if (typeof message.type === "number" && message.data) {
      // For clipboard messages, send as binary data
      const binaryMessage = {
        type: message.type,
        data: Array.from(message.data), // Convert Uint8Array to regular array for JSON
        timestamp: Date.now(),
      };
      this.dataChannel.send(JSON.stringify(binaryMessage));
    } else {
      // For regular messages, send as JSON
      this.dataChannel.send(JSON.stringify(msgWithTimestamp));
    }
  }

  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState: number = 0
  ): void {
    console.log("[WebRTC] Sending key event:", { keycode, action, metaState });
    this.sendControlMessage({
      type: "key",
      action,
      keycode,
      metaState,
    });
  }

  sendClipboardSet(text: string, paste: boolean = false): void {
    console.log("[WebRTC] Sending clipboard set:", { text, paste });
    this.sendControlMessage({
      type: "clipboard_set",
      text,
      paste,
    });
  }

  sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure: number = 1.0
  ): void {
    this.sendControlMessage({
      type: "touch",
      action,
      x,
      y,
      pressure: action === "down" || action === "move" ? pressure : 0,
      pointerId: 0,
    });
  }

  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void {
    if (
      !this.isConnected ||
      !this.dataChannel ||
      !this.videoElement ||
      !this.pc
    ) {
      // Silently return - connection not ready
      return;
    }

    // Only handle left mouse button (button 0) for touch simulation
    // Right click (button 2) and middle click (button 1) should be ignored
    if ((action === "down" || action === "up") && event.button !== 0) {
      console.log(`[WebRTC] Ignoring non-left mouse button: ${event.button}`);
      return;
    }

    // Check if peer connection is in a valid state
    if (
      this.pc.connectionState === "closed" ||
      this.pc.connectionState === "failed"
    ) {
      // Silently return - peer connection not ready
      return;
    }

    // Handle drag state
    if (action === "down") {
      this.isMouseDragging = true;
      this.lastMouseTime = 0; // Reset throttle
      event.preventDefault(); // Prevent text selection during drag
    } else if (action === "up") {
      this.isMouseDragging = false;
    } else if (action === "move" && !this.isMouseDragging) {
      // Only send move events when dragging (simulating touch drag)
      return;
    }

    // Throttle move events to reduce latency (max 120 events per second for better responsiveness)
    if (action === "move") {
      const now = Date.now();
      if (this.lastMouseTime && now - this.lastMouseTime < 8) {
        // 8ms = ~120fps for smoother interaction
        return;
      }
      this.lastMouseTime = now;
    }

    // Calculate the actual video display area within the video element
    // This is needed because object-fit: contain may add letterboxing/pillarboxing
    const rect = this.videoElement.getBoundingClientRect();
    const videoWidth = this.videoElement.videoWidth;
    const videoHeight = this.videoElement.videoHeight;

    if (!videoWidth || !videoHeight) {
      // Video not loaded yet, use simple calculation
      const x = (event.clientX - rect.left) / rect.width;
      const y = (event.clientY - rect.top) / rect.height;

      const clampedX = Math.max(0, Math.min(1, x));
      const clampedY = Math.max(0, Math.min(1, y));

      this.sendControlMessage({
        type: "touch",
        action,
        x: clampedX,
        y: clampedY,
        pressure:
          action === "down" || (action === "move" && this.isMouseDragging)
            ? 1.0
            : 0.0,
        pointerId: 0,
      });
      return;
    }

    // Calculate the actual display dimensions considering aspect ratio
    const containerAspect = rect.width / rect.height;
    const videoAspect = videoWidth / videoHeight;

    let displayWidth: number;
    let displayHeight: number;
    let offsetX: number;
    let offsetY: number;

    if (containerAspect > videoAspect) {
      // Container is wider than video - black bars on left/right (pillarboxing)
      displayHeight = rect.height;
      displayWidth = displayHeight * videoAspect;
      offsetX = (rect.width - displayWidth) / 2;
      offsetY = 0;
    } else {
      // Container is taller than video - black bars on top/bottom (letterboxing)
      displayWidth = rect.width;
      displayHeight = displayWidth / videoAspect;
      offsetX = 0;
      offsetY = (rect.height - displayHeight) / 2;
    }

    // Calculate relative position within the actual video display area
    const relativeX = event.clientX - rect.left - offsetX;
    const relativeY = event.clientY - rect.top - offsetY;

    // Convert to normalized coordinates (0-1)
    const x = relativeX / displayWidth;
    const y = relativeY / displayHeight;

    // Only send touch events if the click is within the actual video display area
    if (x < 0 || x > 1 || y < 0 || y > 1) {
      // Click is in the black bars (letterbox/pillarbox), ignore it
      console.log(`[WebRTC] Click outside video area ignored: x=${x.toFixed(3)}, y=${y.toFixed(3)}`);
      return;
    }

    // Ensure coordinates are within bounds (should already be, but safety check)
    const clampedX = Math.max(0, Math.min(1, x));
    const clampedY = Math.max(0, Math.min(1, y));

    this.sendControlMessage({
      type: "touch",
      action,
      x: clampedX,
      y: clampedY,
      pressure:
        action === "down" || (action === "move" && this.isMouseDragging)
          ? 1.0
          : 0.0,
      pointerId: 0, // Use 0 for mouse to simulate touch
    });
  }

  handleTouchEvent(event: TouchEvent, action: "down" | "up" | "move"): void {
    if (!this.isConnected || !this.dataChannel || !this.videoElement) return;

    event.preventDefault();

    const rect = this.videoElement.getBoundingClientRect();
    const touch = event.touches[0] || event.changedTouches[0];
    const videoWidth = this.videoElement.videoWidth;
    const videoHeight = this.videoElement.videoHeight;

    if (!videoWidth || !videoHeight) {
      // Video not loaded yet, use simple calculation
      const x = (touch.clientX - rect.left) / rect.width;
      const y = (touch.clientY - rect.top) / rect.height;

      this.sendControlMessage({
        type: "touch",
        action,
        x: Math.max(0, Math.min(1, x)),
        y: Math.max(0, Math.min(1, y)),
        pressure: action === "down" || action === "move" ? 1.0 : 0.0,
        pointerId: 0,
      });
      return;
    }

    // Calculate the actual display dimensions considering aspect ratio
    const containerAspect = rect.width / rect.height;
    const videoAspect = videoWidth / videoHeight;

    let displayWidth: number;
    let displayHeight: number;
    let offsetX: number;
    let offsetY: number;

    if (containerAspect > videoAspect) {
      // Container is wider than video - black bars on left/right (pillarboxing)
      displayHeight = rect.height;
      displayWidth = displayHeight * videoAspect;
      offsetX = (rect.width - displayWidth) / 2;
      offsetY = 0;
    } else {
      // Container is taller than video - black bars on top/bottom (letterboxing)
      displayWidth = rect.width;
      displayHeight = displayWidth / videoAspect;
      offsetX = 0;
      offsetY = (rect.height - displayHeight) / 2;
    }

    // Calculate relative position within the actual video display area
    const relativeX = touch.clientX - rect.left - offsetX;
    const relativeY = touch.clientY - rect.top - offsetY;

    // Convert to normalized coordinates (0-1)
    const x = relativeX / displayWidth;
    const y = relativeY / displayHeight;

    // Only send touch events if the touch is within the actual video display area
    if (x < 0 || x > 1 || y < 0 || y > 1) {
      // Touch is in the black bars (letterbox/pillarbox), ignore it
      console.log(`[WebRTC] Touch outside video area ignored: x=${x.toFixed(3)}, y=${y.toFixed(3)}`);
      return;
    }

    // Ensure coordinates are within bounds (should already be, but safety check)
    const clampedX = Math.max(0, Math.min(1, x));
    const clampedY = Math.max(0, Math.min(1, y));

    this.sendControlMessage({
      type: "touch",
      action,
      x: clampedX,
      y: clampedY,
      pressure: action === "down" || action === "move" ? 1.0 : 0.0,
      pointerId: 0,
    });
  }

  // Wheel event handling is now done in the React component with accumulation
  // This method is kept for compatibility but should not be called directly
  handleWheelEvent(_event: WheelEvent): void {
    console.warn(
      "[Wheel] handleWheelEvent called directly - this should be handled by React component"
    );
  }

  requestKeyframe(): void {
    this.sendControlMessage({ type: "reset_video" });
  }

  // Check for video stalls and request keyframe if needed
  private checkForVideoStall(): void {
    if (!this.videoElement || this.videoElement.paused) return;

    const currentTime = this.videoElement.currentTime;
    const timeDiff = currentTime - this.lastVideoTime;

    // If video time hasn't advanced by at least 0.1 seconds in 2 seconds, consider it stalled
    if (timeDiff < 0.1) {
      console.log('[WebRTC] Video appears stalled, requesting keyframe');
      this.requestKeyframe();
    }

    this.lastVideoTime = currentTime;
  }

  private lastVideoTime = 0;
  private stallCheckInterval: number | null = null;

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

  private stopStallDetection(): void {
    if (this.stallCheckInterval) {
      clearInterval(this.stallCheckInterval);
      this.stallCheckInterval = null;
    }
  }

  private lastFramesDecoded = 0;
  private lastFramesReceived = 0;
  private lastStatsTime = 0;

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
        if (report.type === "candidate-pair" && report.state === "succeeded" && report.currentRoundTripTime) {
          webrtcLatency = Math.round(report.currentRoundTripTime * 1000); // Convert to ms
        }
      });

      // Use ping-pong latency if available, otherwise use WebRTC latency
      const latency = this.pingTimes.length > 0 ? 
        Math.round(this.pingTimes.reduce((a, b) => a + b, 0) / this.pingTimes.length) : 
        webrtcLatency;

      this.onStatsUpdate?.({ fps, resolution, latency });
    } catch (err) {
      console.warn("Failed to get WebRTC stats:", err);
    }
  }

  private startReconnection(): void {
    if (this.isReconnecting || !this.lastConnectedDevice) return;
    this.isReconnecting = true;
    this.reconnectAttempts = 0;
    this.attemptReconnection();
  }

  private async attemptReconnection(): Promise<void> {
    if (!this.isReconnecting || !this.lastConnectedDevice) return;

    this.reconnectAttempts++;
    // Use shorter delays for faster reconnection: 1s, 2s, 3s, 5s, then 5s repeatedly
    const delays = [1000, 2000, 3000, 5000];
    const delay =
      delays[Math.min(this.reconnectAttempts - 1, delays.length - 1)];

    this.onConnectionStateChange?.(
      "connecting",
      `Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`
    );

    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.isReconnecting = false;
      this.reconnectAttempts = 0;
      this.onConnectionStateChange?.(
        "error",
        "Reconnection failed after maximum attempts"
      );
      return;
    }

    // Actually attempt to reconnect
    try {
      // Extract base WebSocket URL
      const baseUrl = this.ws?.url?.split("?")[0] || "ws://localhost:29888/ws";

      console.log(
        `[WebRTC] Reconnection attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts}`
      );

      // Try to reconnect
      await this.connect(this.lastConnectedDevice, baseUrl);

      // If successful, reset counters
      this.isReconnecting = false;
      this.reconnectAttempts = 0;
      console.log("[WebRTC] Reconnection successful");
    } catch (error) {
      console.log(
        `[WebRTC] Reconnection attempt ${this.reconnectAttempts} failed:`,
        error
      );

      // Schedule next attempt
      this.reconnectTimer = window.setTimeout(() => {
        this.attemptReconnection();
      }, delay);
    }
  }

  async disconnect(isManual: boolean = true): Promise<void> {
    console.log("[WebRTC] Disconnecting...");

    if (isManual) {
      this.lastConnectedDevice = null;
      this.stopReconnection();
    }

    this.isConnected = false;
    this.onConnectionStateChange?.("disconnected", undefined);

    // Stop all intervals
    if (this.statsInterval) {
      clearInterval(this.statsInterval);
      this.statsInterval = null;
    }

    this.stopStallDetection();
    this.stopPingMeasurement();

    // Close data channel
    if (this.dataChannel) {
      try {
        this.dataChannel.close();
      } catch (e) {
        console.warn("[WebRTC] Error closing data channel:", e);
      }
      this.dataChannel = null;
    }

    // Close peer connection
    if (this.pc) {
      try {
        this.pc.close();
      } catch (e) {
        console.warn("[WebRTC] Error closing peer connection:", e);
      }
      this.pc = null;
    }

    // Close WebSocket
    if (this.ws) {
      try {
        this.ws.close();
      } catch (e) {
        console.warn("[WebRTC] Error closing WebSocket:", e);
      }
      this.ws = null;
    }

    // Clear video element
    if (this.videoElement) {
      this.videoElement.srcObject = null;
    }

    // Clear audio element
    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.srcObject = null;
      this.audioElement.remove();
      this.audioElement = null;
    }

    // Reset state
    this.currentDevice = null;
    this.isMouseDragging = false;
    this.lastFramesDecoded = 0;
    this.lastFramesReceived = 0;
    this.lastStatsTime = 0;
    this.lastVideoTime = 0;

    console.log("[WebRTC] Disconnect completed");
  }

  private stopReconnection(): void {
    this.isReconnecting = false;
    this.reconnectAttempts = 0;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  cleanup(): void {
    this.stopReconnection();
    this.stopStallDetection();
    this.stopPingMeasurement();
    if (this.isConnected || this.pc || this.ws) {
      this.disconnect(true);
    }
  }
}
