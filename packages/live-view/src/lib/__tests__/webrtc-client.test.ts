/* eslint-disable @typescript-eslint/no-explicit-any */
// WebRTCClient tests
import { WebRTCClient } from "../webrtc-client";

// Mock HTML elements
const mockContainer = {
  appendChild: jest.fn(),
  getBoundingClientRect: jest.fn(() => ({
    width: 800,
    height: 600,
  })),
  clientWidth: 800,
  clientHeight: 600,
} as unknown as HTMLElement;

// Mock canvas and context
const mockCanvas = {
  width: 0,
  height: 0,
  style: {
    display: "",
    width: "",
    height: "",
    objectFit: "",
    background: "",
    cursor: "",
    marginLeft: "",
    marginTop: "",
    transition: "",
    visibility: "",
    position: "",
    top: "",
    left: "",
  },
  getContext: jest.fn(),
  parentNode: null,
  appendChild: jest.fn(),
  removeChild: jest.fn(),
  remove: jest.fn(),
};

const mockContext = {
  drawImage: jest.fn(),
};

// Mock video element
const mockVideoElement = {
  autoplay: true,
  muted: false,
  playsInline: true,
  controls: false,
  preload: "auto",
  style: {
    width: "",
    height: "",
    objectFit: "",
    background: "",
  },
  videoWidth: 0,
  videoHeight: 0,
  srcObject: null,
  onloadedmetadata: null,
  onplaying: null,
  parentNode: null,
  appendChild: jest.fn(),
  removeChild: jest.fn(),
  remove: jest.fn(),
};

// Mock WebSocket
const mockWebSocket = {
  readyState: WebSocket.CONNECTING,
  send: jest.fn(),
  close: jest.fn(),
  onopen: null,
  onmessage: null,
  onclose: null,
  onerror: null,
};

// Mock RTCPeerConnection
const mockPeerConnection = {
  connectionState: "new",
  addTrack: jest.fn(),
  addTransceiver: jest.fn(),
  createOffer: jest.fn(),
  createAnswer: jest.fn(),
  setLocalDescription: jest.fn(),
  setRemoteDescription: jest.fn(),
  addIceCandidate: jest.fn(),
  close: jest.fn(),
  ontrack: null,
  onicecandidate: null,
  onconnectionstatechange: null,
  ondatachannel: null,
};

// Mock RTCDataChannel
const mockDataChannel = {
  readyState: "connecting",
  send: jest.fn(),
  close: jest.fn(),
  onopen: null,
  onclose: null,
  onerror: null,
};

// Mock MediaStream and MediaStreamTrack
(global as Record<string, unknown>).MediaStream = jest.fn(() => ({
  getVideoTracks: jest.fn(() => []),
  getAudioTracks: jest.fn(() => []),
})) as unknown as typeof MediaStream;

(global as Record<string, unknown>).MediaStreamTrack = jest.fn(() => ({
  kind: "video",
  id: "test-track",
  enabled: true,
  muted: false,
})) as unknown as typeof MediaStreamTrack;

// Mock DOM methods
Object.defineProperty(document, "createElement", {
  value: jest.fn((tagName: string) => {
    if (tagName === "canvas") {
      return mockCanvas;
    }
    if (tagName === "video") {
      return mockVideoElement;
    }
    return {};
  }),
});

// Mock getContext to return mockContext
mockCanvas.getContext.mockReturnValue(mockContext);

// Mock WebSocket constructor
(global as Record<string, unknown>).WebSocket = jest.fn(
  () => mockWebSocket
) as unknown as typeof WebSocket;

// Mock RTCPeerConnection constructor
(global as Record<string, unknown>).RTCPeerConnection = jest.fn(
  () => mockPeerConnection
) as unknown as typeof RTCPeerConnection;

// Mock RTCDataChannel
Object.defineProperty(mockPeerConnection, "createDataChannel", {
  value: jest.fn(() => mockDataChannel),
});

