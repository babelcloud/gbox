import { useCallback } from "react";
import { ControlClient } from "../lib/types";

export interface UseControlHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  isConnected?: boolean;
}

export interface UseControlHandlerReturn {
  handleControlAction: (action: string) => void;
  handleIMESwitch: () => void;
}

/**
 * Hook for handling control actions (power, volume, back, home, etc.)
 */
export function useControlHandler({
  client,
  enabled = true,
  isConnected = false,
}: UseControlHandlerProps): UseControlHandlerReturn {
  const handleControlAction = useCallback(
    (action: string) => {
      if (!enabled || !client || !isConnected) return;

      const keycode = getAndroidKeycode(action);
      console.log(
        `[ControlHandler] handleControlAction: action=${action}, keycode=${keycode}`
      );
      if (keycode) {
        client.sendKeyEvent(keycode, "down");
        setTimeout(() => {
          client?.sendKeyEvent(keycode, "up");
        }, 100);
      } else {
        console.warn(
          `[ControlHandler] No keycode found for action: ${action}`
        );
      }
    },
    [enabled, client, isConnected]
  );

  const handleIMESwitch = useCallback(() => {
    if (!enabled || !client || !isConnected) return;
    client.sendKeyEvent(204, "down"); // KEYCODE_LANGUAGE_SWITCH
    setTimeout(() => {
      client?.sendKeyEvent(204, "up");
    }, 50);
  }, [enabled, client, isConnected]);

  return {
    handleControlAction,
    handleIMESwitch,
  };
}

// Helper function
function getAndroidKeycode(action: string): number | null {
  const keycodeMap: { [action: string]: number } = {
    power: 26,
    volume_up: 24,
    volume_down: 25,
    back: 4,
    home: 3,
    app_switch: 187,
    menu: 82,
  };

  return keycodeMap[action.toLowerCase()] || null;
}
