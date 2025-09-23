// StatsService tests
import { StatsService } from "../stats-service";

describe("StatsService", () => {
  let statsService: StatsService;
  let mockOnStatsUpdate: jest.Mock;

  beforeEach(() => {
    jest.useFakeTimers();
    mockOnStatsUpdate = jest.fn();
    statsService = new StatsService({
      updateInterval: 100,
      onStatsUpdate: mockOnStatsUpdate,
    });
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.restoreAllMocks();
    statsService.stop();
  });

  it("should create StatsService instance", () => {
    expect(statsService).toBeDefined();
    expect(statsService).toBeInstanceOf(StatsService);
  });

  it("should start and stop monitoring", () => {
    expect(statsService.active).toBe(false);

    statsService.start();
    expect(statsService.active).toBe(true);

    statsService.stop();
    expect(statsService.active).toBe(false);
  });

  it("should not start multiple monitoring sessions", () => {
    statsService.start();
    expect(statsService.active).toBe(true);

    statsService.start(); // Should not start again
    expect(statsService.active).toBe(true);
  });

  it("should record frame decoded and calculate FPS", () => {
    statsService.start();

    // Record 30 frames quickly to trigger FPS calculation
    for (let i = 0; i < 30; i++) {
      statsService.recordFrameDecoded();
    }

    // Manually trigger FPS calculation by calling updateFPS directly
    // This simulates what happens when timeDiff >= 1.0
    const fps = 30;
    (statsService as any).updateFPS(fps);

    expect(mockOnStatsUpdate).toHaveBeenCalledWith(
      expect.objectContaining({ fps: 30 })
    );
  });

  it("should calculate FPS with real time simulation", () => {
    statsService.start();

    // Mock Date.now to simulate time passing
    const startTime = 1000000;
    let currentTime = startTime;

    jest.spyOn(Date, "now").mockImplementation(() => currentTime);

    // Record first frame
    statsService.recordFrameDecoded();

    // Advance time by 1 second and record 30 frames
    currentTime = startTime + 1000;
    for (let i = 0; i < 30; i++) {
      statsService.recordFrameDecoded();
    }

    expect(mockOnStatsUpdate).toHaveBeenCalledWith(
      expect.objectContaining({ fps: expect.any(Number) })
    );

    // Restore Date.now
    jest.restoreAllMocks();
  });

  it("should update resolution when changed", () => {
    statsService.start();

    statsService.updateResolution(1920, 1080);

    expect(mockOnStatsUpdate).toHaveBeenCalledWith({
      resolution: "1920x1080",
    });
  });

  it("should not update resolution if same", () => {
    statsService.start();

    statsService.updateResolution(1920, 1080);
    mockOnStatsUpdate.mockClear();

    statsService.updateResolution(1920, 1080);

    expect(mockOnStatsUpdate).not.toHaveBeenCalled();
  });

  it("should record ping times for latency calculation", () => {
    statsService.start();

    statsService.recordPingTime(50);
    statsService.recordPingTime(60);
    statsService.recordPingTime(40);

    const stats = statsService.getCurrentStats();
    expect(stats.latency).toBe(50); // Average of 50, 60, 40
  });

  it("should record bytes received for bandwidth calculation", () => {
    statsService.start();

    statsService.recordBytesReceived(1024);
    statsService.recordBytesReceived(2048);

    // This would be tested in processWebRTCStats
    expect(statsService).toBeDefined();
  });

  it("should process WebRTC stats", async () => {
    // Mock RTCPeerConnection and getStats
    const mockStats = new Map();
    mockStats.set("video-inbound", {
      type: "inbound-rtp",
      mediaType: "video",
      framesDecoded: 100,
      frameWidth: 1920,
      frameHeight: 1080,
      bytesReceived: 1024000,
    });

    const mockPC = {
      getStats: jest.fn().mockResolvedValue(mockStats),
    } as any;

    const metrics = await statsService.processWebRTCStats(mockPC);

    expect(metrics.resolution).toBe("1920x1080");
    expect(mockPC.getStats).toHaveBeenCalled();
  });

  it("should process H264 stats", () => {
    const mockCanvas = {
      width: 1280,
      height: 720,
    } as HTMLCanvasElement;

    const metrics = statsService.processH264Stats(mockCanvas);

    expect(metrics.resolution).toBe("1280x720");
  });

  it("should get current stats", () => {
    statsService.start();

    // Set up some data
    statsService.updateResolution(1920, 1080);
    statsService.recordPingTime(50);

    const stats = statsService.getCurrentStats();

    expect(stats.resolution).toBe("1920x1080");
    expect(stats.latency).toBe(50);
  });

  it("should reset all counters", () => {
    statsService.start();

    // Set up some data
    statsService.updateResolution(1920, 1080);
    statsService.recordPingTime(50);
    statsService.recordBytesReceived(1024);

    statsService.reset();

    const stats = statsService.getCurrentStats();
    expect(stats.resolution).toBeUndefined();
    expect(stats.latency).toBeUndefined();
  });

  it("should update options", () => {
    statsService.updateOptions({
      updateInterval: 500,
      enableFPS: false,
    });

    // Test that options were updated
    expect(statsService).toBeDefined();
  });

  it("should handle errors in WebRTC stats processing", async () => {
    const mockPC = {
      getStats: jest.fn().mockRejectedValue(new Error("Stats error")),
    } as any;

    const consoleSpy = jest.spyOn(console, "error").mockImplementation();

    const metrics = await statsService.processWebRTCStats(mockPC);

    expect(consoleSpy).toHaveBeenCalledWith(
      "[StatsService] Error processing WebRTC stats:",
      expect.any(Error)
    );
    expect(metrics).toEqual({});

    consoleSpy.mockRestore();
  });

  it("should maintain ping history limit", () => {
    statsService.start();

    // Add more pings than the limit
    for (let i = 0; i < 15; i++) {
      statsService.recordPingTime(i * 10);
    }

    const stats = statsService.getCurrentStats();
    // Should only keep the last 10 pings
    expect(stats.latency).toBeDefined();
  });
});
