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
    this.keyEvents.push({ keycode, action, metaState });
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
    // Mock implementation
  }

  handleTouchEvent(event: any, action: string): void {
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

  it("should handle control actions correctly", () => {
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

  it("should handle volume up action", () => {
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

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(24); // volume up
  });

  it("should handle volume down action", () => {
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

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(25); // volume down
  });

  it("should handle back action", () => {
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

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(4); // back
  });

  it("should handle home action", () => {
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

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(3); // home
  });

  it("should handle app switch action", () => {
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

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(187); // app switch
  });

  it("should handle menu action", () => {
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
      "[useControlHandler] No keycode found for action: unknown_action"
    );

    consoleSpy.mockRestore();
  });

  it("should handle IME switch correctly", () => {
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

  it("should handle case insensitive actions", () => {
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

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(26); // power keycode
  });
});
