/// <reference types="jest" />
import { useClickHandler } from "../useClickHandler";
import { ControlClient } from "../../lib/types";
import { renderHook, act } from "@testing-library/react";

// Mock ControlClient
class MockControlClient implements ControlClient {
  public keyEvents: Array<{
    keycode: number;
    action: string;
    metaState: number;
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
    keycode: number,
    action: "down" | "up",
    metaState: number = 0
  ): void {
    this.keyEvents.push({ keycode, action, metaState });
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

  handleTouchEvent(_event: TouchEvent, _action: "down" | "up" | "move"): void {
    // Mock implementation
  }
}

describe("useClickHandler", () => {
  let mockClient: MockControlClient;

  beforeEach(() => {
    mockClient = new MockControlClient();
    jest.clearAllMocks();
  });

  it("should provide click handler", () => {
    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    expect(result.current.handleClick).toBeDefined();
    expect(typeof result.current.handleClick).toBe("function");
  });

  it("should handle single click correctly", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Single click - position cursor"
    );
    expect(mockClient.keyEvents).toHaveLength(0); // Single click doesn't send key events

    consoleSpy.mockRestore();
  });

  it("should handle double click correctly", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    // First click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    // Second click within 500ms and 50px
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Double click - select word"
    );
    expect(mockClient.keyEvents).toHaveLength(0); // Double click doesn't send key events

    consoleSpy.mockRestore();
  });

  it("should handle triple click correctly", async () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    // First click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    // Second click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    // Third click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Triple click - select line"
    );

    // Wait for the async key events to complete
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockClient.keyEvents).toHaveLength(4); // Ctrl down, A down, A up, Ctrl up
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 113, // Ctrl down
      action: "down",
      metaState: 0x1000, // META_CTRL_ON
    });
    expect(mockClient.keyEvents[1]).toEqual({
      keycode: 29, // A down
      action: "down",
      metaState: 0x1000, // META_CTRL_ON
    });
    expect(mockClient.keyEvents[2]).toEqual({
      keycode: 29, // A up
      action: "up",
      metaState: 0x1000, // META_CTRL_ON
    });
    expect(mockClient.keyEvents[3]).toEqual({
      keycode: 113, // Ctrl up
      action: "up",
      metaState: 0,
    });

    consoleSpy.mockRestore();
  });

  it("should reset click count after triple click", async () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    // Triple click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    // Wait for the async key events to complete
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockClient.keyEvents).toHaveLength(4);

    // Reset click count, so next click should be single
    mockClient.keyEvents = [];

    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Single click - position cursor"
    );
    expect(mockClient.keyEvents).toHaveLength(0);

    consoleSpy.mockRestore();
  });

  it("should not count clicks that are too far apart in time", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    // First click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    // Wait more than 500ms (simulate by advancing time)
    jest.advanceTimersByTime(600);

    // Second click - should be treated as single click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Single click - position cursor"
    );
    expect(consoleSpy).toHaveBeenCalledTimes(2); // Two single clicks

    consoleSpy.mockRestore();
  });

  it("should not count clicks that are too far apart in position", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    // First click
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick({ clientX: 100, clientY: 200 } as any);
    });

    // Second click more than 50px away
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick({ clientX: 200, clientY: 300 } as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Single click - position cursor"
    );
    expect(consoleSpy).toHaveBeenCalledTimes(2); // Two single clicks

    consoleSpy.mockRestore();
  });

  it("should not handle clicks when disabled", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: false,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).not.toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(0);

    consoleSpy.mockRestore();
  });

  it("should not handle clicks when not connected", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: false,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).not.toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(0);

    consoleSpy.mockRestore();
  });

  it("should not handle clicks when client is null", () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: null,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).not.toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(0);

    consoleSpy.mockRestore();
  });

  it("should handle rapid clicks correctly", async () => {
    const consoleSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useClickHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    const mockEvent = {
      clientX: 100,
      clientY: 200,
    } as unknown as MouseEvent;

    // Rapid clicks
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });
    act(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      result.current.handleClick(mockEvent as any);
    });

    expect(consoleSpy).toHaveBeenCalledWith(
      "[ClickHandler] Triple click - select line"
    );

    // Wait for the async key events to complete
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockClient.keyEvents).toHaveLength(4); // Only triple click sends key events

    consoleSpy.mockRestore();
  });
});
