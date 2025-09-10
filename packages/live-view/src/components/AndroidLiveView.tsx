import React, { useEffect, useRef, useState } from 'react';
import { AndroidLiveViewProps, Stats } from '../types';
import { WebRTCClient } from '../lib/webrtc-client';
import { DeviceList } from './DeviceList';
import { ControlButtons } from './ControlButtons';
import {
  useKeyboardHandler,
  useClipboardHandler,
  useMouseHandler,
  useClickHandler,
  useWheelHandler,
  useDeviceManager,
  useControlHandler,
} from '../hooks';
import styles from './AndroidLiveView.module.css';

export const AndroidLiveView: React.FC<AndroidLiveViewProps> = ({
  apiUrl = '/api',
  wsUrl = 'ws://localhost:8080/ws',
  deviceSerial,
  autoConnect = false,
  showControls = true,
  showDeviceList = true,
  showAndroidControls = true,
  onConnect,
  onDisconnect,
  onError,
  className,
}) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const clientRef = useRef<WebRTCClient | null>(null);
  const touchIndicatorRef = useRef<HTMLDivElement>(null);
  const [connectionStatus, setConnectionStatus] = useState<string>('');
  const [isConnected, setIsConnected] = useState(false);
  const [stats, setStats] = useState<Stats>({ fps: 0, resolution: '', latency: 0 });
  const [keyboardCaptureEnabled] = useState(true);

  // Use custom hooks for different functionalities
  const { devices, currentDevice, loading, setCurrentDevice, loadDevices } = useDeviceManager({
    apiUrl,
    showDeviceList,
    autoConnect,
    deviceSerial,
    isConnected,
    onError,
  });

  const { handleSmartPaste, handleSmartCopy } = useClipboardHandler({
    clientRef,
    isConnected,
    keyboardCaptureEnabled,
  });

  const { handleKeyDown, handleKeyUp } = useKeyboardHandler({
    clientRef,
    isConnected,
    keyboardCaptureEnabled,
    onSmartPaste: handleSmartPaste,
    onSmartCopy: handleSmartCopy,
  });

  const { isDragging, touchPosition, handleMouseInteraction, handleTouchInteraction, handleMouseLeave } = useMouseHandler({
    clientRef,
  });

  const { handleClick } = useClickHandler({
    clientRef,
    isConnected,
  });

  const { handleControlAction, handleIMESwitch } = useControlHandler({
    clientRef,
    isConnected,
  });

  // Initialize wheel handler
  useWheelHandler({
    videoRef,
    clientRef,
    isConnected,
  });

  // Handle window resize for keyframe requests
  useEffect(() => {
    const handleResize = () => {
      if (clientRef.current && isConnected) {
        console.log('[WebRTC] Window resized, requesting keyframe');
        clientRef.current.requestKeyframe();
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [isConnected]);


  // Initialize WebRTC client
  useEffect(() => {
    if (!videoRef.current) return;
    
    // Auto-focus video element for keyboard input
    videoRef.current.focus();

    clientRef.current = new WebRTCClient(videoRef.current, {
      onConnectionStateChange: (state, message) => {
        setConnectionStatus(message || '');
        setIsConnected(state === 'connected');
        
        if (state === 'connected' && currentDevice) {
          const device = devices.find(d => d.serial === currentDevice);
          if (device) onConnect?.(device);
          // Auto-focus video element when connected for keyboard input
          if (videoRef.current) {
            videoRef.current.focus();
            console.log('[Keyboard] Video element auto-focused after connection');
          }
        } else if (state === 'disconnected') {
          onDisconnect?.();
        }
      },
      onError: (error) => {
        console.error('WebRTC error:', error);
        onError?.(error);
      },
      onStatsUpdate: (newStats) => {
        setStats(prev => ({ ...prev, ...newStats }));
      },
    });

    return () => {
      clientRef.current?.cleanup();
    };
  }, []);


  // Auto-connect to specified device
  useEffect(() => {
    if (autoConnect && deviceSerial && !isConnected && clientRef.current) {
      handleConnect(deviceSerial);
    }
  }, [autoConnect, deviceSerial]);

  const handleConnect = async (serial: string) => {
    if (!clientRef.current) return;
    
    try {
      // Directly connect via WebSocket (no need for API pre-connection)
      setCurrentDevice(serial);
      await clientRef.current.connect(serial, wsUrl);
    } catch (error) {
      console.error('Connection failed:', error);
      onError?.(error as Error);
    }
  };

  const handleDisconnect = async () => {
    if (!clientRef.current || !currentDevice) return;

    try {
      await clientRef.current.disconnect();
      setCurrentDevice(null);
    } catch (error) {
      console.error('Disconnect failed:', error);
    }
  };









  return (
    <div className={`${styles.container} ${className || ''}`}>
      {showDeviceList && (
        <div className={styles.sidebar}>
          <DeviceList
            devices={devices}
            currentDevice={currentDevice}
            connectionStatus={connectionStatus}
            isConnected={isConnected}
            loading={loading}
            onConnect={handleConnect}
            onDisconnect={handleDisconnect}
            onRefresh={loadDevices}
          />
        </div>
      )}
      
      <div className={styles.mainContent}>
        <div className={styles.videoContainer}>
          <div className={styles.videoWrapper}>
            <video
              ref={videoRef}
              className={`${styles.video} ${isDragging ? styles.dragging : ''}`}
              autoPlay
              playsInline
              onMouseDown={handleMouseInteraction}
              onMouseUp={handleMouseInteraction}
              onMouseMove={handleMouseInteraction}
              onMouseLeave={handleMouseLeave}
              onTouchStart={handleTouchInteraction}
              onTouchEnd={handleTouchInteraction}
              onTouchMove={handleTouchInteraction}
              onKeyDown={handleKeyDown}
              onKeyUp={handleKeyUp}
              onContextMenu={(e) => e.preventDefault()}
              onClick={(e) => {
                // Ensure video element gets focus for keyboard events
                e.currentTarget.focus();
                console.log('[Keyboard] Video element focused for keyboard input');
                
                // Handle click for text selection (single, double, triple)
                handleClick(e);
              }}
              style={{ touchAction: 'none', outline: 'none' }}
              tabIndex={0}
            />
            {showControls && showAndroidControls && isConnected && (
              <ControlButtons 
                onAction={handleControlAction} 
                onIMESwitch={handleIMESwitch}
              />
            )}
          </div>
          
          <div
            ref={touchIndicatorRef}
            className={`${styles.touchIndicator} ${isDragging ? styles.active + ' ' + styles.dragging : ''}`}
            style={{
              left: touchPosition.x,
              top: touchPosition.y,
            }}
          />
          
          {showControls && (
            <div className={styles.stats}>
              <div>Resolution: {stats.resolution || '-'}</div>
              <div>FPS: {stats.fps || '-'}</div>
              <div>Latency: {stats.latency ? `${stats.latency}ms` : '-'}</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};