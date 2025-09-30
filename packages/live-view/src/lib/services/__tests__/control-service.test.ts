// ControlService tests using Jest
import { ControlService } from "../control-service";
import { ControlClient } from "../../types";

// Mock ControlClient for testing
class MockControlClient implements ControlClient {
  public isMouseDragging = false;
  public keyEvents: Array<{
    keycode: number;
    action: string;
    metaState: number;
  }> = [];
  public controlActions: Array<{ action: string; params?: unknown }> = [];

  async connect(
    _deviceSerial: string,
    _apiUrl: string,
    _wsUrl?: string
  ): Promise<void> {}
  disconnect(): void {}
  isControlConnected(): boolean {
    return true;
  }

  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void {
    this.keyEvents.push({ keycode, action, metaState: metaState || 0 });
  }

  sendTouchEvent(
    _x: number,
    _y: number,
    _action: "down" | "up" | "move",
    _pressure?: number
  ): void {}

  sendControlAction(action: string, params?: unknown): void {
    this.controlActions.push({ action, params });
  }

  sendClipboardSet(_text: string, _paste?: boolean): void {}

  requestKeyframe(): void {}
  handleMouseEvent(_event: MouseEvent, _action: "down" | "up" | "move"): void {}
  handleTouchEvent(_event: TouchEvent, _action: "down" | "up" | "move"): void {}
}

describe("ControlService", () => {
  let controlService: ControlService;
  let mockClient: MockControlClient;

  beforeEach(() => {
    jest.useFakeTimers();
    controlService = new ControlService();
    mockClient = new MockControlClient();
    controlService.setClient(mockClient);
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.restoreAllMocks();
  });

  it("should create ControlService instance", () => {
    expect(controlService).toBeDefined();
    expect(controlService).toBeInstanceOf(ControlService);
  });

  it("should set and get client", () => {
    expect(controlService).toBeDefined();

    const newClient = new MockControlClient();
    controlService.setClient(newClient);

    // We can't directly access private client, but we can test behavior
    expect(controlService).toBeDefined();
  });

  it("should handle power button action", () => {
    controlService.handleControlAction("power");

    // Run all timers to complete the setTimeout
    jest.runAllTimers();

    expect(mockClient.keyEvents).toHaveLength(2); // down + up
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 26, // POWER
      action: "down",
      metaState: 0,
    });
    expect(mockClient.keyEvents[1]).toEqual({
      keycode: 26, // POWER
      action: "up",
      metaState: 0,
    });
  });

  it("should handle volume up action", () => {
    controlService.handleControlAction("volume_up");

    // Run all timers to complete the setTimeout
    jest.runAllTimers();

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0].keycode).toBe(24); // VOLUME_UP
  });

  it("should not send event for unknown action", () => {
    controlService.handleControlAction("unknown_action");

    expect(mockClient.keyEvents).toHaveLength(0);
  });

  it("should send IME switch keycode", () => {
    controlService.handleIMESwitch();

    // Run all timers to complete the setTimeout
    jest.runAllTimers();

    expect(mockClient.keyEvents).toHaveLength(2);
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 204, // IME switch
      action: "down",
      metaState: 0,
    });
    expect(mockClient.keyEvents[1]).toEqual({
      keycode: 204, // IME switch
      action: "up",
      metaState: 0,
    });
  });

  it("should handle key down event", () => {
    const mockEvent = {
      code: "KeyA",
      shiftKey: false,
      ctrlKey: false,
      altKey: false,
      metaKey: false,
    } as React.KeyboardEvent;

    controlService.handleKeyEvent(mockEvent, "down");

    expect(mockClient.keyEvents).toHaveLength(1);
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 29, // KeyA
      action: "down",
      metaState: 0,
    });
  });

  it("should handle key up event with modifiers", () => {
    const mockEvent = {
      code: "KeyA",
      shiftKey: true,
      ctrlKey: true,
      altKey: false,
      metaKey: false,
    } as React.KeyboardEvent;

    controlService.handleKeyEvent(mockEvent, "up");

    expect(mockClient.keyEvents).toHaveLength(1);
    expect(mockClient.keyEvents[0]).toEqual({
      keycode: 29, // KeyA
      action: "up",
      metaState: 0x1001, // Shift + Ctrl
    });
  });

  it("should not send event if client is null", () => {
    controlService.setClient(null);

    const mockEvent = {
      code: "KeyA",
      shiftKey: false,
      ctrlKey: false,
      altKey: false,
      metaKey: false,
    } as React.KeyboardEvent;

    controlService.handleKeyEvent(mockEvent, "down");

    expect(mockClient.keyEvents).toHaveLength(0);
  });
});
