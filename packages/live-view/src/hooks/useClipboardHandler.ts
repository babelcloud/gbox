import { useCallback } from 'react';
import { WebRTCClient } from '../lib/webrtc-client';

interface UseClipboardHandlerProps {
  clientRef: React.RefObject<WebRTCClient | null>;
  isConnected: boolean;
  keyboardCaptureEnabled: boolean;
}

export const useClipboardHandler = ({
  clientRef,
  isConnected,
  keyboardCaptureEnabled,
}: UseClipboardHandlerProps) => {
  // Smart clipboard sync - paste to device
  const handleSmartPaste = useCallback(async () => {
    console.log('[Clipboard] handleSmartPaste called, clientRef:', !!clientRef.current, 'isConnected:', isConnected, 'keyboardCaptureEnabled:', keyboardCaptureEnabled);
    
    if (!clientRef.current || !isConnected || !keyboardCaptureEnabled) {
      console.log('[Clipboard] handleSmartPaste early return');
      return;
    }
    
    try {
      // Get clipboard content from host
      console.log('[Clipboard] Reading clipboard content...');
      const text = await navigator.clipboard.readText();
      console.log('[Clipboard] Clipboard content:', text);
      
      if (text) {
        // Limit clipboard content length to prevent OOM issues
        const maxLength = 10000; // 10KB limit
        const truncatedText = text.length > maxLength ? text.substring(0, maxLength) + '...' : text;
        
        console.log('[Clipboard] Smart paste to device:', truncatedText);
        console.log('[Clipboard] Original length:', text.length, 'Truncated length:', truncatedText.length);
        
        // Send set clipboard command with paste flag
        // Format: [Sequence (8 bytes)][Paste flag (1 byte)][Text length (4 bytes)][Text data]
        // Note: Type is handled by sendControlMessage type parameter, not in buffer
        const textBytes = new TextEncoder().encode(truncatedText);
        const textLength = textBytes.length;
        const buffer = new Uint8Array(8 + 1 + 4 + textLength);
        let offset = 0;
        
        // Sequence (8 bytes, big endian) - use 0 for now
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        buffer[offset++] = 0;
        
        // Paste flag (1 byte) - 1 for set and paste
        buffer[offset++] = 1;
        
        // Text length (4 bytes, big endian) - use actual text length
        buffer[offset++] = (textLength >> 24) & 0xFF;
        buffer[offset++] = (textLength >> 16) & 0xFF;
        buffer[offset++] = (textLength >> 8) & 0xFF;
        buffer[offset++] = textLength & 0xFF;
        
        // Text data
        buffer.set(textBytes, offset);
        
        // Debug: verify buffer size matches expected size
        const expectedSize = 8 + 1 + 4 + textLength;
        if (buffer.length !== expectedSize) {
          console.error(`ERROR: Buffer size mismatch! Expected: ${expectedSize}, Actual: ${buffer.length}`);
        }
        
        clientRef.current.sendControlMessage({
          type: 9, // TYPE_SET_CLIPBOARD
          data: buffer
        });
      }
    } catch (error) {
      console.error('[Clipboard] Failed to read clipboard:', error);
    }
  }, [isConnected, keyboardCaptureEnabled, clientRef]);

  // Smart clipboard sync - copy from device
  const handleSmartCopy = useCallback(() => {
    if (!clientRef.current || !isConnected || !keyboardCaptureEnabled) return;
    
    console.log('[Clipboard] Smart copy from device');
    // Send get clipboard command
    clientRef.current.sendControlMessage({
      type: 8, // TYPE_GET_CLIPBOARD
      data: new Uint8Array(0)
    });
  }, [isConnected, keyboardCaptureEnabled, clientRef]);

  return {
    handleSmartPaste,
    handleSmartCopy,
  };
};
