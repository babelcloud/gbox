import { useCallback } from 'react';
import { WebRTCClient } from '../lib/webrtc-client';

interface UseControlHandlerProps {
  clientRef: React.RefObject<WebRTCClient | null>;
  isConnected: boolean;
}

export const useControlHandler = ({ clientRef, isConnected }: UseControlHandlerProps) => {
  const handleControlAction = useCallback((action: string) => {
    if (!clientRef.current) return;

    const keycodes = WebRTCClient.ANDROID_KEYCODES as any;
    const keycode = keycodes[action.toUpperCase()];
    
    if (keycode) {
      clientRef.current.sendKeyEvent(keycode, 'down');
      setTimeout(() => {
        clientRef.current?.sendKeyEvent(keycode, 'up');
      }, 100);
    }
  }, [clientRef]);

  // Handle IME switch button click
  const handleIMESwitch = useCallback(() => {
    if (!clientRef.current || !isConnected) return;
    
    console.log('[IME] Switching input method');
    // Send language switch keycode (204)
    clientRef.current.sendKeyEvent(204, 'down');
    setTimeout(() => {
      clientRef.current?.sendKeyEvent(204, 'up');
    }, 50);
  }, [isConnected, clientRef]);

  return {
    handleControlAction,
    handleIMESwitch,
  };
};
