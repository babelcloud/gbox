// BaseClient tests
import { BaseClient } from "../base-client";
import { ConnectionParams } from "../types";

// Mock concrete implementation for testing
class MockBaseClient extends BaseClient {
  private lastApiUrl: string = "";
  private lastWsUrl: string | undefined;

  protected async establishConnection(params: ConnectionParams): Promise<void> {
    this.lastApiUrl = params.apiUrl;
    this.lastWsUrl = params.wsUrl;
    // Simulate connection establishment
    await new Promise((resolve) => setTimeout(resolve, 10));
  }

  protected async cleanupConnection(): Promise<void> {
    // Simulate cleanup
    await new Promise((resolve) => setTimeout(resolve, 10));
  }

  protected isControlConnectedInternal(): boolean {
    return this.connected;
  }

  protected getLastApiUrl(): string {
    return this.lastApiUrl;
  }

  protected getLastWsUrl(): string | undefined {
    return this.lastWsUrl;
  }
}

// Mock HTML element
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

// Mock DOM methods
Object.defineProperty(document, "createElement", {
  value: jest.fn((tagName: string) => {
    if (tagName === "canvas") {
      return mockCanvas;
    }
    if (tagName === "video") {
      return {
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
    }
    return {};
  }),
});

// Mock getContext to return mockContext
mockCanvas.getContext.mockReturnValue(mockContext);

describe("BaseClient", () => {
  let baseClient: MockBaseClient;
  let mockOnConnectionStateChange: jest.Mock;
  let mockOnError: jest.Mock;
  let mockOnStatsUpdate: jest.Mock;

  beforeEach(() => {
    mockOnConnectionStateChange = jest.fn();
    mockOnError = jest.fn();
    mockOnStatsUpdate = jest.fn();

    baseClient = new MockBaseClient(mockContainer, {
      onConnectionStateChange: mockOnConnectionStateChange,
      onError: mockOnError,
      onStatsUpdate: mockOnStatsUpdate,
    });
  });

  afterEach(() => {
    if (baseClient) {
      baseClient.destroy();
    }
  });

  it("should create BaseClient instance", () => {
    expect(baseClient).toBeDefined();
    expect(baseClient).toBeInstanceOf(BaseClient);
  });

  it("should initialize with correct default state", () => {
    expect(baseClient.connected).toBe(false);
    expect(baseClient.state).toBe("disconnected");
    expect(baseClient.device).toBeNull();
    expect(baseClient.isMouseDragging).toBe(false);
  });

  it("should connect successfully", async () => {
    await baseClient.connect(
      "device123",
      "http://api.example.com",
      "ws://ws.example.com"
    );

    expect(baseClient.connected).toBe(true);
    expect(baseClient.state).toBe("connected");
    expect(baseClient.device).toBe("device123");
    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "connecting",
      "Connecting to device..."
    );
    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "connected",
      "Connected successfully"
    );
  });

  it("should disconnect successfully", async () => {
    await baseClient.connect("device123", "http://api.example.com");
    expect(baseClient.connected).toBe(true);

    await baseClient.disconnect();

    expect(baseClient.connected).toBe(false);
    expect(baseClient.state).toBe("disconnected");
    expect(baseClient.device).toBeNull();
    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "disconnected",
      "Disconnected"
    );
  });

  it("should handle connection errors", async () => {
    // Mock establishConnection to throw error
    jest
      .spyOn(baseClient as any, "establishConnection")
      .mockRejectedValue(new Error("Connection failed"));

    try {
      await baseClient.connect("device123", "http://api.example.com");
    } catch (error) {
      // Expected to throw
    }

    expect(baseClient.connected).toBe(false);
    expect(baseClient.state).toBe("error");
    expect(mockOnError).toHaveBeenCalledWith(expect.any(Error));
    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "error",
      "Connection failed"
    );
  });

  it("should clean up existing connection before connecting", async () => {
    await baseClient.connect("device123", "http://api.example.com");
    expect(baseClient.connected).toBe(true);

    // Connect to different device
    await baseClient.connect("device456", "http://api.example.com");

    expect(baseClient.device).toBe("device456");
    expect(baseClient.connected).toBe(true);
  });

  it("should implement ControlClient interface", () => {
    // Test that all ControlClient methods exist
    expect(typeof baseClient.sendKeyEvent).toBe("function");
    expect(typeof baseClient.sendTouchEvent).toBe("function");
    expect(typeof baseClient.sendControlAction).toBe("function");
    expect(typeof baseClient.sendClipboardSet).toBe("function");
    expect(typeof baseClient.requestKeyframe).toBe("function");
    expect(typeof baseClient.handleMouseEvent).toBe("function");
    expect(typeof baseClient.handleTouchEvent).toBe("function");
    expect(typeof baseClient.isControlConnected).toBe("function");
  });

  it("should handle control actions", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation();

    baseClient.sendControlAction("power");
    baseClient.sendKeyEvent(26, "down");
    baseClient.sendTouchEvent(100, 200, "down");
    baseClient.sendClipboardSet("test text", true);
    baseClient.requestKeyframe();

    // Should not throw errors
    expect(baseClient).toBeDefined();
    expect(consoleSpy).toHaveBeenCalled();

    consoleSpy.mockRestore();
  });

  it("should check control connection status", () => {
    expect(baseClient.isControlConnected()).toBe(false);

    // Mock connected state
    (baseClient as any).isConnected = true;
    (baseClient as any).isControlConnectedInternal = jest
      .fn()
      .mockReturnValue(true);

    expect(baseClient.isControlConnected()).toBe(true);
  });

  it("should handle mouse and touch events", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation();

    const mockMouseEvent = {
      clientX: 100,
      clientY: 200,
    } as MouseEvent;

    const mockTouchEvent = {} as TouchEvent;

    baseClient.handleMouseEvent(mockMouseEvent, "down");
    baseClient.handleTouchEvent(mockTouchEvent, "down");

    // Should not throw errors
    expect(baseClient).toBeDefined();
    expect(consoleSpy).toHaveBeenCalled();

    consoleSpy.mockRestore();
  });

  it("should provide service access", () => {
    expect(baseClient.control).toBeDefined();
    expect(baseClient.stats).toBeDefined();
    expect(baseClient.videoRender).toBeDefined();
    expect(baseClient.errorHandling).toBeDefined();
  });

  it("should handle error with context", () => {
    const error = new Error("Test error");
    
    // Start the error handling service first
    baseClient.errorHandling.start();
    
    // Call handleError directly
    (baseClient as any).handleError(error, "TestComponent", "testOperation", {
      test: "data",
    });

    // The error handling service should call onError
    expect(mockOnError).toHaveBeenCalledWith(error);
  });

  it("should update connection state", () => {
    (baseClient as any).updateConnectionState("connecting", "Test message");

    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "connecting",
      "Test message"
    );
  });

  it("should start and stop services", () => {
    (baseClient as any).startServices();
    (baseClient as any).stopServices();

    // Should not throw errors
    expect(baseClient).toBeDefined();
  });

  it("should handle reconnection", () => {
    (baseClient as any).isReconnecting = false;
    (baseClient as any).currentDevice = "device123";
    (baseClient as any).getLastApiUrl = jest
      .fn()
      .mockReturnValue("http://api.example.com");
    (baseClient as any).getLastWsUrl = jest
      .fn()
      .mockReturnValue("ws://ws.example.com");

    (baseClient as any).startReconnection();

    expect((baseClient as any).isReconnecting).toBe(true);
    expect(mockOnConnectionStateChange).toHaveBeenCalledWith(
      "connecting",
      "Reconnecting..."
    );
  });

  it("should stop reconnection", () => {
    (baseClient as any).isReconnecting = true;

    (baseClient as any).stopReconnection();

    expect((baseClient as any).isReconnecting).toBe(false);
  });

  it("should destroy properly", () => {
    baseClient.destroy();

    expect(baseClient.connected).toBe(false);
    expect(baseClient.state).toBe("disconnected");
  });

  it("should handle multiple connection attempts", async () => {
    // First connection
    await baseClient.connect("device123", "http://api.example.com");
    expect(baseClient.device).toBe("device123");

    // Second connection to different device
    await baseClient.connect("device456", "http://api2.example.com");
    expect(baseClient.device).toBe("device456");

    // Third connection to same device
    await baseClient.connect("device456", "http://api2.example.com");
    expect(baseClient.device).toBe("device456");
  });

  it("should maintain state consistency", async () => {
    expect(baseClient.connected).toBe(false);
    expect(baseClient.state).toBe("disconnected");
    expect(baseClient.device).toBeNull();

    await baseClient.connect("device123", "http://api.example.com");

    expect(baseClient.connected).toBe(true);
    expect(baseClient.state).toBe("connected");
    expect(baseClient.device).toBe("device123");

    await baseClient.disconnect();

    expect(baseClient.connected).toBe(false);
    expect(baseClient.state).toBe("disconnected");
    expect(baseClient.device).toBeNull();
  });
});
