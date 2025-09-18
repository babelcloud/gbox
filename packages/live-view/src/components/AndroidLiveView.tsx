import React, { useEffect, useRef, useState } from 'react';
import { AndroidLiveViewProps, Stats } from '../types';
import { WebRTCClient } from '../lib/webrtc-client';
import { MSEClient } from '../lib/mse-client';
import { H264Client } from '../lib/h264-client';
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
  apiUrl = 'http://localhost:29888/api',
  wsUrl = 'ws://localhost:8080/ws',
  mode = 'h264',
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
  const videoWrapperRef = useRef<HTMLDivElement>(null);
  // Use a polymorphic client ref so we can switch among WebRTC/Streaming/H264
  const clientRef = useRef<any>(null);
  const touchIndicatorRef = useRef<HTMLDivElement>(null);
  const [connectionStatus, setConnectionStatus] = useState<string>('');
  const [isConnected, setIsConnected] = useState(false);
  const [stats, setStats] = useState<Stats>({ fps: 0, resolution: '', latency: 0 });
  const [keyboardCaptureEnabled] = useState(true);
  const [currentMode, setCurrentMode] = useState<'webrtc' | 'stream' | 'h264'>(mode as 'webrtc' | 'stream' | 'h264');

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
    clientRef: clientRef as any,
    isConnected,
    keyboardCaptureEnabled,
  });

  const { handleKeyDown, handleKeyUp } = useKeyboardHandler({
    clientRef: clientRef as any,
    isConnected,
    keyboardCaptureEnabled,
    onSmartPaste: handleSmartPaste,
    onSmartCopy: handleSmartCopy,
  });

  const { isDragging, touchPosition, handleMouseInteraction, handleTouchInteraction, handleMouseLeave } = useMouseHandler({
    clientRef: clientRef as any,
  });

  const { handleClick } = useClickHandler({
    clientRef: clientRef as any,
    isConnected,
  });

  const { handleControlAction, handleIMESwitch } = useControlHandler({
    clientRef: clientRef as any,
    isConnected,
  });

  // Initialize wheel handler
  useWheelHandler({
    videoRef,
    clientRef,
    isConnected,
  });

  // Video resize handler - centralized and debounced
  const resizeVideo = React.useCallback(() => {
    if (!videoRef.current || !videoWrapperRef.current) return;

    const video = videoRef.current;
    const videoWrapper = videoWrapperRef.current;
    const container = videoWrapper.parentElement; // videoMainArea

    if (!container) return;

    const containerRect = container.getBoundingClientRect();
    const computedStyle = window.getComputedStyle(container);
    const paddingRight = parseInt(computedStyle.paddingRight) || 8;
    const paddingLeft = parseInt(computedStyle.paddingLeft) || 8;
    const paddingTop = parseInt(computedStyle.paddingTop) || 8;
    const paddingBottom = parseInt(computedStyle.paddingBottom) || 8;

    const availableWidth = containerRect.width - paddingLeft - paddingRight;
    const availableHeight = containerRect.height - paddingTop - paddingBottom;

    // Get actual video dimensions, fallback to default mobile aspect ratio
    let videoWidth = video.videoWidth || 1080;
    let videoHeight = video.videoHeight || 2340;

    const aspectRatio = videoWidth / videoHeight;

    // Calculate optimal dimensions
    const widthBasedHeight = availableWidth / aspectRatio;
    const heightBasedWidth = availableHeight * aspectRatio;

    let newWidth, newHeight;

    if (widthBasedHeight <= availableHeight) {
      // Width-constrained
      newWidth = availableWidth;
      newHeight = widthBasedHeight;
    } else {
      // Height-constrained
      newWidth = heightBasedWidth;
      newHeight = availableHeight;
    }

    // Apply dimensions
    video.style.width = `${Math.floor(newWidth)}px`;
    video.style.height = `${Math.floor(newHeight)}px`;
    video.style.maxWidth = '100%';
    video.style.maxHeight = '100%';
    video.style.objectFit = 'contain';

    videoWrapper.style.width = `${Math.floor(newWidth)}px`;
    videoWrapper.style.height = `${Math.floor(newHeight)}px`;
    videoWrapper.style.maxWidth = '100%';
    videoWrapper.style.maxHeight = '100%';

    console.log('[Video] Resized:', {
      dimensions: { width: Math.floor(newWidth), height: Math.floor(newHeight) },
      videoSize: { width: videoWidth, height: videoHeight },
      container: { width: containerRect.width, height: containerRect.height },
      available: { width: availableWidth, height: availableHeight },
      aspectRatio
    });
  }, []);

  // Debounced resize handler for window resize events
  const debouncedResize = React.useMemo(() => {
    let timeoutId: number;
    return () => {
      clearTimeout(timeoutId);
      timeoutId = window.setTimeout(() => {
        resizeVideo();
        // Request keyframe on resize if connected
        if (clientRef.current && isConnected) {
          console.log('[WebRTC] Window resized, requesting keyframe');
          clientRef.current.requestKeyframe();
        }
      }, 100);
    };
  }, [resizeVideo, isConnected]);

  // Window resize listener - always active, independent of connection state
  useEffect(() => {
    window.addEventListener('resize', debouncedResize);
    return () => window.removeEventListener('resize', debouncedResize);
  }, [debouncedResize]);

  // Video event listeners for metadata and resize events
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const handleVideoLoadedMetadata = () => {
      console.log('[Video] Metadata loaded, resizing');
      setTimeout(resizeVideo, 100);
    };

    const handleVideoResize = () => {
      console.log('[Video] Video element resized');
      setTimeout(resizeVideo, 50);
    };

    video.addEventListener('loadedmetadata', handleVideoLoadedMetadata);
    video.addEventListener('resize', handleVideoResize);

    return () => {
      video.removeEventListener('loadedmetadata', handleVideoLoadedMetadata);
      video.removeEventListener('resize', handleVideoResize);
    };
  }, [resizeVideo]);

  // Initial resize and connection state change handler
  useEffect(() => {
    const timer = setTimeout(resizeVideo, isConnected ? 500 : 100);
    return () => clearTimeout(timer);
  }, [resizeVideo, isConnected]);



  // Initialize client based on mode
  useEffect(() => {
    console.log('[AndroidLiveView] Initializing client for mode:', currentMode);
    console.log('[AndroidLiveView] Video ref:', videoRef.current);
    console.log('[AndroidLiveView] showControls:', showControls, 'showAndroidControls:', showAndroidControls);

    if (!videoRef.current) {
      console.error('[AndroidLiveView] Video ref is null, cannot initialize client');
      return;
    }
    
    // Auto-focus video element for keyboard input
    videoRef.current.focus();

    const clientOptions = {
      onConnectionStateChange: (state: "connecting" | "connected" | "disconnected" | "error", message?: string) => {
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
      onError: (error: Error) => {
        console.error(`${currentMode.toUpperCase()} error:`, error);
        onError?.(error);
      },
      onStatsUpdate: (newStats: any) => {
        setStats(prev => ({ ...prev, ...newStats }));
      },
    };

    if (currentMode === 'webrtc') {
      if (videoRef.current) {
        clientRef.current = new WebRTCClient(videoRef.current, clientOptions);
      }
    } else if (currentMode === 'stream') {
      if (videoRef.current) {
        clientRef.current = new MSEClient(videoRef.current, clientOptions);
      }
    } else if (currentMode === 'h264') {
      // h264: use the video element's parent container to host canvas
      // Don't replace the video element, just hide it and add canvas alongside
      console.log('[AndroidLiveView] Setting up H264 client...');
      if (videoRef.current && videoRef.current.parentElement) {
        const parent = videoRef.current.parentElement;
        console.log('[AndroidLiveView] Parent element found:', parent);

        // Clean up any existing H264 container first
        const existingContainer = document.getElementById('h264-container');
        if (existingContainer && existingContainer.parentElement) {
          existingContainer.parentElement.removeChild(existingContainer);
        }

        // Hide the video element for H264 mode
        videoRef.current.style.display = 'none';

        // Create container for H264 canvas
        const h264Container = document.createElement('div');
        h264Container.style.width = '100%';
        h264Container.style.height = '100%';
        h264Container.style.position = 'absolute';
        h264Container.style.top = '0';
        h264Container.style.left = '0';
        h264Container.id = 'h264-container';
        parent.appendChild(h264Container);

        try {
          clientRef.current = new H264Client(h264Container, clientOptions);
          console.log('[AndroidLiveView] H264 client created successfully');
        } catch (error) {
          console.error('[AndroidLiveView] Failed to create H264 client:', error);
        }
      } else {
        console.error('[AndroidLiveView] Video element or parent not found for H264 mode');
      }
    }

    return () => {
      // Clean up client first
      if (clientRef.current) {
        clientRef.current.cleanup();
        clientRef.current = null;
      }
      
      // Cleanup H264 mode: remove H264 container and restore video element
      if (currentMode === 'h264' && videoRef.current) {
        // Show the video element again
        videoRef.current.style.display = '';
        
        // Remove H264 container if it exists
        const h264Container = document.getElementById('h264-container');
        if (h264Container && h264Container.parentElement) {
          h264Container.parentElement.removeChild(h264Container);
        }
      }
    };
  }, [currentMode]);


  // Auto-connect to specified device
  useEffect(() => {
    if (autoConnect && deviceSerial && !isConnected && clientRef.current) {
      handleConnect(deviceSerial);
    }
  }, [autoConnect, deviceSerial]);

  const handleConnect = async (serial: string) => {
    console.log('[AndroidLiveView] handleConnect called with serial:', serial);
    console.log('[AndroidLiveView] Current mode:', currentMode);
    console.log('[AndroidLiveView] Client ref current:', clientRef.current);

    if (!clientRef.current) {
      console.error('[AndroidLiveView] Client not initialized, cannot connect to device:', serial);
      console.error('[AndroidLiveView] Current mode:', currentMode);
      console.error('[AndroidLiveView] Video ref:', videoRef.current);
      return;
    }

    try {
      console.log('[AndroidLiveView] Setting current device to:', serial);
      setCurrentDevice(serial);

      if (currentMode === 'webrtc') {
        // WebRTC mode: connect via WebSocket
        console.log('[AndroidLiveView] Connecting via WebRTC with wsUrl:', wsUrl);
        await (clientRef.current as WebRTCClient).connect(serial, wsUrl);
      } else if (currentMode === 'h264') {
        // H264 mode: connect via HTTP API
        console.log('[AndroidLiveView] Connecting via H264 with apiUrl:', apiUrl);
        await (clientRef.current as H264Client).connect(serial, apiUrl);
      } else {
        // Streaming mode: connect via HTTP API
        console.log('[AndroidLiveView] Connecting via MSE with apiUrl:', apiUrl);
        await (clientRef.current as MSEClient).connect(serial, apiUrl);
      }
      console.log('[AndroidLiveView] Connection attempt completed for:', serial);
    } catch (error) {
      console.error('[AndroidLiveView] Connection failed:', error);
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









  const handleModeSwitch = async (newMode: 'webrtc' | 'stream' | 'h264') => {
    if (isConnected) {
      await handleDisconnect();
      // Wait a bit for cleanup to complete
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
    
    // Clean up existing client
    if (clientRef.current) {
      clientRef.current.cleanup();
      clientRef.current = null;
    }
    
    setCurrentMode(newMode);
    
    // Update URL to reflect current mode
    const url = new URL(window.location.href);
    if (newMode === 'stream') {
      url.searchParams.delete('mode'); // Default mode
    } else {
      url.searchParams.set('mode', newMode);
    }
    window.history.replaceState({}, '', url.toString());
  };

  return (
    <div className={`${styles.container} ${className || ''}`}>
      <div className={styles.contentWrapper}>
        {showDeviceList && (
          <div className={styles.sidebar}>
            {/* Mode Switcher */}
            <div className={styles.modeSwitcher}>
              <div className={styles.modeSwitcherTitle}>Streaming Mode</div>
              <div className={styles.modeButtonGroup}>
                <button
                  className={`${styles.modeBtn} ${currentMode === 'h264' ? styles.active : ''}`}
                  onClick={() => handleModeSwitch('h264')}
                >
                  H.264
                </button>
                <button
                  className={`${styles.modeBtn} ${currentMode === 'stream' ? styles.active : ''}`}
                  onClick={() => handleModeSwitch('stream')}
                >
                  MSE
                </button>
                <button
                  className={`${styles.modeBtn} ${currentMode === 'webrtc' ? styles.active : ''}`}
                  onClick={() => handleModeSwitch('webrtc')}
                >
                  WebRTC
                </button>
              </div>
            </div>
            
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
            <div className={styles.videoContent}>
              <div className={styles.videoMainArea}>
                <div ref={videoWrapperRef} className={styles.videoWrapper}>
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

                  <div
                    ref={touchIndicatorRef}
                    className={`${styles.touchIndicator} ${isDragging ? styles.active + ' ' + styles.dragging : ''}`}
                    style={{
                      left: touchPosition.x,
                      top: touchPosition.y,
                    }}
                  />
                </div>
              </div>

              {showControls && showAndroidControls && (
                <div className={styles.controlsArea}>
                  <ControlButtons
                    onAction={handleControlAction}
                    onIMESwitch={handleIMESwitch}
                    onDisconnect={handleDisconnect}
                    isVisible={true}
                    onToggleVisibility={() => {}}
                    showDisconnect={currentMode === 'h264'}
                  />
                </div>
              )}
            </div>

            {showControls && (
              <div className={styles.statsArea}>
                <div className={styles.stats}>
                  <div>Resolution: {stats.resolution || '-'}</div>
                  <div>FPS: {stats.fps || '-'}</div>
                  <div>Latency: {stats.latency ? `${stats.latency}ms` : '-'}</div>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};