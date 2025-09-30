/// <reference types="jest" />
import { useWheelHandler } from "../useWheelHandler";
import { ControlClient } from "../../lib/types";
import { renderHook, act } from "@testing-library/react";

// Mock ControlClient
class MockControlClient implements ControlClient {
  public controlActions: Array<{
    action: string;
    data?: unknown;
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

  sendControlAction(action: string, params?: unknown): void {
    this.controlActions.push({ action, data: params });
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

  handleTouchEvent(_event: TouchEvent, _action: "down" | "up" | "move"): void {
    // Mock implementation
  }
}

describe("useWheelHandler", () => {
  let mockClient: MockControlClient;

  beforeEach(() => {
    mockClient = new MockControlClient();
    jest.clearAllMocks();
  });

  it("should provide wheel handler", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    expect(result.current.handleWheel).toBeDefined();
    expect(typeof result.current.handleWheel).toBe("function");
  });

  it("should handle wheel events correctly", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 400,
        height: 300,
      }),
    };

    const mockEvent = {
      clientX: 200,
      clientY: 150,
      deltaX: 10,
      deltaY: -20,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockEvent.stopPropagation).toHaveBeenCalled();
    expect(mockClient.controlActions).toHaveLength(1);
    expect(mockClient.controlActions[0]).toEqual({
      action: "scroll",
      data: {
        x: 0.5, // 200/400
        y: 0.5, // 150/300
        hScroll: -5, // -10 * 0.5
        vScroll: 10, // -(-20) * 0.5
        timestamp: expect.any(Number),
      },
    });
  });

  it("should handle wheel events with different coordinates", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 100,
        top: 50,
        width: 200,
        height: 150,
      }),
    };

    const mockEvent = {
      clientX: 150,
      clientY: 100,
      deltaX: -5,
      deltaY: 15,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    expect(mockClient.controlActions[0].data).toEqual({
      x: 0.25, // (150-100)/200
      y: 0.3333333333333333, // (100-50)/150
      hScroll: 2.5, // -(-5) * 0.5
      vScroll: -7.5, // -(15) * 0.5
      timestamp: expect.any(Number),
    });
  });

  it("should clamp scroll values to valid range", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 100,
        height: 100,
      }),
    };

    // Test with very large delta values
    const mockEvent = {
      clientX: 50,
      clientY: 50,
      deltaX: 1000, // Very large delta
      deltaY: -1000,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    expect((mockClient.controlActions[0].data as { hScroll: number; vScroll: number }).hScroll).toBe(-16); // Clamped to min -16
    expect((mockClient.controlActions[0].data as { hScroll: number; vScroll: number }).vScroll).toBe(16); // Clamped to max 16
  });

  it("should handle zero scroll values", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 100,
        height: 100,
      }),
    };

    const mockEvent = {
      clientX: 50,
      clientY: 50,
      deltaX: 0,
      deltaY: 0,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    // Should not send control action when both scroll values are 0
    expect(mockClient.controlActions).toHaveLength(0);
  });

  it("should handle coordinates outside bounds", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 100,
        height: 100,
      }),
    };

    // Test with coordinates outside the element bounds
    const mockEvent = {
      clientX: 150, // Outside width
      clientY: 150, // Outside height
      deltaX: 10,
      deltaY: -10,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    // Should not send control action when coordinates are outside bounds
    expect(mockClient.controlActions).toHaveLength(0);
  });

  it("should not handle wheel events when disabled", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: false,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 100,
        height: 100,
      }),
    };

    const mockEvent = {
      clientX: 50,
      clientY: 50,
      deltaX: 10,
      deltaY: -10,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    expect(mockEvent.preventDefault).not.toHaveBeenCalled();
    expect(mockEvent.stopPropagation).not.toHaveBeenCalled();
    expect(mockClient.controlActions).toHaveLength(0);
  });

  it("should not handle wheel events when not connected", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: false,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 100,
        height: 100,
      }),
    };

    const mockEvent = {
      clientX: 50,
      clientY: 50,
      deltaX: 10,
      deltaY: -10,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    expect(mockEvent.preventDefault).not.toHaveBeenCalled();
    expect(mockEvent.stopPropagation).not.toHaveBeenCalled();
    expect(mockClient.controlActions).toHaveLength(0);
  });

  it("should not handle wheel events when client is null", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: null,
        enabled: true,
        isConnected: true,
      })
    );

    const mockElement = {
      getBoundingClientRect: () => ({
        left: 0,
        top: 0,
        width: 100,
        height: 100,
      }),
    };

    const mockEvent = {
      clientX: 50,
      clientY: 50,
      deltaX: 10,
      deltaY: -10,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: mockElement,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    expect(mockEvent.preventDefault).not.toHaveBeenCalled();
    expect(mockEvent.stopPropagation).not.toHaveBeenCalled();
    expect(mockClient.controlActions).toHaveLength(0);
  });

  it("should handle missing target element gracefully", () => {
    const { result } = renderHook(() =>
      useWheelHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 50,
      clientY: 50,
      deltaX: 10,
      deltaY: -10,
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      target: null,
    } as unknown as WheelEvent;

    act(() => {
      result.current.handleWheel(mockEvent);
    });

    // Should not send control action when target is null
    expect(mockClient.controlActions).toHaveLength(0);
  });
});