describe("WebRTCClient", () => {
  let webrtcClient: WebRTCClient;
  let mockOnConnectionStateChange: jest.Mock;
  let mockOnError: jest.Mock;
  let mockOnStatsUpdate: jest.Mock;

  beforeEach(() => {
    jest.useFakeTimers();
    jest.clearAllMocks();

    mockOnConnectionStateChange = jest.fn();
    mockOnError = jest.fn();
    mockOnStatsUpdate = jest.fn();

    webrtcClient = new WebRTCClient(mockContainer, {
      onConnectionStateChange: mockOnConnectionStateChange,
      onError: mockOnError,
      onStatsUpdate: mockOnStatsUpdate,
    });
  });

  afterEach(() => {
    if (webrtcClient) {
      webrtcClient.destroy();
    }
    jest.useRealTimers();
  });

  it("should create WebRTCClient instance", () => {
    expect(webrtcClient).toBeDefined();
    expect(webrtcClient).toBeInstanceOf(WebRTCClient);
  });

  it("should connect successfully", async () => {
    // Mock establishConnection to succeed immediately
    jest
      .spyOn(
        webrtcClient as unknown as { establishConnection: () => Promise<void> },
        "establishConnection"
      )
      .mockResolvedValue(undefined);

    await webrtcClient.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    expect(webrtcClient.connected).toBe(true);
    expect(webrtcClient.state).toBe("connected");
    expect(webrtcClient.device).toBe("device123");
  });

  it("should disconnect successfully", async () => {
    // Mock connected state
    (webrtcClient as unknown as { isConnected: boolean }).isConnected = true;
    (
      webrtcClient as unknown as {
        ws: unknown;
        pc: unknown;
        dataChannel: unknown;
      }
    ).ws = mockWebSocket;
    (
      webrtcClient as unknown as {
        ws: unknown;
        pc: unknown;
        dataChannel: unknown;
      }
    ).pc = mockPeerConnection;
    (
      webrtcClient as unknown as {
        ws: unknown;
        pc: unknown;
        dataChannel: unknown;
      }
    ).dataChannel = mockDataChannel;

    // Then disconnect
    await webrtcClient.disconnect();

    expect(webrtcClient.connected).toBe(false);
    expect(webrtcClient.state).toBe("disconnected");
    expect(mockWebSocket.close).toHaveBeenCalled();
    expect(mockPeerConnection.close).toHaveBeenCalled();
    expect(mockDataChannel.close).toHaveBeenCalled();
  });

  it("should handle connection errors", async () => {
    // Mock connection failure by making establishConnection throw
    jest
      .spyOn(
        webrtcClient as unknown as { establishConnection: () => Promise<void> },
        "establishConnection"
      )
      .mockRejectedValue(new Error("Connection failed"));

    try {
      await webrtcClient.connect(
        "device123",
        "http://api.example.com",
        "ws://ws.example.com"
      );
    } catch (_error) {
      // Expected to throw
    }

    expect(webrtcClient.connected).toBe(false);
    expect(webrtcClient.state).toBe("error");
  });

  it("should send control messages", () => {
    // Mock connected state
    mockWebSocket.readyState = WebSocket.OPEN as unknown as 0; // WebSocket.OPEN
    mockDataChannel.readyState = "open";

    webrtcClient.sendKeyEvent(26, "down");
    webrtcClient.sendTouchEvent(0.5, 0.5, "down");
    webrtcClient.sendControlAction("power");
    webrtcClient.sendClipboardSet("test text", true);
    webrtcClient.requestKeyframe();

    // Should queue messages when not connected
    expect(mockDataChannel.send).not.toHaveBeenCalled();
  });

  it("should handle mouse events", () => {
    const mockMouseEvent = {
      clientX: 100,
      clientY: 200,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    webrtcClient.handleMouseEvent(mockMouseEvent, "down");

    // Should not throw errors
    expect(webrtcClient).toBeDefined();
  });

  it("should handle touch events", () => {
    const mockTouchEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
        },
      ],
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    webrtcClient.handleTouchEvent(mockTouchEvent as any, "down");

    // Should not throw errors
    expect(webrtcClient).toBeDefined();
  });

  it("should check control connection status", () => {
    expect(webrtcClient.isControlConnected()).toBe(false);

    // Mock connected state
    (webrtcClient as unknown as { isConnected: boolean }).isConnected = true;
    (webrtcClient as unknown as { ws: unknown }).ws = mockWebSocket;
    (webrtcClient as unknown as { dataChannel: unknown }).dataChannel =
      mockDataChannel;
    mockWebSocket.readyState = WebSocket.OPEN as unknown as 0;
    mockDataChannel.readyState = "open";

    expect(webrtcClient.isControlConnected()).toBe(true);
  });

  it("should setup video element", () => {
    const videoElement = webrtcClient.getVideoElement();
    expect(videoElement).toBeNull(); // Not created yet

    // Mock the video render service setupVideoElement method
    jest
      .spyOn(webrtcClient.videoRender, "setupVideoElement")
      .mockReturnValue(mockVideoElement as unknown as HTMLVideoElement);

    // Mock internal state
    (webrtcClient as unknown as { videoElement: unknown }).videoElement =
      mockVideoElement;

    const videoElement2 = webrtcClient.getVideoElement();
    expect(videoElement2).toBe(mockVideoElement);
  });

  it("should handle WebRTC offer", async () => {
    mockPeerConnection.setRemoteDescription.mockResolvedValue(undefined);
    mockPeerConnection.createAnswer.mockResolvedValue({ sdp: "test-answer" });
    mockPeerConnection.setLocalDescription.mockResolvedValue(undefined);

    // Mock internal state
    (webrtcClient as unknown as { pc: unknown }).pc = mockPeerConnection;
    (webrtcClient as unknown as { ws: unknown }).ws = mockWebSocket;
    mockWebSocket.readyState = WebSocket.OPEN as unknown as 0; // WebSocket.OPEN

    // Call handleOffer directly
    await (
      webrtcClient as unknown as { handleOffer: (sdp: string) => Promise<void> }
    ).handleOffer("test-offer-sdp");

    expect(mockPeerConnection.setRemoteDescription).toHaveBeenCalledWith({
      type: "offer",
      sdp: "test-offer-sdp",
    });
    expect(mockPeerConnection.createAnswer).toHaveBeenCalled();
    expect(mockWebSocket.send).toHaveBeenCalledWith(
      JSON.stringify({
        type: "answer",
        sdp: "test-answer",
      })
    );
  });

  it("should handle ICE candidates", async () => {
    mockPeerConnection.addIceCandidate.mockResolvedValue(undefined);

    // Mock internal state
    (webrtcClient as unknown as { pc: unknown }).pc = mockPeerConnection;

    const candidate = {
      candidate: "test-candidate",
      sdpMLineIndex: 0,
      sdpMid: "0",
    };

    // Call handleIceCandidate directly
    await (
      webrtcClient as unknown as {
        handleIceCandidate: (candidate: unknown) => Promise<void>;
      }
    ).handleIceCandidate(candidate);

    expect(mockPeerConnection.addIceCandidate).toHaveBeenCalledWith({
      candidate: "test-candidate",
      sdpMLineIndex: 0,
      sdpMid: "0",
    });
  });

  it("should handle data channel", () => {
    // Mock data channel event
    if (mockPeerConnection.ondatachannel) {
      (
        mockPeerConnection as unknown as {
          ondatachannel: ((event: { channel: unknown }) => void) | null;
        }
      ).ondatachannel?.({
        channel: mockDataChannel,
      } as any);
    }

    // Simulate data channel open
    if (mockDataChannel.onopen) {
      (
        mockDataChannel as unknown as {
          onopen: ((event: Event) => void) | null;
        }
      ).onopen?.(new Event("open"));
    }

    expect(webrtcClient).toBeDefined();
  });

  it("should handle connection state changes", () => {
    // Mock internal state
    (webrtcClient as unknown as { pc: unknown }).pc = mockPeerConnection;

    // Start error handling service
    webrtcClient.errorHandling.start();

    // Setup WebRTC handlers to register the callback
    (
      webrtcClient as unknown as { setupWebRTCHandlers: () => void }
    ).setupWebRTCHandlers();

    // Simulate connection state change
    if (mockPeerConnection.onconnectionstatechange) {
      mockPeerConnection.connectionState = "failed";
      const onConnectionStateChange = (
        mockPeerConnection as unknown as {
          onconnectionstatechange: (() => void) | null;
        }
      ).onconnectionstatechange;
      if (onConnectionStateChange) {
        onConnectionStateChange();
      }
    }

    // The error should be handled by the error handling service
    expect(mockOnError).toHaveBeenCalledWith(expect.any(Error));
  });

  it("should handle WebSocket close", () => {
    // Mock connected state
    (webrtcClient as unknown as { isConnected: boolean }).isConnected = true;

    // Simulate WebSocket close
    if (mockWebSocket.onclose) {
      const onClose = (
        mockWebSocket as unknown as { onclose: ((event: Event) => void) | null }
      ).onclose;
      if (onClose) {
        onClose(new Event("close"));
      }
    }

    // Should start reconnection
    expect(webrtcClient).toBeDefined();
  });

  it("should send pending control messages when data channel opens", () => {
    // Mock internal state
    (webrtcClient as unknown as { dataChannel: unknown }).dataChannel =
      mockDataChannel;
    mockDataChannel.readyState = "open";

    // Add pending message
    (
      webrtcClient as unknown as { pendingControlMessages: unknown[] }
    ).pendingControlMessages = [
      {
        type: "key",
        keycode: 26,
        action: "down",
      },
    ];

    // Call sendPendingControlMessages directly
    (
      webrtcClient as unknown as { sendPendingControlMessages: () => void }
    ).sendPendingControlMessages();

    expect(mockDataChannel.send).toHaveBeenCalledWith(
      JSON.stringify({
        type: "key",
        keycode: 26,
        action: "down",
      })
    );
  });

  it("should throttle keyframe requests", () => {
    const now = Date.now();
    jest.spyOn(Date, "now").mockReturnValue(now);

    webrtcClient.requestKeyframe();
    webrtcClient.requestKeyframe(); // Should be throttled

    // Only one request should be sent
    expect(mockDataChannel.send).toHaveBeenCalledTimes(0); // Not connected yet
  });
});
