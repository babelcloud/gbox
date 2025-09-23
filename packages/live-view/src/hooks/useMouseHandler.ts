import { useCallback, useState } from "react";
import { ControlClient } from "../lib/types";

export interface UseMouseHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  isConnected?: boolean;
}

export interface UseMouseHandlerReturn {
  handleMouseDown: (e: React.MouseEvent) => void;
  handleMouseUp: (e: React.MouseEvent) => void;
  handleMouseMove: (e: React.MouseEvent) => void;
  handleMouseLeave: (e: React.MouseEvent) => void;
  isMouseDragging: boolean;
}

/**
 * Hook for handling mouse events
 */
export function useMouseHandler({
  client,
  enabled = true,
  isConnected = false,
}: UseMouseHandlerProps): UseMouseHandlerReturn {
  const [isMouseDragging, setIsMouseDragging] = useState(false);

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      if (!enabled || !client || !isConnected) return;
      setIsMouseDragging(true);
      client.handleMouseEvent(e.nativeEvent, "down");
    },
    [enabled, client, isConnected]
  );

  const handleMouseUp = useCallback(
    (e: React.MouseEvent) => {
      if (!enabled || !client || !isConnected) return;
      setIsMouseDragging(false);
      client.handleMouseEvent(e.nativeEvent, "up");
    },
    [enabled, client, isConnected]
  );

  const handleMouseMove = useCallback(
    (e: React.MouseEvent) => {
      if (!enabled || !client || !isConnected) return;
      if (isMouseDragging) {
        client.handleMouseEvent(e.nativeEvent, "move");
      }
    },
    [enabled, client, isConnected, isMouseDragging]
  );

  const handleMouseLeave = useCallback(
    (e: React.MouseEvent) => {
      if (!enabled || !client || !isConnected) return;
      if (isMouseDragging) {
        client.handleMouseEvent(e.nativeEvent, "up");
        setIsMouseDragging(false);
      }
    },
    [enabled, client, isConnected, isMouseDragging]
  );

  return {
    handleMouseDown,
    handleMouseUp,
    handleMouseMove,
    handleMouseLeave,
    isMouseDragging,
  };
}
