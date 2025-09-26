/// <reference types="jest" />
import { useClipboardHandler } from "../useClipboardHandler";
import { ControlClient } from "../../lib/types";
import { renderHook, act } from "@testing-library/react";

// Mock ControlClient
class MockControlClient implements ControlClient {
  public clipboardEvents: Array<{
    text: string;
    paste: boolean;
  }> = [];
  public controlActions: Array<{ action: string; data: unknown }> = [];
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
    this.controlActions.push({ action, data: params || {} });
  }

  sendClipboardSet(text: string, paste?: boolean): void {
    this.clipboardEvents.push({ text, paste: paste || false });
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

// Mock navigator.clipboard
Object.defineProperty(navigator, "clipboard", {
  value: {
    readText: jest.fn().mockResolvedValue("test clipboard text"),
    writeText: jest.fn().mockResolvedValue(undefined),
  },
  writable: true,
});

describe("useClipboardHandler", () => {
  let mockClient: MockControlClient;
  let mockOnError: jest.Mock;

  beforeEach(() => {
    mockClient = new MockControlClient();
    mockOnError = jest.fn();
    jest.clearAllMocks();
  });

  it("should provide clipboard handlers", () => {
    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
        onError: mockOnError,
      })
    );

    expect(result.current.handleClipboardPaste).toBeDefined();
    expect(result.current.handleClipboardCopy).toBeDefined();
    expect(typeof result.current.handleClipboardPaste).toBe("function");
    expect(typeof result.current.handleClipboardCopy).toBe("function");
  });

  it("should handle clipboard paste correctly", async () => {
    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
        onError: mockOnError,
      })
    );

    await act(async () => {
      await result.current.handleClipboardPaste();
    });

    expect(navigator.clipboard.readText).toHaveBeenCalled();
    expect(mockClient.clipboardEvents).toHaveLength(1);
    expect(mockClient.clipboardEvents[0]).toEqual({
      text: "test clipboard text",
      paste: true,
    });
  });

  it("should handle clipboard copy correctly", () => {
    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
        onError: mockOnError,
      })
    );

    act(() => {
      result.current.handleClipboardCopy();
    });

    expect(mockClient.controlActions).toHaveLength(1);
    expect(mockClient.controlActions[0]).toEqual({
      action: "clipboard_get",
      data: {},
    });
  });

  it("should not handle clipboard operations when disabled", async () => {
    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: false,
        isConnected: true,
        onError: mockOnError,
      })
    );

    await act(async () => {
      await result.current.handleClipboardPaste();
    });

    act(() => {
      result.current.handleClipboardCopy();
    });

    expect(navigator.clipboard.readText).not.toHaveBeenCalled();
    expect(mockClient.clipboardEvents).toHaveLength(0);
  });

  it("should not handle clipboard operations when not connected", async () => {
    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: true,
        isConnected: false,
        onError: mockOnError,
      })
    );

    await act(async () => {
      await result.current.handleClipboardPaste();
    });

    act(() => {
      result.current.handleClipboardCopy();
    });

    expect(navigator.clipboard.readText).not.toHaveBeenCalled();
    expect(mockClient.clipboardEvents).toHaveLength(0);
  });

  it("should not handle clipboard operations when client is null", async () => {
    const { result } = renderHook(() =>
      useClipboardHandler({
        client: null,
        enabled: true,
        isConnected: true,
        onError: mockOnError,
      })
    );

    await act(async () => {
      await result.current.handleClipboardPaste();
    });

    act(() => {
      result.current.handleClipboardCopy();
    });

    expect(navigator.clipboard.readText).not.toHaveBeenCalled();
    expect(mockClient.clipboardEvents).toHaveLength(0);
  });

  it("should handle clipboard read errors", async () => {
    const error = new Error("Clipboard read failed");
    (navigator.clipboard.readText as jest.Mock).mockRejectedValueOnce(error);

    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
        onError: mockOnError,
      })
    );

    await act(async () => {
      await result.current.handleClipboardPaste();
    });

    expect(mockOnError).toHaveBeenCalledWith(error);
    expect(mockClient.clipboardEvents).toHaveLength(0);
  });

  it("should not send clipboard set when text is empty", async () => {
    (navigator.clipboard.readText as jest.Mock).mockResolvedValueOnce("");

    const { result } = renderHook(() =>
      useClipboardHandler({
        client: mockClient,
        enabled: true,
        isConnected: true,
        onError: mockOnError,
      })
    );

    await act(async () => {
      await result.current.handleClipboardPaste();
    });

    expect(navigator.clipboard.readText).toHaveBeenCalled();
    expect(mockClient.clipboardEvents).toHaveLength(0);
  });
});
