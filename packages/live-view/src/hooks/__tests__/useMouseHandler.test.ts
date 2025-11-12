/* eslint-disable @typescript-eslint/no-explicit-any */
/// <reference types="jest" />
import { useMouseHandler } from "../useMouseHandler";
import { ControlClient } from "../../lib/types";
import { renderHook, act } from "@testing-library/react";

// Mock ControlClient
class MockControlClient implements ControlClient {
  public mouseEvents: Array<{
    x: number;
    y: number;
    action: string;
    button: number;
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

  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void {
    // Simplified implementation for tests - just use default values
    const x = 0.5; // default normalized position
    const y = 0.5;
    // Try to get button from event or use 0 as default
    const button = event?.button ?? 0;
    this.mouseEvents.push({ x, y, action, button });
  }

  handleTouchEvent(_event: TouchEvent, _action: "down" | "up" | "move"): void {
    // Mock implementation
  }
}

describe("useMouseHandler", () => {
  let mockClient: MockControlClient;

  beforeEach(() => {
    mockClient = new MockControlClient();
    jest.clearAllMocks();
  });

  it("should provide mouse handlers and state", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    expect(result.current.handleMouseDown).toBeDefined();
    expect(result.current.handleMouseUp).toBeDefined();
    expect(result.current.handleMouseMove).toBeDefined();
    expect(result.current.handleMouseLeave).toBeDefined();
    expect(result.current.isMouseDragging).toBe(false);
    expect(typeof result.current.handleMouseDown).toBe("function");
    expect(typeof result.current.handleMouseUp).toBe("function");
    expect(typeof result.current.handleMouseMove).toBe("function");
    expect(typeof result.current.handleMouseLeave).toBe("function");
  });

  it("should handle mouse down events correctly", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    act(() => {
      result.current.handleMouseDown(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(true);
    expect(mockClient.mouseEvents).toHaveLength(1);
    expect(mockClient.mouseEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "down",
      button: 0,
    });
  });

  it("should handle mouse up events correctly", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 150,
      clientY: 250,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    act(() => {
      result.current.handleMouseUp(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(1);
    expect(mockClient.mouseEvents[0]).toEqual({
      x: 0.5, // default test value
      y: 0.5, // default test value
      action: "up",
      button: 0,
    });
  });

  it("should handle mouse move events only when dragging", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 200,
      clientY: 300,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    // First move without dragging - should not trigger
    act(() => {
      result.current.handleMouseMove(mockEvent as any);
    });

    expect(mockClient.mouseEvents).toHaveLength(0);

    // Start dragging
    act(() => {
      result.current.handleMouseDown(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(true);

    // Move while dragging - should trigger
    act(() => {
      result.current.handleMouseMove(mockEvent as any);
    });

    expect(mockClient.mouseEvents).toHaveLength(2); // down + move
    expect(mockClient.mouseEvents[1].action).toBe("move");
  });

  it("should handle mouse leave events correctly", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    // Start dragging first
    act(() => {
      result.current.handleMouseDown(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(true);

    // Mouse leave should end dragging
    act(() => {
      result.current.handleMouseLeave(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(2); // down + up (from leave)
    expect(mockClient.mouseEvents[1].action).toBe("up");
  });

  it("should handle mouse leave when not dragging", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    // Mouse leave without dragging - should not trigger
    act(() => {
      result.current.handleMouseLeave(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(0);
  });

  it("should not handle events when disabled", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: false,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    act(() => {
      result.current.handleMouseDown(mockEvent as any);
      result.current.handleMouseUp(mockEvent as any);
      result.current.handleMouseMove(mockEvent as any);
      result.current.handleMouseLeave(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(0);
  });

  it("should not handle events when not connected", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: false,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    act(() => {
      result.current.handleMouseDown(mockEvent as any);
      result.current.handleMouseUp(mockEvent as any);
      result.current.handleMouseMove(mockEvent as any);
      result.current.handleMouseLeave(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(0);
  });

  it("should not handle events when client is null", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: null,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 0,
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
    } as unknown as MouseEvent;

    act(() => {
      result.current.handleMouseDown(mockEvent as any);
      result.current.handleMouseUp(mockEvent as any);
      result.current.handleMouseMove(mockEvent as any);
      result.current.handleMouseLeave(mockEvent as any);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(0);
  });

  it("should handle different mouse buttons", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
      button: 2, // Right mouse button
      target: {
        getBoundingClientRect: () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 300,
        }),
      },
      nativeEvent: {
        clientX: 100,
        clientY: 200,
        button: 2, // Right mouse button
        target: {
          getBoundingClientRect: () => ({
            left: 0,
            top: 0,
            width: 400,
            height: 300,
          }),
        },
      },
    } as unknown as MouseEvent;

    act(() => {
      result.current.handleMouseDown(mockEvent as any);
    });

    expect(mockClient.mouseEvents[0].button).toBe(2);
  });

  it("should handle complex drag sequence", () => {
    const { result } = renderHook(() =>
      useMouseHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const createMockEvent = (x: number, y: number, button: number = 0) =>
      ({
        clientX: x,
        clientY: y,
        button,
        target: {
          getBoundingClientRect: () => ({
            left: 0,
            top: 0,
            width: 400,
            height: 300,
          }),
        },
      } as any);

    // Start drag
    act(() => {
      result.current.handleMouseDown(createMockEvent(100, 200));
    });

    expect(result.current.isMouseDragging).toBe(true);
    expect(mockClient.mouseEvents).toHaveLength(1);

    // Move while dragging
    act(() => {
      result.current.handleMouseMove(createMockEvent(150, 250));
    });

    expect(mockClient.mouseEvents).toHaveLength(2);
    expect(mockClient.mouseEvents[1].action).toBe("move");

    // Move again
    act(() => {
      result.current.handleMouseMove(createMockEvent(200, 300));
    });

    expect(mockClient.mouseEvents).toHaveLength(3);
    expect(mockClient.mouseEvents[2].action).toBe("move");

    // End drag
    act(() => {
      result.current.handleMouseUp(createMockEvent(200, 300));
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(4);
    expect(mockClient.mouseEvents[3].action).toBe("up");
  });
});
