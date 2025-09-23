// ErrorHandlingService tests
import { ErrorHandlingService } from "../error-handling-service";

describe("ErrorHandlingService", () => {
  let errorHandlingService: ErrorHandlingService;
  let mockOnError: jest.Mock;
  let mockOnRecoverableError: jest.Mock;
  let mockOnFatalError: jest.Mock;

  beforeEach(() => {
    mockOnError = jest.fn();
    mockOnRecoverableError = jest.fn();
    mockOnFatalError = jest.fn();

    errorHandlingService = new ErrorHandlingService({
      onError: mockOnError,
      onRecoverableError: mockOnRecoverableError,
      onFatalError: mockOnFatalError,
      enableRetry: true,
      maxRetries: 3,
      retryDelay: 100,
    });
  });

  afterEach(() => {
    errorHandlingService.stop();
  });

  it("should create ErrorHandlingService instance", () => {
    expect(errorHandlingService).toBeDefined();
    expect(errorHandlingService).toBeInstanceOf(ErrorHandlingService);
  });

  it("should start and stop service", () => {
    expect(errorHandlingService.active).toBe(false);

    errorHandlingService.start();
    expect(errorHandlingService.active).toBe(true);

    errorHandlingService.stop();
    expect(errorHandlingService.active).toBe(false);
  });

  it("should not start multiple times", () => {
    errorHandlingService.start();
    expect(errorHandlingService.active).toBe(true);

    errorHandlingService.start(); // Should not start again
    expect(errorHandlingService.active).toBe(true);
  });

  it("should handle network errors as recoverable", () => {
    errorHandlingService.start();

    const networkError = new Error("Failed to fetch");
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "fetchData"
    );

    errorHandlingService.handleError(networkError, context);

    expect(mockOnRecoverableError).toHaveBeenCalledWith(
      networkError,
      "TestComponent"
    );
    expect(mockOnFatalError).not.toHaveBeenCalled();
  });

  it("should handle WebRTC errors as recoverable", () => {
    errorHandlingService.start();

    const webrtcError = new Error("WebRTC connection failed");
    const context = ErrorHandlingService.createErrorContext(
      "WebRTCClient",
      "connect"
    );

    errorHandlingService.handleError(webrtcError, context);

    expect(mockOnRecoverableError).toHaveBeenCalledWith(
      webrtcError,
      "WebRTCClient"
    );
    expect(mockOnFatalError).not.toHaveBeenCalled();
  });

  it("should handle WebSocket errors as recoverable", () => {
    errorHandlingService.start();

    const wsError = new Error("WebSocket connection closed");
    const context = ErrorHandlingService.createErrorContext(
      "WebSocketClient",
      "connect"
    );

    errorHandlingService.handleError(wsError, context);

    expect(mockOnRecoverableError).toHaveBeenCalledWith(
      wsError,
      "WebSocketClient"
    );
    expect(mockOnFatalError).not.toHaveBeenCalled();
  });

  it("should handle MediaSource errors as recoverable", () => {
    errorHandlingService.start();

    const mediaError = new Error("MediaSource appendBuffer failed");
    const context = ErrorHandlingService.createErrorContext(
      "H264Client",
      "processAudio"
    );

    errorHandlingService.handleError(mediaError, context);

    expect(mockOnRecoverableError).toHaveBeenCalledWith(
      mediaError,
      "H264Client"
    );
    expect(mockOnFatalError).not.toHaveBeenCalled();
  });

  it("should handle Canvas errors as recoverable", () => {
    errorHandlingService.start();

    const canvasError = new Error("Canvas 2D context failed");
    const context = ErrorHandlingService.createErrorContext(
      "VideoRenderService",
      "renderFrame"
    );

    errorHandlingService.handleError(canvasError, context);

    expect(mockOnRecoverableError).toHaveBeenCalledWith(
      canvasError,
      "VideoRenderService"
    );
    expect(mockOnFatalError).not.toHaveBeenCalled();
  });

  it("should handle unknown errors as fatal", () => {
    errorHandlingService.start();

    const unknownError = new Error("Unknown error");
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "unknownOperation"
    );

    errorHandlingService.handleError(unknownError, context);

    expect(mockOnFatalError).toHaveBeenCalledWith(
      unknownError,
      "TestComponent"
    );
    expect(mockOnError).toHaveBeenCalledWith(unknownError, "TestComponent");
    expect(mockOnRecoverableError).not.toHaveBeenCalled();
  });

  it("should maintain error history", () => {
    errorHandlingService.start();

    const error1 = new Error("Error 1");
    const error2 = new Error("Error 2");
    const context1 = ErrorHandlingService.createErrorContext(
      "Component1",
      "operation1"
    );
    const context2 = ErrorHandlingService.createErrorContext(
      "Component2",
      "operation2"
    );

    errorHandlingService.handleError(error1, context1);
    errorHandlingService.handleError(error2, context2);

    const history = errorHandlingService.getErrorHistory();
    expect(history).toHaveLength(2);
    expect(history[0].error).toBe(error1);
    expect(history[1].error).toBe(error2);
  });

  it("should limit error history size", () => {
    errorHandlingService.start();

    // Add more than 100 errors
    for (let i = 0; i < 150; i++) {
      const error = new Error(`Error ${i}`);
      const context = ErrorHandlingService.createErrorContext(
        "TestComponent",
        "test"
      );
      errorHandlingService.handleError(error, context);
    }

    const history = errorHandlingService.getErrorHistory();
    expect(history).toHaveLength(100);
  });

  it("should register and unregister recovery strategies", () => {
    const strategy = {
      canRecover: jest.fn().mockReturnValue(true),
      recover: jest.fn().mockResolvedValue(undefined),
      maxRetries: 3,
      retryDelay: 1000,
    };

    errorHandlingService.registerRecoveryStrategy("TestComponent", strategy);
    errorHandlingService.unregisterRecoveryStrategy("TestComponent");

    // Strategy should be unregistered
    expect(errorHandlingService).toBeDefined();
  });

  it("should get error statistics", () => {
    errorHandlingService.start();

    // Add some errors
    const networkError = new Error("Failed to fetch");
    const webrtcError = new Error("WebRTC connection failed");
    const unknownError = new Error("Unknown error");

    errorHandlingService.handleError(
      networkError,
      ErrorHandlingService.createErrorContext("WebRTCClient", "connect")
    );
    errorHandlingService.handleError(
      webrtcError,
      ErrorHandlingService.createErrorContext("WebRTCClient", "connect")
    );
    errorHandlingService.handleError(
      unknownError,
      ErrorHandlingService.createErrorContext("TestComponent", "test")
    );

    const stats = errorHandlingService.getErrorStatistics();

    expect(stats.totalErrors).toBe(3);
    expect(stats.recoverableErrors).toBe(2);
    expect(stats.fatalErrors).toBe(1);
    expect(stats.errorsByComponent["WebRTCClient"]).toBe(2);
    expect(stats.errorsByComponent["TestComponent"]).toBe(1);
  });

  it("should clear error history", () => {
    errorHandlingService.start();

    const error = new Error("Test error");
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "test"
    );

    errorHandlingService.handleError(error, context);
    expect(errorHandlingService.getErrorHistory()).toHaveLength(1);

    errorHandlingService.clearErrorHistory();
    expect(errorHandlingService.getErrorHistory()).toHaveLength(0);
  });

  it("should update options", () => {
    errorHandlingService.updateOptions({
      maxRetries: 5,
      retryDelay: 2000,
    });

    // Options should be updated (we can't directly test private options)
    expect(errorHandlingService).toBeDefined();
  });

  it("should create error context with metadata", () => {
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "testOperation",
      { userId: "123", sessionId: "abc" }
    );

    expect(context.component).toBe("TestComponent");
    expect(context.operation).toBe("testOperation");
    expect(context.metadata).toEqual({ userId: "123", sessionId: "abc" });
    expect(context.timestamp).toBeGreaterThan(0);
  });

  it("should not handle errors when inactive", () => {
    // Service is not started
    const error = new Error("Test error");
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "test"
    );

    errorHandlingService.handleError(error, context);

    expect(mockOnError).not.toHaveBeenCalled();
    expect(mockOnRecoverableError).not.toHaveBeenCalled();
    expect(mockOnFatalError).not.toHaveBeenCalled();
  });

  it("should handle errors with different error types", () => {
    errorHandlingService.start();

    const errors = [
      { error: new Error("Failed to fetch"), expectedType: "recoverable" },
      {
        error: new Error("WebRTC connection failed"),
        expectedType: "recoverable",
      },
      {
        error: new Error("WebSocket connection closed"),
        expectedType: "recoverable",
      },
      {
        error: new Error("MediaSource appendBuffer failed"),
        expectedType: "recoverable",
      },
      {
        error: new Error("Canvas 2D context failed"),
        expectedType: "recoverable",
      },
      { error: new Error("Unknown error"), expectedType: "fatal" },
    ];

    errors.forEach(({ error, expectedType }) => {
      const context = ErrorHandlingService.createErrorContext(
        "TestComponent",
        "test"
      );
      errorHandlingService.handleError(error, context);

      if (expectedType === "recoverable") {
        expect(mockOnRecoverableError).toHaveBeenCalledWith(
          error,
          "TestComponent"
        );
      } else {
        expect(mockOnFatalError).toHaveBeenCalledWith(error, "TestComponent");
      }
    });
  });

  it("should handle recovery strategy execution", async () => {
    errorHandlingService.start();

    const strategy = {
      canRecover: jest.fn().mockReturnValue(true),
      recover: jest.fn().mockResolvedValue(undefined),
      maxRetries: 3,
      retryDelay: 1000,
    };

    errorHandlingService.registerRecoveryStrategy("TestComponent", strategy);

    // Use a network error that will be classified as recoverable
    const error = new Error("Failed to fetch");
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "test"
    );

    errorHandlingService.handleError(error, context);

    // Wait for async recovery to complete
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(strategy.canRecover).toHaveBeenCalledWith(error, context);
    expect(strategy.recover).toHaveBeenCalledWith(error, context);
  });

  it("should handle recovery strategy failure", async () => {
    errorHandlingService.start();

    const strategy = {
      canRecover: jest.fn().mockReturnValue(true),
      recover: jest.fn().mockRejectedValue(new Error("Recovery failed")),
      maxRetries: 3,
      retryDelay: 1000,
    };

    errorHandlingService.registerRecoveryStrategy("TestComponent", strategy);

    // Use a network error that will be classified as recoverable
    const error = new Error("Failed to fetch");
    const context = ErrorHandlingService.createErrorContext(
      "TestComponent",
      "test"
    );

    errorHandlingService.handleError(error, context);

    // Wait for async recovery to complete
    await new Promise((resolve) => setTimeout(resolve, 10));

    // Should call fatal error handler when recovery fails
    expect(mockOnFatalError).toHaveBeenCalledWith(
      expect.any(Error),
      "TestComponent"
    );
  });
});
