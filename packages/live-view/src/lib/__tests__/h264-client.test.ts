/// <reference types="jest" />
import { H264ClientRefactored } from "../h264-client";

// Mock WebCodecs
const mockVideoDecoder = jest.fn();
const mockEncodedVideoChunk = jest.fn();

// Mock VideoDecoder
class MockVideoDecoder {
  private config: any;
  constructor(config: any) {
    this.config = config;
    mockVideoDecoder(config);
  }
  decode(_chunk: any) {
    // Simulate frame output
    if (this.config.output) {
      setTimeout(() => {
        this.config.output({
          displayWidth: 1920,
          displayHeight: 1080,
          close: jest.fn(),
        });
      }, 10);
    }
  }
  close() {}
}

// Mock EncodedVideoChunk
class MockEncodedVideoChunk {
  constructor(config: any) {
    mockEncodedVideoChunk(config);
  }
}

// Mock global objects
Object.defineProperty(global as any, "VideoDecoder", {
  value: MockVideoDecoder,
  writable: true,
});

Object.defineProperty(global as any, "EncodedVideoChunk", {
  value: MockEncodedVideoChunk,
  writable: true,
});

// Mock fetch
(global as any).fetch = jest.fn();

// Mock ReadableStream
const mockReader = {
  read: jest.fn(),
  cancel: jest.fn(),
};

const mockResponse = {
  ok: true,
  body: {
    getReader: jest.fn().mockReturnValue(mockReader),
  },
};

// Mock WebSocket
class MockWebSocket {
  public readyState = WebSocket.CONNECTING;
  public onopen: ((event: Event) => void) | null = null;
  public onclose: ((event: CloseEvent) => void) | null = null;
  public onmessage: ((event: MessageEvent) => void) | null = null;
  public onerror: ((event: Event) => void) | null = null;

  constructor(public url: string) {
    // Simulate connection after a short delay
    setTimeout(() => {
      this.readyState = 1 as any; // WebSocket.OPEN
      if (this.onopen) {
        this.onopen(new Event("open"));
      }
    }, 10);
  }

  send(_data: string) {
    // Mock send
  }

  close() {
    this.readyState = 3 as any; // WebSocket.CLOSED
    if (this.onclose) {
      this.onclose(new CloseEvent("close"));
    }
  }
}

Object.defineProperty(global as any, "WebSocket", {
  value: MockWebSocket,
  writable: true,
});

// Mock MediaSource
class MockMediaSource {
  public readyState = "closed";
  public addSourceBuffer = jest.fn().mockReturnValue({
    addEventListener: jest.fn(),
    appendBuffer: jest.fn(),
    updating: false,
  });
  public addEventListener = jest.fn();
}

Object.defineProperty(global as any, "MediaSource", {
  value: MockMediaSource,
  writable: true,
});

Object.defineProperty(global as any, "URL", {
  value: {
    createObjectURL: jest.fn().mockReturnValue("blob:mock-url"),
  },
  writable: true,
});

