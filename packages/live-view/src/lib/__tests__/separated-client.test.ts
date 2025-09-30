/* eslint-disable @typescript-eslint/no-explicit-any */
/// <reference types="jest" />
import { H264Client } from "../separated-client";

// Mock WebCodecs
const mockEncodedVideoChunk = jest.fn();

// Mock VideoDecoder
class MockVideoDecoder {
  // private config: any;
  // private output: any;
  // private error: any;

  constructor(_args: { output: unknown; error: unknown }) {}

  configure(_config: unknown) {}

  decode(_chunk: unknown) {}

  close() {}

  get state() {
    return "configured";
  }
}

// Mock global objects
(global as Record<string, unknown>).VideoDecoder = MockVideoDecoder;
(global as Record<string, unknown>).EncodedVideoChunk = mockEncodedVideoChunk;

// Mock WebCodecs availability
Object.defineProperty(global, "VideoDecoder", {
  value: MockVideoDecoder,
  writable: true,
  configurable: true,
});

// Mock DOM methods
const mockCanvas = {
  getContext: jest.fn().mockReturnValue({
    drawImage: jest.fn(),
  }),
  getBoundingClientRect: jest.fn().mockReturnValue({
    left: 0,
    top: 0,
    width: 100,
    height: 100,
  }),
  width: 0,
  height: 0,
  style: {},
  appendChild: jest.fn(),
  removeChild: jest.fn(),
  parentNode: null,
};

const mockContainer = {
  appendChild: jest.fn(),
  removeChild: jest.fn(),
};

// Mock document.createElement
(global as Record<string, unknown>).document = {
  createElement: jest.fn().mockReturnValue(mockCanvas),
};

// Mock MediaSource
(global as Record<string, unknown>).MediaSource = {
  isTypeSupported: jest.fn().mockReturnValue(true),
};

// Mock ResizeObserver
(global as Record<string, unknown>).ResizeObserver = jest
  .fn()
  .mockImplementation(() => ({
    observe: jest.fn(),
    unobserve: jest.fn(),
    disconnect: jest.fn(),
  }));

