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

  // ControlClient interface implementation
  connect(): Promise<void> {
    return Promise.resolve();
  }

  disconnect(): void {
    // Mock implementation
  }

  isControlConnected(): boolean {
    return true;
  }

  sendKeyEvent(keycode: number, action: string, metaState: number = 0): void {
    // Mock implementation
  }

  sendTouchEvent(
    x: number,
    y: number,
    action: string,
    pressure: number = 1.0
  ): void {
    // Mock implementation
  }

  sendControlAction(action: string, data?: any): void {
    // Mock implementation
  }

  sendClipboardSet(text: string, paste: boolean): void {
    // Mock implementation
  }

  requestKeyframe(): void {
    // Mock implementation
  }

  handleMouseEvent(event: any, action: string): void {
    const rect = (event.target as HTMLElement).getBoundingClientRect();
    const x = (event.clientX - rect.left) / rect.width;
    const y = (event.clientY - rect.top) / rect.height;
    this.mouseEvents.push({ x, y, action, button: event.button });
  }

  handleTouchEvent(event: any, action: string): void {
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
    } as any;

    act(() => {
      result.current.handleMouseDown(mockEvent);
    });

    expect(result.current.isMouseDragging).toBe(true);
    expect(mockClient.mouseEvents).toHaveLength(1);
    expect(mockClient.mouseEvents[0]).toEqual({
      x: 0.25, // 100/400
      y: 0.6666666666666666, // 200/300
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
    } as any;

    act(() => {
      result.current.handleMouseUp(mockEvent);
    });

    expect(result.current.isMouseDragging).toBe(false);
    expect(mockClient.mouseEvents).toHaveLength(1);
    expect(mockClient.mouseEvents[0]).toEqual({
      x: 0.375, // 150/400
      y: 0.8333333333333334, // 250/300
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
    } as any;

    // First move without dragging - should not trigger
    act(() => {
      result.current.handleMouseMove(mockEvent);
    });

    expect(mockClient.mouseEvents).toHaveLength(0);

    // Start dragging
    act(() => {
      result.current.handleMouseDown(mockEvent);
    });

    expect(result.current.isMouseDragging).toBe(true);

    // Move while dragging - should trigger
    act(() => {
      result.current.handleMouseMove(mockEvent);
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
    } as any;

    // Start dragging first
    act(() => {
      result.current.handleMouseDown(mockEvent);
    });

    expect(result.current.isMouseDragging).toBe(true);

    // Mouse leave should end dragging
    act(() => {
      result.current.handleMouseLeave(mockEvent);
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
    } as any;

    // Mouse leave without dragging - should not trigger
    act(() => {
      result.current.handleMouseLeave(mockEvent);
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
    } as any;

    act(() => {
      result.current.handleMouseDown(mockEvent);
      result.current.handleMouseUp(mockEvent);
      result.current.handleMouseMove(mockEvent);
      result.current.handleMouseLeave(mockEvent);
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
    } as any;

    act(() => {
      result.current.handleMouseDown(mockEvent);
      result.current.handleMouseUp(mockEvent);
      result.current.handleMouseMove(mockEvent);
      result.current.handleMouseLeave(mockEvent);
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
    } as any;

    act(() => {
      result.current.handleMouseDown(mockEvent);
      result.current.handleMouseUp(mockEvent);
      result.current.handleMouseMove(mockEvent);
      result.current.handleMouseLeave(mockEvent);
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
    } as any;

    act(() => {
      result.current.handleMouseDown(mockEvent);
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
