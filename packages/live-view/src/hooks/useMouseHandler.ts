import { useCallback, useState } from 'react';
import { WebRTCClient } from '../lib/webrtc-client';

interface UseMouseHandlerProps {
  clientRef: React.RefObject<WebRTCClient | null>;
}

export const useMouseHandler = ({ clientRef }: UseMouseHandlerProps) => {
  const [isDragging, setIsDragging] = useState(false);
  const [touchPosition, setTouchPosition] = useState({ x: 0, y: 0 });

  const handleMouseInteraction = useCallback((e: React.MouseEvent) => {
    if (!clientRef.current) return;

    const action = e.type === 'mousedown' ? 'down' :
                   e.type === 'mouseup' ? 'up' : 'move';
    
    // Handle mouse event in WebRTC client
    clientRef.current.handleMouseEvent(e.nativeEvent, action);
    
    // Update dragging state and touch indicator position
    if (action === 'down') {
      setIsDragging(true);
      setTouchPosition({ x: e.clientX, y: e.clientY });
    } else if (action === 'up') {
      setIsDragging(false);
      // Hide indicator immediately
      setTouchPosition({ x: -100, y: -100 });
    } else if (action === 'move' && clientRef.current.isMouseDragging) {
      // Update position immediately during drag
      setTouchPosition({ x: e.clientX, y: e.clientY });
    }
  }, [clientRef]);

  const handleTouchInteraction = useCallback((e: React.TouchEvent) => {
    if (!clientRef.current) return;

    const action = e.type === 'touchstart' ? 'down' :
                   e.type === 'touchend' ? 'up' : 'move';
    
    clientRef.current.handleTouchEvent(e.nativeEvent, action);
  }, [clientRef]);

  const handleMouseLeave = useCallback((e: React.MouseEvent) => {
    // Release drag if mouse leaves the video element
    if (clientRef.current && clientRef.current.isMouseDragging) {
      clientRef.current.handleMouseEvent(e.nativeEvent, 'up');
      setIsDragging(false);
      setTouchPosition({ x: -100, y: -100 });
    }
  }, [clientRef]);

  return {
    isDragging,
    touchPosition,
    handleMouseInteraction,
    handleTouchInteraction,
    handleMouseLeave,
  };
};
