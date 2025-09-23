import { useCallback } from "react";
import { ControlClient } from "../lib/types";

export interface UseTouchHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  isConnected?: boolean;
}

export interface UseTouchHandlerReturn {
  handleTouchStart: (e: React.TouchEvent) => void;
  handleTouchEnd: (e: React.TouchEvent) => void;
  handleTouchMove: (e: React.TouchEvent) => void;
}

/**
 * Hook for handling touch events
 */
export function useTouchHandler({
  client,
  enabled = true,
  isConnected = false,
}: UseTouchHandlerProps): UseTouchHandlerReturn {
  const handleTouchStart = useCallback(
    (e: React.TouchEvent) => {
      if (!enabled || !client || !isConnected) return;
      client.handleTouchEvent(e.nativeEvent, "down");
    },
    [enabled, client, isConnected]
  );

  const handleTouchEnd = useCallback(
    (e: React.TouchEvent) => {
      if (!enabled || !client || !isConnected) return;
      client.handleTouchEvent(e.nativeEvent, "up");
    },
    [enabled, client, isConnected]
  );

  const handleTouchMove = useCallback(
    (e: React.TouchEvent) => {
      if (!enabled || !client || !isConnected) return;
      client.handleTouchEvent(e.nativeEvent, "move");
    },
    [enabled, client, isConnected]
  );

  return {
    handleTouchStart,
    handleTouchEnd,
    handleTouchMove,
  };
}
