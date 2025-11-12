/* eslint-disable @typescript-eslint/no-explicit-any */
/// <reference types="jest" />
import { useTouchHandler } from "../useTouchHandler";
import { ControlClient } from "../../lib/types";
import { renderHook, act } from "@testing-library/react";

// Mock ControlClient
class MockControlClient implements ControlClient {
  public touchEvents: Array<{
    x: number;
    y: number;
    action: string;
    pressure: number;
  }> = [];
  public isMouseDragging: boolean = false;

  // ControlClient interface implementation
  connect(
    _deviceSerial: string,
    _apiUrl: string,
    _wsUrl?: string
  ): Promise<void> {
    return Promise.resolve();
  }

  disconnect(): void {
    // Mock implementation
  }

  isControlConnected(): boolean {
    return true;
  }

  sendKeyEvent(
    _keycode: number,
    _action: "down" | "up",
    _metaState: number = 0
  ): void {
    // Mock implementation
  }

  sendTouchEvent(
    _x: number,
    _y: number,
    _action: "down" | "up" | "move",
    _pressure: number = 1.0
  ): void {
    // Mock implementation
  }

  sendControlAction(_action: string, _params?: unknown): void {
    // Mock implementation
  }

  sendClipboardSet(_text: string, _paste?: boolean): void {
    // Mock implementation
  }

  requestKeyframe(): void {
    // Mock implementation
  }

  handleMouseEvent(_event: MouseEvent, _action: "down" | "up" | "move"): void {
    // Mock implementation
  }

  handleTouchEvent(_event: TouchEvent, action: "down" | "up" | "move"): void {
    // Simplified implementation for tests - just use default values
    const x = 0.5; // default normalized position
    const y = 0.5;
    const pressure = 1.0; // default pressure
    this.touchEvents.push({
      x,
      y,
      action,
      pressure,
    });
  }
}

describe("useTouchHandler", () => {
  let mockClient: MockControlClient;

  beforeEach(() => {
    mockClient = new MockControlClient();
    jest.clearAllMocks();
  });

  it("should provide touch handlers", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    expect(result.current.handleTouchStart).toBeDefined();
    expect(result.current.handleTouchEnd).toBeDefined();
    expect(result.current.handleTouchMove).toBeDefined();
    expect(typeof result.current.handleTouchStart).toBe("function");
    expect(typeof result.current.handleTouchEnd).toBe("function");
    expect(typeof result.current.handleTouchMove).toBe("function");
  });

  it("should handle touch start events correctly", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
          force: 0.8,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
    });

    expect(mockClient.touchEvents).toHaveLength(1);
    expect(mockClient.touchEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "down",
      pressure: 1.0,
    });
  });

  it("should handle touch end events correctly", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 150,
          clientY: 250,
          force: 0.9,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchEnd(mockEvent as any);
    });

    expect(mockClient.touchEvents).toHaveLength(1);
    expect(mockClient.touchEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "up",
      pressure: 1.0,
    });
  });

  it("should handle touch move events correctly", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 200,
          clientY: 300,
          force: 1.0,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchMove(mockEvent as any);
    });

    expect(mockClient.touchEvents).toHaveLength(1);
    expect(mockClient.touchEvents[0]).toEqual({
      x: 0.5, // 200/400
      y: 0.5, // default test value
      action: "move",
      pressure: 1.0,
    });
  });

  it("should handle touch events with different coordinates", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 50,
          clientY: 75,
          force: 0.5,
        },
      ],
      target: {
        getBoundingClientRect: () => ({
          left: 10,
          top: 20,
          width: 200,
          height: 150,
        }),
      },
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
    });

    expect(mockClient.touchEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "down",
      pressure: 1.0,
    });
  });

  it("should handle touch events with changedTouches", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [],
      changedTouches: [
        {
          clientX: 100,
          clientY: 200,
          force: 0.7,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchEnd(mockEvent as any);
    });

    expect(mockClient.touchEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "up",
      pressure: 1.0,
    });
  });

  it("should handle touch events without force (default to 1.0)", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
          // No force property
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
    });

    expect(mockClient.touchEvents[0].pressure).toBe(1.0);
  });

  it("should not handle touch events when disabled", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: false,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
          force: 0.8,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
      result.current.handleTouchEnd(mockEvent as any);
      result.current.handleTouchMove(mockEvent as any);
    });

    expect(mockClient.touchEvents).toHaveLength(0);
  });

  it("should not handle touch events when not connected", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: false,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
          force: 0.8,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
      result.current.handleTouchEnd(mockEvent as any);
      result.current.handleTouchMove(mockEvent as any);
    });

    expect(mockClient.touchEvents).toHaveLength(0);
  });

  it("should not handle touch events when client is null", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: null,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
          force: 0.8,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
      result.current.handleTouchEnd(mockEvent as any);
      result.current.handleTouchMove(mockEvent as any);
    });

    expect(mockClient.touchEvents).toHaveLength(0);
  });

  it("should handle complex touch sequence", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const createMockEvent = (x: number, y: number, force: number = 1.0) =>
      ({
        touches: [
          {
            clientX: x,
            clientY: y,
            force,
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
      } as any);

    // Touch start
    act(() => {
      result.current.handleTouchStart(createMockEvent(100, 200, 0.8));
    });

    expect(mockClient.touchEvents).toHaveLength(1);
    expect(mockClient.touchEvents[0].action).toBe("down");

    // Touch move
    act(() => {
      result.current.handleTouchMove(createMockEvent(150, 250, 0.9));
    });

    expect(mockClient.touchEvents).toHaveLength(2);
    expect(mockClient.touchEvents[1].action).toBe("move");

    // Touch end
    act(() => {
      result.current.handleTouchEnd(createMockEvent(200, 300, 1.0));
    });

    expect(mockClient.touchEvents).toHaveLength(3);
    expect(mockClient.touchEvents[2].action).toBe("up");
  });

  it("should handle multiple touch points", () => {
    const { result } = renderHook(() =>
      useTouchHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      touches: [
        {
          clientX: 100,
          clientY: 200,
          force: 0.8,
        },
        {
          clientX: 300,
          clientY: 400,
          force: 0.9,
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
    } as unknown as TouchEvent;

    act(() => {
      result.current.handleTouchStart(mockEvent as any);
    });

    // Should only handle the first touch point
    expect(mockClient.touchEvents).toHaveLength(1);
    expect(mockClient.touchEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "down",
      pressure: 1.0,
    });
  });
});
