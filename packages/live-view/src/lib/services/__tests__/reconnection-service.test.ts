// Simple ReconnectionService tests
import { ReconnectionService } from "../reconnection-service";

describe("ReconnectionService - Simple Tests", () => {
  let reconnectionService: ReconnectionService;
  let mockReconnectFn: jest.Mock;

  beforeEach(() => {
    jest.useFakeTimers();
    mockReconnectFn = jest.fn();
    reconnectionService = new ReconnectionService({
      maxAttempts: 3,
      baseDelay: 100,
      maxDelay: 1000,
    });
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.restoreAllMocks();
    reconnectionService.stop();
  });

  it("should create ReconnectionService instance", () => {
    expect(reconnectionService).toBeDefined();
    expect(reconnectionService).toBeInstanceOf(ReconnectionService);
  });

  it("should start reconnection process", async () => {
    mockReconnectFn.mockResolvedValue(undefined);

    const startPromise = reconnectionService.startReconnection(mockReconnectFn);

    // Fast-forward timers to complete the first attempt
    jest.runAllTimers();
    await startPromise;

    expect(mockReconnectFn).toHaveBeenCalledTimes(1);
    expect(reconnectionService.isActive).toBe(false);
  });

  it("should retry on failure", async () => {
    mockReconnectFn
      .mockRejectedValueOnce(new Error("First attempt fails"))
      .mockResolvedValue(undefined);

    const startPromise = reconnectionService.startReconnection(mockReconnectFn);

    // First attempt fails
    jest.advanceTimersByTime(100);
    await Promise.resolve();

    // Second attempt succeeds
    jest.advanceTimersByTime(200);
    await startPromise;

    expect(mockReconnectFn).toHaveBeenCalledTimes(2);
    expect(reconnectionService.isActive).toBe(false);
  });

  it("should stop after max attempts", async () => {
    mockReconnectFn.mockRejectedValue(new Error("Always fails"));

    const onMaxAttemptsReached = jest.fn();
    reconnectionService.updateOptions({ onMaxAttemptsReached });

    const startPromise = reconnectionService.startReconnection(mockReconnectFn);

    // Run all attempts
    for (let i = 0; i < 3; i++) {
      jest.advanceTimersByTime(100 * Math.pow(2, i));
      await Promise.resolve();
    }

    await startPromise;

    expect(mockReconnectFn).toHaveBeenCalledTimes(3);
    expect(onMaxAttemptsReached).toHaveBeenCalled();
    expect(reconnectionService.isActive).toBe(false);
  });

  it("should not start multiple reconnection processes", async () => {
    mockReconnectFn.mockResolvedValue(undefined);

    // Start first reconnection
    const firstPromise = reconnectionService.startReconnection(mockReconnectFn);

    // Try to start second reconnection while first is active
    const secondPromise =
      reconnectionService.startReconnection(mockReconnectFn);

    jest.runAllTimers();
    await Promise.all([firstPromise, secondPromise]);

    // Should only call reconnectFn once
    expect(mockReconnectFn).toHaveBeenCalledTimes(1);
  });

  it("should reset state on successful reconnection", async () => {
    mockReconnectFn.mockResolvedValue(undefined);

    const startPromise = reconnectionService.startReconnection(mockReconnectFn);
    jest.runAllTimers();
    await startPromise;

    expect(reconnectionService.currentAttempt).toBe(0);
    expect(reconnectionService.isActive).toBe(false);
  });

  it("should stop reconnection process", () => {
    mockReconnectFn.mockImplementation(() => new Promise(() => {})); // Never resolves

    reconnectionService.startReconnection(mockReconnectFn);

    expect(reconnectionService.isActive).toBe(true);

    reconnectionService.stop();

    expect(reconnectionService.isActive).toBe(false);
  });

  it("should create predefined configurations", () => {
    const webRTCService = ReconnectionService.createForWebRTC();
    const h264Service = ReconnectionService.createForH264();
    const webSocketService = ReconnectionService.createForWebSocket();

    expect(webRTCService.maxAttempts).toBe(30);
    expect(webRTCService.maxDelay).toBe(10000);

    expect(h264Service.maxAttempts).toBe(5);
    expect(h264Service.maxDelay).toBe(10000);

    expect(webSocketService.maxAttempts).toBe(5);
    expect(webSocketService.maxDelay).toBe(10000);
  });
});
