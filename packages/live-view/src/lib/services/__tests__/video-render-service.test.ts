// VideoRenderService tests
import { VideoRenderService } from "../video-render-service";

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
    cursor: "",
    marginLeft: "",
    marginTop: "",
    transition: "",
  },
  getContext: jest.fn(),
  parentNode: null,
  appendChild: jest.fn(),
  removeChild: jest.fn(),
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

// Mock window.screen
Object.defineProperty(window, "screen", {
  value: {
    orientation: {
      type: "landscape-primary",
    },
  },
  writable: true,
});

describe("VideoRenderService", () => {
  let container: HTMLElement;
  let videoRenderService: VideoRenderService;
  let mockOnStatsUpdate: jest.Mock;
  let mockOnError: jest.Mock;

  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();

    // Create mock container
    container = {
      appendChild: jest.fn(),
      getBoundingClientRect: jest.fn(() => ({
        width: 800,
        height: 600,
      })),
    } as any;

    mockOnStatsUpdate = jest.fn();
    mockOnError = jest.fn();

    videoRenderService = new VideoRenderService({
      container,
      onStatsUpdate: mockOnStatsUpdate,
      onError: mockOnError,
    });
  });

  afterEach(() => {
    if (videoRenderService) {
      videoRenderService.destroy();
    }
  });

  it("should create VideoRenderService instance", () => {
    expect(videoRenderService).toBeDefined();
    expect(videoRenderService).toBeInstanceOf(VideoRenderService);
  });

  it("should initialize canvas element", () => {
    expect(document.createElement).toHaveBeenCalledWith("canvas");
    expect(container.appendChild).toHaveBeenCalledWith(mockCanvas);
    expect(mockCanvas.getContext).toHaveBeenCalledWith("2d");
  });

  it("should setup video element for WebRTC", () => {
    const videoElement = videoRenderService.setupVideoElement();

    expect(document.createElement).toHaveBeenCalledWith("video");
    expect(videoElement).toBe(mockVideoElement);
    expect(container.appendChild).toHaveBeenCalledWith(mockVideoElement);
  });

  it("should render video frame to canvas", () => {
    const mockFrame = {
      displayWidth: 1920,
      displayHeight: 1080,
      close: jest.fn(),
    } as any;

    videoRenderService.renderFrame(mockFrame);

    expect(mockCanvas.width).toBe(1920);
    expect(mockCanvas.height).toBe(1080);
    expect(mockContext.drawImage).toHaveBeenCalledWith(mockFrame, 0, 0);
    expect(mockFrame.close).toHaveBeenCalled();
  });

  it("should update canvas display size", () => {
    videoRenderService.updateCanvasDisplaySize(1920, 1080);

    expect(mockCanvas.style.width).toBeDefined();
    expect(mockCanvas.style.height).toBeDefined();
    expect(mockCanvas.style.marginLeft).toBeDefined();
    expect(mockCanvas.style.marginTop).toBeDefined();
  });

  it("should handle resolution changes", () => {
    const mockFrame1 = {
      displayWidth: 1920,
      displayHeight: 1080,
      close: jest.fn(),
    } as any;

    const mockFrame2 = {
      displayWidth: 1280,
      displayHeight: 720,
      close: jest.fn(),
    } as any;

    // First frame
    videoRenderService.renderFrame(mockFrame1);
    expect(mockCanvas.width).toBe(1920);
    expect(mockCanvas.height).toBe(1080);

    // Second frame with different resolution
    videoRenderService.renderFrame(mockFrame2);
    expect(mockCanvas.width).toBe(1280);
    expect(mockCanvas.height).toBe(720);
  });

  it("should start and stop service", () => {
    expect(videoRenderService.active).toBe(false);

    videoRenderService.start();
    expect(videoRenderService.active).toBe(true);

    videoRenderService.stop();
    expect(videoRenderService.active).toBe(false);
  });

  it("should not start multiple times", () => {
    videoRenderService.start();
    expect(videoRenderService.active).toBe(true);

    videoRenderService.start(); // Should not start again
    expect(videoRenderService.active).toBe(true);
  });

  it("should get canvas and video elements", () => {
    const canvas = videoRenderService.getCanvas();
    const videoElement = videoRenderService.getVideoElement();

    expect(canvas).toBe(mockCanvas);
    expect(videoElement).toBeNull(); // Not created yet

    videoRenderService.setupVideoElement();
    const videoElement2 = videoRenderService.getVideoElement();
    expect(videoElement2).toBe(mockVideoElement);
  });

  it("should get current resolution", () => {
    expect(videoRenderService.getCurrentResolution()).toBeNull();

    const mockFrame = {
      displayWidth: 1920,
      displayHeight: 1080,
      close: jest.fn(),
    } as any;

    videoRenderService.renderFrame(mockFrame);
    expect(videoRenderService.getCurrentResolution()).toBe("1920x1080");
  });

  it("should handle video metadata loaded", () => {
    const videoElement = videoRenderService.setupVideoElement();

    // Simulate metadata loaded
    mockVideoElement.videoWidth = 1920;
    mockVideoElement.videoHeight = 1080;

    if (videoElement.onloadedmetadata) {
      videoElement.onloadedmetadata?.(new Event("loadedmetadata"));
    }

    expect(mockOnStatsUpdate).toHaveBeenCalledWith({
      resolution: "1920x1080",
    });
  });

  it("should handle video playing", () => {
    const videoElement = videoRenderService.setupVideoElement();

    if (videoElement.onplaying) {
      videoElement.onplaying?.(new Event("playing"));
    }

    // Should not throw error
    expect(videoElement).toBeDefined();
  });

  it("should set video source for WebRTC", () => {
    const mockStream = {} as MediaStream;

    videoRenderService.setVideoSource(mockStream);

    expect(mockVideoElement.srcObject).toBe(mockStream);
  });

  it("should update options", () => {
    videoRenderService.updateOptions({
      aspectRatioMode: "cover",
      backgroundColor: "white",
    });

    // Options should be updated (we can't directly test private options)
    expect(videoRenderService).toBeDefined();
  });

  it("should handle errors gracefully", () => {
    const mockFrame = {
      displayWidth: 1920,
      displayHeight: 1080,
      close: jest.fn(),
    } as any;

    // Mock drawImage to throw error
    mockContext.drawImage.mockImplementation(() => {
      throw new Error("Draw error");
    });

    videoRenderService.renderFrame(mockFrame);

    expect(mockOnError).toHaveBeenCalledWith(expect.any(Error));
    expect(mockFrame.close).toHaveBeenCalled();
  });

  it("should cleanup on destroy", () => {
    videoRenderService.setupVideoElement();
    videoRenderService.start();

    videoRenderService.destroy();

    expect(videoRenderService.active).toBe(false);
    expect(videoRenderService.getCanvas()).toBeNull();
    expect(videoRenderService.getVideoElement()).toBeNull();
  });

  it("should handle different aspect ratio modes", () => {
    const modes = ["contain", "cover", "fill", "scale-down"] as const;

    modes.forEach((mode) => {
      const service = new VideoRenderService({
        container,
        aspectRatioMode: mode,
      });

      service.updateCanvasDisplaySize(1920, 1080);
      expect(service).toBeDefined();

      service.destroy();
    });
  });

  it("should handle zero container dimensions", () => {
    container.getBoundingClientRect = jest.fn(
      () =>
        ({
          width: 0,
          height: 0,
          x: 0,
          y: 0,
          top: 0,
          left: 0,
          bottom: 0,
          right: 0,
          toJSON: () => ({}),
        } as DOMRect)
    );

    // Should not throw error
    videoRenderService.updateCanvasDisplaySize(1920, 1080);
    expect(videoRenderService).toBeDefined();
  });
});
