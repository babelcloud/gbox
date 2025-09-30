import { useCallback, useRef } from "react";
import { ControlClient } from "../lib/types";

export interface UseClickHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  isConnected?: boolean;
}

export interface UseClickHandlerReturn {
  handleClick: (e: React.MouseEvent) => void;
}

/**
 * Hook for handling click events (single, double, triple click)
 */
export function useClickHandler({
  client,
  enabled = true,
  isConnected = false,
}: UseClickHandlerProps): UseClickHandlerReturn {
  // Refs for tracking click state
  const lastClickTimeRef = useRef<number>(0);
  const lastClickPositionRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });
  const clickCountRef = useRef<number>(0);

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      if (!enabled || !client || !isConnected) return;

      const currentTime = Date.now();
      const currentPosition = { x: e.clientX, y: e.clientY };

      const timeDiff = currentTime - lastClickTimeRef.current;
      const positionDiff = Math.sqrt(
        Math.pow(currentPosition.x - lastClickPositionRef.current.x, 2) +
          Math.pow(currentPosition.y - lastClickPositionRef.current.y, 2)
      );

      if (timeDiff < 500 && positionDiff < 50) {
        clickCountRef.current++;
        if (clickCountRef.current === 2) {
          // Double click - select word (Android handles this automatically with a double tap)
          console.log("[ClickHandler] Double click - select word");
        } else if (clickCountRef.current === 3) {
          // Triple click - select line/paragraph (simulate Ctrl+A)
          console.log("[ClickHandler] Triple click - select line");
          const META_CTRL_ON = 0x1000;
          client.sendKeyEvent(113, "down", META_CTRL_ON); // Ctrl down
          setTimeout(() => {
            client?.sendKeyEvent(29, "down", META_CTRL_ON); // A down
            setTimeout(() => {
              client?.sendKeyEvent(29, "up", META_CTRL_ON); // A up
              setTimeout(() => {
                client?.sendKeyEvent(113, "up", 0); // Ctrl up
              }, 10);
            }, 10);
          }, 10);
        }
      } else {
        clickCountRef.current = 1;
        console.log("[ClickHandler] Single click - position cursor");
      }

      // Reset counters after processing
      if (clickCountRef.current >= 3) {
        clickCountRef.current = 0;
        lastClickTimeRef.current = 0;
        lastClickPositionRef.current = { x: 0, y: 0 };
      }

      lastClickTimeRef.current = currentTime;
      lastClickPositionRef.current = currentPosition;
    },
    [enabled, client, isConnected]
  );

  return {
    handleClick,
  };
}
