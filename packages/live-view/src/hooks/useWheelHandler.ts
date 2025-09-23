import { useCallback } from "react";
import { ControlClient } from "../lib/types";

export interface UseWheelHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  isConnected?: boolean;
}

export interface UseWheelHandlerReturn {
  handleWheel: (e: WheelEvent) => void;
}

/**
 * Hook for handling wheel/scroll events
 */
export function useWheelHandler({
  client,
  enabled = true,
  isConnected = false,
}: UseWheelHandlerProps): UseWheelHandlerReturn {
  const handleWheel = useCallback(
    (e: WheelEvent) => {
      if (!enabled || !client || !isConnected) return;

      e.preventDefault();
      e.stopPropagation();

      const targetElement = e.target as HTMLElement;
      if (!targetElement) return;

      const rect = targetElement.getBoundingClientRect();
      const x = (e.clientX - rect.left) / rect.width;
      const y = (e.clientY - rect.top) / rect.height;

      let hScroll = -e.deltaX;
      let vScroll = -e.deltaY;

      const scaleFactor = 0.5;
      hScroll *= scaleFactor;
      vScroll *= scaleFactor;

      hScroll = Math.max(-16, Math.min(16, hScroll));
      vScroll = Math.max(-16, Math.min(16, vScroll));

      if (hScroll !== 0 || vScroll !== 0) {
        if (x >= 0 && x <= 1 && y >= 0 && y <= 1) {
          client.sendControlAction("scroll", {
            x,
            y,
            hScroll,
            vScroll,
            timestamp: Date.now(),
          });
        }
      }
    },
    [enabled, client, isConnected]
  );

  return {
    handleWheel,
  };
}
