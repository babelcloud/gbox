import { useCallback, useState } from 'react';
import { WebRTCClient } from '../lib/webrtc-client';

interface UseClickHandlerProps {
  clientRef: React.RefObject<WebRTCClient | null>;
  isConnected: boolean;
}

export const useClickHandler = ({ clientRef, isConnected }: UseClickHandlerProps) => {
  // Click detection for text selection
  const [lastClickTime, setLastClickTime] = useState(0);
  const [lastClickPosition, setLastClickPosition] = useState({ x: 0, y: 0 });
  const [clickCount, setClickCount] = useState(0);

  // Handle click for text selection (single, double, triple)
  const handleClick = useCallback((e: React.MouseEvent) => {
    if (!clientRef.current || !isConnected) return;
    
    const currentTime = Date.now();
    const currentPosition = { x: e.clientX, y: e.clientY };
    
    // Check if this is a continuation of previous clicks (within 500ms and similar position)
    const timeDiff = currentTime - lastClickTime;
    const positionDiff = Math.sqrt(
      Math.pow(currentPosition.x - lastClickPosition.x, 2) + 
      Math.pow(currentPosition.y - lastClickPosition.y, 2)
    );
    
    if (timeDiff < 500 && positionDiff < 50) {
      // This is a continuation click
      const newClickCount = clickCount + 1;
      setClickCount(newClickCount);
      
      if (newClickCount === 2) {
        // Double click - select word
        console.log('[Click] Double click - select word');
        // Android double tap to select word (no special key combination needed)
        // The system will handle word selection automatically
      } else if (newClickCount === 3) {
        // Triple click - select line/paragraph
        console.log('[Click] Triple click - select line');
        // Send Ctrl+A for select all (closest to line selection)
        const META_CTRL_ON = 0x1000;
        clientRef.current.sendKeyEvent(113, 'down', META_CTRL_ON);
        setTimeout(() => {
          if (clientRef.current) {
            clientRef.current.sendKeyEvent(29, 'down', META_CTRL_ON);
            setTimeout(() => {
              if (clientRef.current) {
                clientRef.current.sendKeyEvent(29, 'up', META_CTRL_ON);
                setTimeout(() => {
                  if (clientRef.current) {
                    clientRef.current.sendKeyEvent(113, 'up', 0);
                  }
                }, 10);
              }
            }, 10);
          }
        }, 10);
        
        // Reset after triple click
        setClickCount(0);
        setLastClickTime(0);
        setLastClickPosition({ x: 0, y: 0 });
      }
    } else {
      // New click sequence
      setClickCount(1);
      setLastClickTime(currentTime);
      setLastClickPosition(currentPosition);
      
      // Single click - just position cursor (handled by normal touch event)
      console.log('[Click] Single click - position cursor');
    }
  }, [clientRef, isConnected, lastClickTime, lastClickPosition, clickCount]);

  return {
    handleClick,
  };
};