// Mock HTMLCanvasElement and CanvasRenderingContext2D
const mockCanvas = {
  width: 0,
  height: 0,
  style: {
    display: "",
    width: "",
    height: "",
    objectFit: "",
    background: "",
    margin: "",
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

// Mock DOM methods
Object.defineProperty(document, "createElement", {
  value: jest.fn((tagName: string) => {
    if (tagName === "canvas") {
      return mockCanvas;
    }
    if (tagName === "video") {
      return mockVideoElement;
    }
    if (tagName === "audio") {
      return {
        controls: false,
        style: { display: "" },
        addEventListener: jest.fn(),
        pause: jest.fn(),
        play: jest.fn().mockResolvedValue(undefined),
        remove: jest.fn(),
        error: null,
        networkState: 0,
        readyState: 0,
        currentTime: 0,
        duration: 0,
        paused: false,
        muted: false,
        buffered: {
          length: 0,
          end: jest.fn().mockReturnValue(0),
        },
      };
    }
    return {};
  }),
});

// Mock getContext to return mockContext
mockCanvas.getContext.mockReturnValue(mockContext);

// Mock ResizeObserver
(global as any).ResizeObserver = jest.fn().mockImplementation(() => ({
  observe: jest.fn(),
  unobserve: jest.fn(),
  disconnect: jest.fn(),
}));

describe("H264ClientRefactored", () => {
  let container: HTMLElement;
  let h264Client: H264ClientRefactored;
  let mockOnConnectionStateChange: jest.Mock;
  let mockOnError: jest.Mock;
  let mockOnStatsUpdate: jest.Mock;

  beforeEach(() => {
    jest.useFakeTimers();
    jest.clearAllMocks();

    // Create mock container
    container = document.createElement("div");
    Object.defineProperty(container, "clientWidth", {
      value: 1000,
      writable: true,
    });
    Object.defineProperty(container, "clientHeight", {
      value: 800,
      writable: true,
    });
    // Add missing methods to container
    (container as any).appendChild = jest.fn();
    (container as any).removeChild = jest.fn();

    mockOnConnectionStateChange = jest.fn();
    mockOnError = jest.fn();
    mockOnStatsUpdate = jest.fn();

    h264Client = new H264ClientRefactored(container, {
      onConnectionStateChange: mockOnConnectionStateChange,
      onError: mockOnError,
      onStatsUpdate: mockOnStatsUpdate,
      enableAudio: true,
      audioCodec: "opus",
    });

    // Mock fetch
    ((global as any).fetch as jest.Mock).mockResolvedValue(mockResponse);

    // Mock reader
    mockReader.read.mockResolvedValue({
      done: true,
      value: new Uint8Array([0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1e]), // Mock NAL unit
    });
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.restoreAllMocks();
    if (h264Client) {
      h264Client.disconnect();
    }
  });

  it("should create H264ClientRefactored instance", () => {
    expect(h264Client).toBeDefined();
    expect(h264Client).toBeInstanceOf(H264ClientRefactored);
  });

  it("should connect successfully", async () => {
    // Mock establishConnection to succeed immediately
    jest
      .spyOn(h264Client as any, "establishConnection")
      .mockResolvedValue(undefined);

    await h264Client.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    expect(mockOnConnectionStateChange).toHaveBeenLastCalledWith(
      "connected",
      "Connected successfully"
    );
  });

  it("should disconnect successfully", async () => {
    // Mock establishConnection to succeed
    jest
      .spyOn(h264Client as any, "establishConnection")
      .mockResolvedValue(undefined);

    await h264Client.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );
    await h264Client.disconnect();

    expect(mockOnConnectionStateChange).toHaveBeenLastCalledWith(
      "disconnected",
      "Disconnected"
    );
  });

  it("should handle connection errors", async () => {
    const error = new Error("Connection failed");
    jest
      .spyOn(h264Client as any, "establishConnection")
      .mockRejectedValue(error);

    try {
      await h264Client.connect(
        "device123",
        "http://api.example.com",
        "ws://ws.example.com"
      );
    } catch (e) {
      // Expected to throw
    }

    expect(mockOnError).toHaveBeenCalledWith(error);
  });

  it("should send control messages", async () => {
    // Mock establishConnection to succeed
    jest
      .spyOn(h264Client as any, "establishConnection")
      .mockResolvedValue(undefined);

    await h264Client.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    // Wait for WebSocket to connect
    jest.advanceTimersByTime(20);

    // Test key event
    h264Client.sendKeyEvent(29, "down", 0); // KeyA down

    // Test touch event
    h264Client.sendTouchEvent(0.5, 0.5, "down", 1.0);

    // Test control action
    h264Client.sendControlAction("scroll", { x: 0.5, y: 0.5, hScroll: 10 });

    // Test clipboard
    h264Client.sendClipboardSet("test text", true);

    expect(h264Client).toBeDefined();
  });

  it("should handle mouse events", () => {
    const mockEvent = {
      clientX: 100,
      clientY: 200,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 1000,
          height: 800,
        }),
      },
    } as any;

    h264Client.handleMouseEvent(mockEvent, "down");
    expect(h264Client).toBeDefined();
  });

  it("should handle touch events", () => {
    const mockEvent = {
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
          width: 1000,
          height: 800,
        }),
      },
    } as any;

    h264Client.handleTouchEvent(mockEvent, "down");
    expect(h264Client).toBeDefined();
  });

  it("should check control connection status", () => {
    expect(h264Client.isControlConnected()).toBe(false);

    // Mock connected state - need both isConnected and controlWs
    (h264Client as any).isConnected = true;
    (h264Client as any).controlWs = {
      readyState: WebSocket.OPEN, // Use actual WebSocket.OPEN constant
    };

    // Verify the internal method works
    expect((h264Client as any).isControlConnectedInternal()).toBe(true);
    expect(h264Client.isControlConnected()).toBe(true);
  });

  it("should setup video decoder", () => {
    expect(mockVideoDecoder).toHaveBeenCalledWith(
      expect.objectContaining({
        output: expect.any(Function),
        error: expect.any(Function),
      })
    );
  });

  it("should process NAL units", () => {
    const nalUnit = new Uint8Array([0x67, 0x42, 0x00, 0x1e]); // SPS
    (h264Client as any).processNALUnit(nalUnit);

    expect((h264Client as any).spsData).toEqual(nalUnit);
  });

  it("should handle frame decoding", () => {
    const mockFrame = {
      displayWidth: 1920,
      displayHeight: 1080,
      close: jest.fn(),
    };

    // Mock the video render service
    jest.spyOn(h264Client as any, "videoRender", "get").mockReturnValue({
      renderFrame: jest.fn(),
    });

    // Mock the stats service
    jest.spyOn(h264Client as any, "stats", "get").mockReturnValue({
      recordFrameDecoded: jest.fn(),
      updateResolution: jest.fn(),
    });

    // Mock the renderFrame method to prevent errors
    jest.spyOn(h264Client as any, "renderFrame").mockImplementation(() => {});

    (h264Client as any).onFrameDecoded(mockFrame);

    expect((h264Client as any).decodedFrames).toHaveLength(1);
    expect((h264Client as any).decodedFrames[0].frame).toBe(mockFrame);
  });

  it("should request keyframes", () => {
    // Mock connected control WebSocket
    (h264Client as any).controlWs = {
      readyState: WebSocket.OPEN,
      send: jest.fn(),
    };

    h264Client.requestKeyframe();

    expect((h264Client as any).controlWs.send).toHaveBeenCalledWith(
      expect.stringContaining('"type":"reset_video"')
    );
  });

  it("should handle WebSocket close", () => {
    // Mock connected state
    (h264Client as any).isConnected = true;
    (h264Client as any).controlWs = {
      readyState: WebSocket.OPEN,
      close: jest.fn(),
    };

    // Simulate WebSocket close
    if ((h264Client as any).controlWs.onclose) {
      (h264Client as any).controlWs.onclose();
    }

    expect(h264Client).toBeDefined();
  });

  it("should handle audio connection", async () => {
    // Mock MediaSource
    const mockMediaSource = {
      readyState: "closed",
      addSourceBuffer: jest.fn().mockReturnValue({
        addEventListener: jest.fn(),
        appendBuffer: jest.fn(),
        updating: false,
      }),
      addEventListener: jest.fn((event, callback) => {
        if (event === "sourceopen") {
          setTimeout(() => callback(), 10);
        }
      }),
    };

    Object.defineProperty(global as any, "MediaSource", {
      value: jest.fn().mockImplementation(() => mockMediaSource),
      writable: true,
    });

    // Mock fetch for audio
    ((global as any).fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: jest.fn().mockReturnValue({
          read: jest
            .fn()
            .mockResolvedValue({ done: true, value: new Uint8Array() }),
        }),
      },
    });

    await (h264Client as any).connectAudio("http://api.example.com/audio");

    expect(h264Client).toBeDefined();
  });

  it("should handle different NAL unit types", () => {
    const spsUnit = new Uint8Array([0x67, 0x42, 0x00, 0x1e]); // SPS
    const ppsUnit = new Uint8Array([0x68, 0xce, 0x38, 0x80]); // PPS
    const idrUnit = new Uint8Array([0x65, 0x88, 0x84, 0x00]); // IDR

    (h264Client as any).processNALUnit(spsUnit);
    expect((h264Client as any).spsData).toEqual(spsUnit);

    (h264Client as any).processNALUnit(ppsUnit);
    expect((h264Client as any).ppsData).toEqual(ppsUnit);

    (h264Client as any).processNALUnit(idrUnit);
    expect((h264Client as any).waitingForKeyframe).toBe(false);
  });

  it("should handle buffer processing", () => {
    const buffer = new Uint8Array([
      0x00,
      0x00,
      0x00,
      0x01,
      0x67,
      0x42,
      0x00,
      0x1e, // SPS
      0x00,
      0x00,
      0x00,
      0x01,
      0x68,
      0xce,
      0x38,
      0x80, // PPS
    ]);

    (h264Client as any).buffer = buffer;
    (h264Client as any).processNALUnits();

    expect((h264Client as any).spsData).toBeDefined();
    expect((h264Client as any).ppsData).toBeDefined();
  });

  it("should handle resize observer", () => {
    (h264Client as any).lastCanvasDimensions = { width: 1920, height: 1080 };
    (h264Client as any).startResizeObserver();

    // Simulate resize
    const resizeObserverCallback = ((global as any).ResizeObserver as jest.Mock)
      .mock.calls[0][0];
    resizeObserverCallback();

    expect(h264Client).toBeDefined();
  });

  it("should get audio processor", () => {
    const audioProcessor = h264Client.getAudioProcessor();
    expect(audioProcessor).toBeNull(); // Not connected yet

    // Mock audio processor
    (h264Client as any).audioProcessor = { isStreaming: true };
    const audioProcessor2 = h264Client.getAudioProcessor();
    expect(audioProcessor2).toBeDefined();
  });

  it("should handle control reconnection", () => {
    (h264Client as any).controlRetryCount = 0;
    (h264Client as any).maxControlRetries = 3;

    (h264Client as any).handleControlReconnection(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    expect((h264Client as any).controlRetryCount).toBe(1);
    // controlReconnectTimer is no longer used, reconnection service handles it
  });

  it("should handle WebCodecs not supported", () => {
    // Mock VideoDecoder not available
    const originalVideoDecoder = (global as any).VideoDecoder;
    Object.defineProperty(global as any, "VideoDecoder", {
      value: undefined,
      writable: true,
    });

    // Clear previous calls
    mockOnError.mockClear();

    // Create a new client instance to trigger the error
    const client = new H264ClientRefactored(container, {
      onError: mockOnError,
    });

    // Start error handling service to ensure it's active
    (client as any).errorHandlingService.start();

    // Manually trigger the error handling by calling initializeWebCodecs
    (client as any).initializeWebCodecs();

    // The error should be called during initializeWebCodecs
    expect(mockOnError).toHaveBeenCalledWith(
      expect.objectContaining({
        message: "WebCodecs not supported",
      })
    );

    // Restore original VideoDecoder
    Object.defineProperty(global as any, "VideoDecoder", {
      value: originalVideoDecoder,
      writable: true,
    });
  });

  it("should handle HTTP stream errors", async () => {
    // Reset fetch mock and set it to reject
    ((global as any).fetch as jest.Mock).mockReset();
    ((global as any).fetch as jest.Mock).mockRejectedValueOnce(
      new Error("Network error")
    );

    // Test that startHTTP throws the error
    await expect(
      (h264Client as any).startHTTP("http://api.example.com/stream")
    ).rejects.toThrow("Network error");

    // Verify fetch was called
    expect((global as any).fetch).toHaveBeenCalledWith(
      "http://api.example.com/stream",
      {
        signal: expect.any(AbortSignal),
      }
    );
  });

  it("should handle stream processing errors", async () => {
    const mockReader = {
      read: jest.fn().mockRejectedValueOnce(new Error("Read error")),
      cancel: jest.fn(),
    };

    // Clear previous calls
    mockOnError.mockClear();

    // Start the error handling service to ensure it's active
    (h264Client as any).errorHandlingService.start();

    await (h264Client as any).processVideoStream(mockReader);

    expect(mockOnError).toHaveBeenCalledWith(
      expect.objectContaining({
        message: "Read error",
      })
    );
  });
});
