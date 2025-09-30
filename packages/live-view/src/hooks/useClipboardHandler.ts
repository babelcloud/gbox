import { useCallback } from "react";
import { ControlClient } from "../lib/types";

export interface UseClipboardHandlerProps {
  client: ControlClient | null;
  enabled?: boolean;
  isConnected?: boolean;
  onError?: (error: Error) => void;
}

export interface UseClipboardHandlerReturn {
  handleClipboardPaste: () => Promise<void>;
  handleClipboardCopy: () => void;
}

/**
 * Hook for handling clipboard operations
 */
export function useClipboardHandler({
  client,
  enabled = true,
  isConnected = false,
  onError,
}: UseClipboardHandlerProps): UseClipboardHandlerReturn {
  const handleClipboardPaste = useCallback(async () => {
    if (!enabled || !client || !isConnected) return;
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        client.sendClipboardSet(text, true);
      }
    } catch (error) {
      console.error("[ClipboardHandler] Failed to read clipboard:", error);
      onError?.(error as Error);
    }
  }, [enabled, client, isConnected, onError]);

  const handleClipboardCopy = useCallback(() => {
    if (!enabled || !client || !isConnected) return;
    client.sendControlAction("clipboard_get");
    console.log("[ClipboardHandler] Requested clipboard content from device.");
  }, [enabled, client, isConnected]);

  return {
    handleClipboardPaste,
    handleClipboardCopy,
  };
}
