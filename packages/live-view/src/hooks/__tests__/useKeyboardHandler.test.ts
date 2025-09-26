/* eslint-disable @typescript-eslint/no-explicit-any */
/// <reference types="jest" />
import { useKeyboardHandler } from "../useKeyboardHandler";
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

// Mock clipboard handlers
const mockClipboardPaste = jest.fn();
const mockClipboardCopy = jest.fn();

describe("useKeyboardHandler", () => {
  let mockClient: MockControlClient;

  beforeEach(() => {
    mockClient = new MockControlClient();
    jest.clearAllMocks();
  });

  it("should provide keyboard handlers", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    expect(result.current.handleKeyDown).toBeDefined();
    expect(result.current.handleKeyUp).toBeDefined();
    expect(typeof result.current.handleKeyDown).toBe("function");
    expect(typeof result.current.handleKeyUp).toBe("function");
  });

  it("should handle key down events correctly", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
    });

    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(1);
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 29, // KeyA
      action: "down",
      metaState: 0,
    });
  });

  it("should handle key up events correctly", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyUp(mockEvent as any);
    });

    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(1);
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 29, // KeyA
      action: "up",
      metaState: 0,
    });
  });

  it("should handle clipboard paste shortcut (Ctrl+V)", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "v",
      code: "KeyV",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: true,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
    });

    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockClipboardPaste).toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(0); // Should not send key event
  });

  it("should handle clipboard copy shortcut (Ctrl+C)", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "c",
      code: "KeyC",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: true,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
    });

    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockClipboardCopy).toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(0); // Should not send key event
  });

  it("should handle Mac clipboard shortcuts (Cmd+V, Cmd+C)", () => {
    // Mock navigator.platform for Mac
    Object.defineProperty(navigator, "platform", {
      value: "MacIntel",
      writable: true,
    });

    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "v",
      code: "KeyV",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: true, // Cmd key on Mac
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
    });

    expect(mockClipboardPaste).toHaveBeenCalled();
  });

  it("should handle meta state correctly", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: true,
      metaKey: true,
      altKey: true,
      shiftKey: true,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
    });

    expect(mockClient.keyEvents[0].metaState).toBe(0x100b); // All modifiers
  });

  it("should handle special keys correctly", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const testCases = [
      { code: "Enter", expectedKeycode: 66 },
      { code: "Backspace", expectedKeycode: 67 },
      { code: "Delete", expectedKeycode: 112 },
      { code: "Escape", expectedKeycode: 111 },
      { code: "Tab", expectedKeycode: 61 },
      { code: "Space", expectedKeycode: 62 },
      { code: "ArrowUp", expectedKeycode: 19 },
      { code: "ArrowDown", expectedKeycode: 20 },
      { code: "ArrowLeft", expectedKeycode: 21 },
      { code: "ArrowRight", expectedKeycode: 22 },
    ];

    testCases.forEach(({ code, expectedKeycode }) => {
      mockClient.keyEvents = []; // Reset

      const mockEvent = {
        key: code,
        code,
        preventDefault: jest.fn(),
        stopPropagation: jest.fn(),
        ctrlKey: false,
        metaKey: false,
        altKey: false,
        shiftKey: false,
      } as unknown as KeyboardEvent;

      act(() => {
        result.current.handleKeyDown(mockEvent as any);
      });

      expect(mockClient.keyEvents[0].keycode).toBe(expectedKeycode);
    });
  });

  it("should not handle events when disabled", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: false,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
      result.current.handleKeyUp(mockEvent as any);
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should not handle events when keyboard capture is disabled", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: false,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
      result.current.handleKeyUp(mockEvent as any);
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should not handle events when not connected", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: false,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
      result.current.handleKeyUp(mockEvent as any);
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should not handle events when client is null", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: null,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "a",
      code: "KeyA",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: false,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyDown(mockEvent as any);
      result.current.handleKeyUp(mockEvent as any);
    });

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should prevent Cmd/Ctrl keyup from being sent", () => {
    const { result } = renderHook(() =>
      useKeyboardHandler({
        client: mockClient,
        enabled: true,
        keyboardCaptureEnabled: true,
        isConnected: true,
        onClipboardPaste: mockClipboardPaste,
        onClipboardCopy: mockClipboardCopy,
      })
    );

    const mockEvent = {
      key: "Meta",
      code: "MetaLeft",
      preventDefault: jest.fn(),
      stopPropagation: jest.fn(),
      ctrlKey: false,
      metaKey: true,
      altKey: false,
      shiftKey: false,
    } as unknown as KeyboardEvent;

    act(() => {
      result.current.handleKeyUp(mockEvent as any);
    });

    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockClient.keyEvents).toHaveLength(0);
  });
});
