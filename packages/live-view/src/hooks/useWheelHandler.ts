import { useEffect, useRef } from 'react';
import { WebRTCClient } from '../lib/webrtc-client';

interface UseWheelHandlerProps {
  videoRef: React.RefObject<HTMLVideoElement>;
  clientRef: React.RefObject<WebRTCClient | null>;
  isConnected: boolean;
}

export const useWheelHandler = ({ videoRef, clientRef, isConnected }: UseWheelHandlerProps) => {
  // Handle wheel events with non-passive listener to allow preventDefault
  useEffect(() => {
    const videoElement = videoRef.current;
    if (!videoElement) return;

    const handleWheel = (e: WheelEvent) => {
      if (!clientRef.current || !isConnected) return;
      
      e.preventDefault();
      e.stopPropagation();
      
      // Send scroll event (exactly like scrcpy does)
      // scrcpy uses event->preciseX/preciseY directly without throttling
      const rect = videoElement.getBoundingClientRect();
      const x = (e.clientX - rect.left) / rect.width;
      const y = (e.clientY - rect.top) / rect.height;
      
      // Use precise values like scrcpy (event->preciseX/preciseY)
      // Invert for Android (negative values scroll up)
      // scrcpy accepts values in range [-16, 16]
      let hScroll = -e.deltaX;
      let vScroll = -e.deltaY;
      
      // Apply scrcpy-like scaling for better responsiveness
      // scrcpy uses raw values but we need to scale for web compatibility
      const scaleFactor = 0.5; // Scale down for web compatibility
      hScroll *= scaleFactor;
      vScroll *= scaleFactor;
      
      // Clamp to scrcpy's acceptable range (like scrcpy does)
      hScroll = Math.max(-16, Math.min(16, hScroll));
      vScroll = Math.max(-16, Math.min(16, vScroll));
      
      console.log('[Wheel] Raw scroll event:', { 
        deltaX: e.deltaX, 
        deltaY: e.deltaY, 
        hScroll, 
        vScroll,
        x, 
        y,
        willSend: (hScroll !== 0 || vScroll !== 0) && (x >= 0 && x <= 1 && y >= 0 && y <= 1)
      });
      
      // Only send if there's actual scroll movement
      if (hScroll !== 0 || vScroll !== 0) {
        // Ensure coordinates are valid
        if (x >= 0 && x <= 1 && y >= 0 && y <= 1) {
          console.log('[Wheel] Sending scroll event:', { x, y, hScroll, vScroll });
          
          clientRef.current.sendControlMessage({
            type: "scroll",
            x,
            y,
            hScroll,
            vScroll,
            timestamp: Date.now(),
          });
        } else {
          console.warn('[Wheel] Invalid coordinates:', { x, y });
        }
      }
    };

    // Add non-passive wheel event listener
    videoElement.addEventListener('wheel', handleWheel, { passive: false });
    
    return () => {
      videoElement.removeEventListener('wheel', handleWheel);
    };
  }, [isConnected, videoRef, clientRef]);
};
