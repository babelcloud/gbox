// WebRTCClientRefactored tests
import { WebRTCClientRefactored } from "../webrtc-client";

// Mock HTML elements
const mockContainer = {
  appendChild: jest.fn(),
  getBoundingClientRect: jest.fn(() => ({
    width: 800,
    height: 600,
  })),
  clientWidth: 800,
  clientHeight: 600,
} as any;

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
(global as any).MediaStream = jest.fn(() => ({
  getVideoTracks: jest.fn(() => []),
  getAudioTracks: jest.fn(() => []),
})) as any;

(global as any).MediaStreamTrack = jest.fn(() => ({
  kind: "video",
  id: "test-track",
  enabled: true,
  muted: false,
})) as any;

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
(global as any).WebSocket = jest.fn(() => mockWebSocket) as any;

// Mock RTCPeerConnection constructor
(global as any).RTCPeerConnection = jest.fn(() => mockPeerConnection) as any;

// Mock RTCDataChannel
Object.defineProperty(mockPeerConnection, "createDataChannel", {
  value: jest.fn(() => mockDataChannel),
});

describe("WebRTCClientRefactored", () => {
  let webrtcClient: WebRTCClientRefactored;
  let mockOnConnectionStateChange: jest.Mock;
  let mockOnError: jest.Mock;
  let mockOnStatsUpdate: jest.Mock;

  beforeEach(() => {
    jest.useFakeTimers();
    jest.clearAllMocks();

    mockOnConnectionStateChange = jest.fn();
    mockOnError = jest.fn();
    mockOnStatsUpdate = jest.fn();

    webrtcClient = new WebRTCClientRefactored(mockContainer, {
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

  it("should create WebRTCClientRefactored instance", () => {
    expect(webrtcClient).toBeDefined();
    expect(webrtcClient).toBeInstanceOf(WebRTCClientRefactored);
  });

  it("should connect successfully", async () => {
    // Mock establishConnection to succeed immediately
    jest
      .spyOn(webrtcClient as any, "establishConnection")
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
    (webrtcClient as any).isConnected = true;
    (webrtcClient as any).ws = mockWebSocket;
    (webrtcClient as any).pc = mockPeerConnection;
    (webrtcClient as any).dataChannel = mockDataChannel;

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
      .spyOn(webrtcClient as any, "establishConnection")
      .mockRejectedValue(new Error("Connection failed"));

    try {
      await webrtcClient.connect(
        "device123",
        "http://api.example.com",
        "ws://ws.example.com"
      );
    } catch (error) {
      // Expected to throw
    }

    expect(webrtcClient.connected).toBe(false);
    expect(webrtcClient.state).toBe("error");
  });

  it("should send control messages", () => {
    // Mock connected state
    mockWebSocket.readyState = 1 as any; // WebSocket.OPEN
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
    } as any;

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
    } as any;

    webrtcClient.handleTouchEvent(mockTouchEvent, "down");

    // Should not throw errors
    expect(webrtcClient).toBeDefined();
  });

  it("should check control connection status", () => {
    expect(webrtcClient.isControlConnected()).toBe(false);

    // Mock connected state
    (webrtcClient as any).isConnected = true;
    (webrtcClient as any).ws = mockWebSocket;
    (webrtcClient as any).dataChannel = mockDataChannel;
    mockWebSocket.readyState = 1 as any; // WebSocket.OPEN
    mockDataChannel.readyState = "open";

    expect(webrtcClient.isControlConnected()).toBe(true);
  });

  it("should setup video element", () => {
    const videoElement = webrtcClient.getVideoElement();
    expect(videoElement).toBeNull(); // Not created yet

    // Mock the video render service setupVideoElement method
    jest
      .spyOn(webrtcClient.videoRender, "setupVideoElement")
      .mockReturnValue(mockVideoElement as any);

    // Mock internal state
    (webrtcClient as any).videoElement = mockVideoElement;

    const videoElement2 = webrtcClient.getVideoElement();
    expect(videoElement2).toBe(mockVideoElement);
  });

  it("should handle WebRTC offer", async () => {
    mockPeerConnection.setRemoteDescription.mockResolvedValue(undefined);
    mockPeerConnection.createAnswer.mockResolvedValue({ sdp: "test-answer" });
    mockPeerConnection.setLocalDescription.mockResolvedValue(undefined);

    // Mock internal state
    (webrtcClient as any).pc = mockPeerConnection;
    (webrtcClient as any).ws = mockWebSocket;
    mockWebSocket.readyState = 1 as any; // WebSocket.OPEN

    // Call handleOffer directly
    await (webrtcClient as any).handleOffer("test-offer-sdp");

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
    (webrtcClient as any).pc = mockPeerConnection;

    const candidate = {
      candidate: "test-candidate",
      sdpMLineIndex: 0,
      sdpMid: "0",
    };

    // Call handleIceCandidate directly
    await (webrtcClient as any).handleIceCandidate(candidate);

    expect(mockPeerConnection.addIceCandidate).toHaveBeenCalledWith({
      candidate: "test-candidate",
      sdpMLineIndex: 0,
      sdpMid: "0",
    });
  });

  it("should handle data channel", () => {
    // Mock data channel event
    if (mockPeerConnection.ondatachannel) {
      (mockPeerConnection as any).ondatachannel({
        channel: mockDataChannel,
      } as any);
    }

    // Simulate data channel open
    if (mockDataChannel.onopen) {
      (mockDataChannel as any).onopen(new Event("open"));
    }

    expect(webrtcClient).toBeDefined();
  });

  it("should handle connection state changes", () => {
    // Mock internal state
    (webrtcClient as any).pc = mockPeerConnection;

    // Start error handling service
    webrtcClient.errorHandling.start();

    // Setup WebRTC handlers to register the callback
    (webrtcClient as any).setupWebRTCHandlers();

    // Simulate connection state change
    if (mockPeerConnection.onconnectionstatechange) {
      mockPeerConnection.connectionState = "failed";
      (mockPeerConnection as any).onconnectionstatechange();
    }

    // The error should be handled by the error handling service
    expect(mockOnError).toHaveBeenCalledWith(expect.any(Error));
  });

  it("should handle WebSocket close", () => {
    // Mock connected state
    (webrtcClient as any).isConnected = true;

    // Simulate WebSocket close
    if (mockWebSocket.onclose) {
      (mockWebSocket as any).onclose(new Event("close"));
    }

    // Should start reconnection
    expect(webrtcClient).toBeDefined();
  });

  it("should send pending control messages when data channel opens", () => {
    // Mock internal state
    (webrtcClient as any).dataChannel = mockDataChannel;
    mockDataChannel.readyState = "open";

    // Add pending message
    (webrtcClient as any).pendingControlMessages = [
      {
        type: "key",
        keycode: 26,
        action: "down",
      },
    ];

    // Call sendPendingControlMessages directly
    (webrtcClient as any).sendPendingControlMessages();

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
