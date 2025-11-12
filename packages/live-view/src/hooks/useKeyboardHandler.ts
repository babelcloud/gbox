import { useCallback } from "react";
import { ControlClient } from "../lib/types";

export interface UseKeyboardHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  keyboardCaptureEnabled?: boolean;
  isConnected?: boolean;
  onClipboardPaste?: () => Promise<void>;
  onClipboardCopy?: () => void;
}

export interface UseKeyboardHandlerReturn {
  handleKeyDown: (e: React.KeyboardEvent) => void;
  handleKeyUp: (e: React.KeyboardEvent) => void;
}

/**
 * Hook for handling keyboard events
 */
export function useKeyboardHandler({
  client,
  enabled = true,
  keyboardCaptureEnabled = true,
  isConnected = false,
  onClipboardPaste,
  onClipboardCopy,
}: UseKeyboardHandlerProps): UseKeyboardHandlerReturn {
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!enabled || !client || !keyboardCaptureEnabled || !isConnected) return;

      const isMac = navigator.platform.toUpperCase().indexOf("MAC") >= 0;
      const isCtrlOrCmd = isMac ? e.metaKey : e.ctrlKey;

      // Smart clipboard paste (Cmd/Ctrl+V)
      if (isCtrlOrCmd && e.key.toLowerCase() === "v") {
        e.preventDefault();
        e.stopPropagation();
        onClipboardPaste?.();
        return;
      }

      // Smart clipboard copy (Cmd/Ctrl+C)
      if (isCtrlOrCmd && e.key.toLowerCase() === "c") {
        e.preventDefault();
        e.stopPropagation();
        onClipboardCopy?.();
        return;
      }

      // Map keyboard events to Android keycodes
      const keycode = getKeycodeFromEvent(e);
      if (keycode) {
        e.preventDefault();
        client.sendKeyEvent(keycode, "down", getMetaState(e));
      }
    },
    [enabled, client, keyboardCaptureEnabled, isConnected, onClipboardPaste, onClipboardCopy]
  );

  const handleKeyUp = useCallback(
    (e: React.KeyboardEvent) => {
      if (!enabled || !client || !keyboardCaptureEnabled || !isConnected) return;

      // Prevent Cmd/Ctrl keyup from being sent to device when used in combinations
      if (e.key === "Meta" || e.key === "Control") {
        e.preventDefault();
        e.stopPropagation();
        return;
      }

      const keycode = getKeycodeFromEvent(e);
      if (keycode) {
        e.preventDefault();
        client.sendKeyEvent(keycode, "up", getMetaState(e));
      }
    },
    [enabled, client, keyboardCaptureEnabled, isConnected]
  );

  return {
    handleKeyDown,
    handleKeyUp,
  };
}

// Helper functions
function getMetaState(e: React.KeyboardEvent | KeyboardEvent): number {
  let metaState = 0;
  if (e.altKey) metaState |= 0x02; // AMETA_ALT_ON
  if (e.ctrlKey) metaState |= 0x1000; // AMETA_CTRL_ON
  if (e.metaKey) metaState |= 0x08; // AMETA_META_ON (Cmd key on Mac)
  if (e.shiftKey) metaState |= 0x01; // AMETA_SHIFT_ON
  return metaState;
}

function getKeycodeFromEvent(
  e: React.KeyboardEvent | KeyboardEvent
): number | null {
  // Functional keys
  const keyMap: { [code: string]: number } = {
    Enter: 66,
    Backspace: 67,
    Delete: 112,
    Escape: 111,
    Tab: 61,
    Space: 62,
    CapsLock: 115,
    ShiftLeft: 59,
    ShiftRight: 60,
    ControlLeft: 113,
    ControlRight: 114,
    AltLeft: 57,
    AltRight: 58,
    MetaLeft: 117,
    MetaRight: 118,
    ArrowUp: 19,
    ArrowDown: 20,
    ArrowLeft: 21,
    ArrowRight: 22,
    Home: 122,
    End: 123,
    PageUp: 92,
    PageDown: 93,
    Insert: 124,
    Digit0: 7,
    Digit1: 8,
    Digit2: 9,
    Digit3: 10,
    Digit4: 11,
    Digit5: 12,
    Digit6: 13,
    Digit7: 14,
    Digit8: 15,
    Digit9: 16,
    Period: 56,
    Comma: 55,
    Slash: 76,
    Backslash: 73,
    Quote: 75,
    Semicolon: 74,
    BracketLeft: 71,
    BracketRight: 72,
    Minus: 69,
    Equal: 70,
    Backquote: 68,
  };

  // Letters (A-Z)
  for (let i = 0; i < 26; i++) {
    const charCode = "A".charCodeAt(0) + i;
    keyMap[`Key${String.fromCharCode(charCode)}`] = 29 + i;
  }

  // F-keys (F1-F12)
  for (let i = 1; i <= 12; i++) {
    keyMap[`F${i}`] = 131 + i; // Approximate mapping, Android keycodes for F-keys vary
  }

  return keyMap[e.code] || null;
}
