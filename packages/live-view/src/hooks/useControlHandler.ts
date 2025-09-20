import { useCallback } from "react";
import { WebRTCClient } from "../lib/webrtc-client";
import { H264Client } from "../lib/h264-client";

interface UseControlHandlerProps {
  clientRef: React.RefObject<WebRTCClient | H264Client | null>;
  isConnected: boolean;
}

export const useControlHandler = ({
  clientRef,
  isConnected,
}: UseControlHandlerProps) => {
  const handleControlAction = useCallback(
    (action: string) => {
      if (!clientRef.current) return;

      // 支持两种客户端的键盘码
      const keycodes =
        (clientRef.current as any).constructor.ANDROID_KEYCODES ||
        WebRTCClient.ANDROID_KEYCODES;
      const keycode = keycodes[action.toUpperCase()];

      if (keycode) {
        clientRef.current.sendKeyEvent(keycode, "down");
        setTimeout(() => {
          clientRef.current?.sendKeyEvent(keycode, "up");
        }, 100);
      }
    },
    [clientRef]
  );

  // Handle IME switch button click
  const handleIMESwitch = useCallback(() => {
    if (!clientRef.current || !isConnected) return;

    console.log("[IME] Switching input method");
    // Send language switch keycode (204)
    clientRef.current.sendKeyEvent(204, "down");
    setTimeout(() => {
      clientRef.current?.sendKeyEvent(204, "up");
    }, 50);
  }, [isConnected, clientRef]);

  return {
    handleControlAction,
    handleIMESwitch,
  };
};
