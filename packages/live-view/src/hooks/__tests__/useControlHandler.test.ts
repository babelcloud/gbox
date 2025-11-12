/// <reference types="jest" />
import { useControlHandler } from "../useControlHandler";
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

describe("useControlHandler", () => {
  let mockClient: MockControlClient;

  beforeEach(() => {
    mockClient = new MockControlClient();
    jest.clearAllMocks();
  });

  it("should provide control handlers", () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    expect(result.current.handleControlAction).toBeDefined();
    expect(result.current.handleIMESwitch).toBeDefined();
    expect(typeof result.current.handleControlAction).toBe("function");
    expect(typeof result.current.handleIMESwitch).toBe("function");
  });

  it("should handle control actions correctly", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("power");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2); // down + up
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 26, // power keycode
      action: "down",
      metaState: 0,
    });
    expect(mockClient.keyEvents[1]).toEqual({
      keycode: 26,
      action: "up",
      metaState: 0,
    });
  });

  it("should handle volume up action", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("volume_up");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(24); // volume up
  });

  it("should handle volume down action", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("volume_down");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(25); // volume down
  });

  it("should handle back action", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("back");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(4); // back
  });

  it("should handle home action", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("home");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(3); // home
  });

  it("should handle app switch action", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("app_switch");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(187); // app switch
  });

  it("should handle menu action", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("menu");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(82); // menu
  });

  it("should handle unknown action gracefully", () => {
    const consoleSpy = jest.spyOn(console, "warn").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("unknown_action");
    });

    expect(mockClient.keyEvents).toHaveLength(0);
    expect(consoleSpy).toHaveBeenCalledWith(
      "[ControlHandler] No keycode found for action: unknown_action"
    );

    consoleSpy.mockRestore();
  });

  it("should handle IME switch correctly", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleIMESwitch();
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 100));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 204, // KEYCODE_LANGUAGE_SWITCH
      action: "down",
      metaState: 0,
    });
    expect(mockClient.keyEvents[1]).toEqual({
      keycode: 204,
      action: "up",
      metaState: 0,
    });
  });

  it("should not handle actions when disabled", () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: false,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("power");
      result.current.handleIMESwitch();
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should not handle actions when not connected", () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: false,
      })
    );

    act(() => {
      result.current.handleControlAction("power");
      result.current.handleIMESwitch();
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should not handle actions when client is null", () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: null,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("power");
      result.current.handleIMESwitch();
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should handle case insensitive actions", async () => {
    const { result } = renderHook(() =>
      useControlHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
      })
    );

    act(() => {
      result.current.handleControlAction("POWER");
    });

    // Wait for the async up event
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(26); // power keycode
  });
});