describe("H264Client", () => {
  let container: HTMLElement;
  let h264Client: H264Client;
  let mockOnConnectionStateChange: jest.Mock;
  let mockOnError: jest.Mock;
  let mockOnStatsUpdate: jest.Mock;

  beforeEach(() => {
    // Reset all mocks
    jest.clearAllMocks();

    // Create container
    container = mockContainer as unknown as HTMLElement;

    mockOnConnectionStateChange = jest.fn();
    mockOnError = jest.fn();
    mockOnStatsUpdate = jest.fn();

    // Mock WebCodecs before creating the client
    (global as Record<string, unknown>).VideoDecoder = MockVideoDecoder;
    (global as Record<string, unknown>).EncodedVideoChunk =
      mockEncodedVideoChunk;

    // Mock document.createElement to return our mock canvas
    (global as any).document.createElement = jest
      .fn()
      .mockReturnValue(mockCanvas);

    h264Client = new H264Client(container, {
      onConnectionStateChange: mockOnConnectionStateChange,
      onError: mockOnError,
      onStatsUpdate: mockOnStatsUpdate,
      enableAudio: true,
      audioCodec: "opus",
    });
  });

  afterEach(() => {
    if (h264Client) {
      // Set a mock WebSocket to prevent errors during cleanup
      (
        h264Client as unknown as { controlWebSocket: { close: jest.Mock } }
      ).controlWebSocket = {
        close: jest.fn(),
      };

      // Set mock audio processor to prevent errors during cleanup
      (
        h264Client as unknown as {
          audioProcessor: {
            audioElement: { pause: jest.Mock; remove: jest.Mock };
          };
        }
      ).audioProcessor = {
        disconnect: jest.fn(),
        audioElement: { pause: jest.fn(), remove: jest.fn() },
      } as any;

      h264Client.disconnect();
    }
  });

  it("should create H264Client instance", () => {
    expect(h264Client).toBeDefined();
    expect(h264Client).toBeInstanceOf(H264Client);
  });

  it("should connect successfully", async () => {
    // Mock fetch to return a successful response
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: jest.fn().mockReturnValue({
          read: jest.fn().mockResolvedValue({ done: true, value: undefined }),
        }),
      },
    });

    // Mock WebSocket
    const mockWebSocket = {
      readyState: WebSocket.OPEN as any,
      send: jest.fn(),
      close: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
    };
    global.WebSocket = jest.fn(
      () => mockWebSocket
    ) as unknown as typeof WebSocket;

    // Set the mock WebSocket on the client
    (h264Client as unknown as { controlWebSocket: unknown }).controlWebSocket =
      mockWebSocket;

    await h264Client.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "connected",
      "H.264 stream connected"
    );
  });

  it("should disconnect successfully", async () => {
    // Mock fetch and WebSocket
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: jest.fn().mockReturnValue({
          read: jest.fn().mockResolvedValue({ done: true, value: undefined }),
        }),
      },
    });

    const mockWebSocket = {
      readyState: WebSocket.OPEN as any,
      send: jest.fn(),
      close: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
    };
    global.WebSocket = jest.fn(
      () => mockWebSocket
    ) as unknown as typeof WebSocket;

    await h264Client.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    // Mock audio processor with proper audioElement after connect
    if (
      (h264Client as unknown as { audioProcessor: { audioElement: unknown } })
        .audioProcessor
    ) {
      (
        h264Client as unknown as {
          audioProcessor: {
            audioElement: { pause: jest.Mock; remove: jest.Mock };
          };
        }
      ).audioProcessor.audioElement = {
        pause: jest.fn(),
        remove: jest.fn(),
      };
    }

    await h264Client.disconnect();

    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "disconnected",
      "H.264 stream disconnected"
    );
  });

  it("should handle connection errors", async () => {
    const error = new Error("Connection failed");
    global.fetch = jest.fn().mockRejectedValue(error);

    try {
      await h264Client.connect(
        "device123",
        "http://api.example.com",
        "ws://ws.example.com"
      );
    } catch (_e) {
      // Expected to throw
    }

    expect(mockOnError).toHaveBeenCalledWith(error);
  });

  it("should send control messages", async () => {
    // Mock fetch and WebSocket
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: jest.fn().mockReturnValue({
          read: jest.fn().mockResolvedValue({ done: true, value: undefined }),
        }),
      },
    });

    const mockWebSocket = {
      readyState: WebSocket.OPEN as any,
      send: jest.fn(),
      close: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
    };
    global.WebSocket = jest.fn(
      () => mockWebSocket
    ) as unknown as typeof WebSocket;

    await h264Client.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    h264Client.sendKeyEvent(4, "down");
    h264Client.sendTouchEvent(0.5, 0.5, "down");
    h264Client.sendControlAction("home");

    expect(mockWebSocket.send).toHaveBeenCalledTimes(3);
  });

  it("should handle mouse events", () => {
    const mockEvent = {
      clientX: 50,
      clientY: 50,
    } as MouseEvent;

    // Mock WebSocket
    const mockWebSocket = {
      readyState: WebSocket.OPEN as any,
      send: jest.fn(),
    };
    (h264Client as unknown as { controlWebSocket: unknown }).controlWebSocket =
      mockWebSocket;
    (h264Client as unknown as { controlConnected: boolean }).controlConnected =
      true;

    h264Client.handleMouseEvent(mockEvent, "down");

    expect(mockWebSocket.send).toHaveBeenCalled();
  });

  it("should handle touch events", () => {
    const mockEvent = {
      touches: [{ clientX: 50, clientY: 50 }],
    } as unknown as TouchEvent;

    // Mock WebSocket
    const mockWebSocket = {
      readyState: WebSocket.OPEN as any,
      send: jest.fn(),
    };
    (h264Client as unknown as { controlWebSocket: unknown }).controlWebSocket =
      mockWebSocket;
    (h264Client as unknown as { controlConnected: boolean }).controlConnected =
      true;

    h264Client.handleTouchEvent(mockEvent, "down");

    expect(mockWebSocket.send).toHaveBeenCalled();
  });

  it("should check control connection status", () => {
    expect(h264Client.isControlConnected()).toBe(false);

    // Mock connected WebSocket
    (
      h264Client as unknown as { controlWebSocket: { close: jest.Mock } }
    ).controlWebSocket = {
      close: jest.fn(),
      readyState: WebSocket.OPEN as any,
    } as any;
    (h264Client as unknown as { controlConnected: boolean }).controlConnected =
      true;

    expect(h264Client.isControlConnected()).toBe(true);
  });
});
