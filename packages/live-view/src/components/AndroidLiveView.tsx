import React, { useEffect, useRef, useState } from 'react';
import { AndroidLiveViewProps, Stats, Device } from '../types';
import { WebRTCClientRefactored } from '../lib/webrtc-client';
import { H264ClientRefactored } from '../lib/h264-client';
import { DeviceList } from './DeviceList';
import { ControlButtons } from './ControlButtons';
import { 
  useClipboardHandler, 
  useControlHandler, 
  useKeyboardHandler, 
  useWheelHandler, 
  useMouseHandler, 
  useClickHandler,
  useTouchHandler
} from '../hooks';
import { useDeviceManager } from '../hooks/useDeviceManager';
import styles from './AndroidLiveView.module.css';

export const AndroidLiveView: React.FC<AndroidLiveViewProps> = ({
  apiUrl = 'http://localhost:29888/api',
  wsUrl = 'ws://localhost:8080/ws',
  mode = 'h264',
  deviceSerial,
  autoConnect = false,
  showDeviceList = true,
  showAndroidControls = true,
  onConnect,
  onDisconnect,
  onError,
  className,
}) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLDivElement>(null);
  const videoWrapperRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const touchIndicatorRef = useRef<HTMLDivElement>(null);
  
  // Use a polymorphic client ref so we can switch among WebRTC/H264
  const clientRef = useRef<WebRTCClientRefactored | H264ClientRefactored | null>(null);
  
  const [connectionStatus, setConnectionStatus] = useState<string>('');
  const [isConnected, setIsConnected] = useState(false);
  const [stats, setStats] = useState<Stats>({ fps: 0, resolution: '', latency: 0 });
  const [keyboardCaptureEnabled] = useState(true);
  const [currentMode, setCurrentMode] = useState<'webrtc' | 'h264'>(mode as 'webrtc' | 'h264');
  const [userDisconnected, setUserDisconnected] = useState(false);
  const [touchIndicator, setTouchIndicator] = useState<{ visible: boolean; x: number; y: number; dragging: boolean }>({
    visible: false,
    x: 0,
    y: 0,
    dragging: false
  });

  // Use device manager hook
  const { devices, currentDevice, loading, setCurrentDevice, loadDevices } = useDeviceManager({
    apiUrl,
    showDeviceList,
    autoConnect,
    deviceSerial,
    isConnected,
    onError,
  });

  // Use specialized control handlers
  const clipboardHandler = useClipboardHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
    onError,
  });

  const controlHandler = useControlHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  const keyboardHandler = useKeyboardHandler({
    client: clientRef.current,
    enabled: isConnected,
    keyboardCaptureEnabled,
    isConnected,
    onClipboardPaste: clipboardHandler.handleClipboardPaste,
    onClipboardCopy: clipboardHandler.handleClipboardCopy,
  });

  const wheelHandler = useWheelHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  const mouseHandler = useMouseHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  const clickHandler = useClickHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  const touchHandler = useTouchHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  // Video resize handler - centralized and debounced
  const resizeVideo = React.useCallback(() => {
    if (!videoWrapperRef.current) return;

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
    let videoWidth = 1080;
    let videoHeight = 2340;

    if (currentMode === 'webrtc' && videoRef.current) {
      videoWidth = videoRef.current.videoWidth || 1080;
      videoHeight = videoRef.current.videoHeight || 2340;
    } else if (currentMode === 'h264' && canvasRef.current) {
      // Get canvas from the container
      const canvas = canvasRef.current.querySelector('canvas');
      if (canvas && canvas.width > 0 && canvas.height > 0) {
        videoWidth = canvas.width;
        videoHeight = canvas.height;
        console.log('[Video] Using H264 canvas dimensions:', { videoWidth, videoHeight });
      }
    }

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

    // For H264 mode, allow video to fill more of the screen
    if (currentMode === 'h264') {
      // Check if we're in landscape mode (width > height)
      const isLandscape = availableWidth > availableHeight;
      
      if (isLandscape) {
        // In landscape, prioritize filling the width
        newWidth = availableWidth;
        newHeight = availableWidth / aspectRatio;
        
        // If height exceeds available space, scale down proportionally
        if (newHeight > availableHeight) {
          const scale = availableHeight / newHeight;
          newWidth *= scale;
          newHeight *= scale;
        }
      } else {
        // In portrait, prioritize filling the height
        newHeight = availableHeight;
        newWidth = availableHeight * aspectRatio;
        
        // If width exceeds available space, scale down proportionally
        if (newWidth > availableWidth) {
          const scale = availableWidth / newWidth;
          newWidth *= scale;
          newHeight *= scale;
        }
      }
    }

    // Apply the calculated dimensions to the appropriate element
    let targetElement: HTMLElement | null = null;
    if (currentMode === 'webrtc') {
      targetElement = videoRef.current;

    } else if (currentMode === 'h264' && canvasRef.current) {
      // Get canvas from the container
      targetElement = canvasRef.current.querySelector('canvas');
    }
    
    if (targetElement) {
      // Apply calculated dimensions (like old code)
      targetElement.style.width = `${newWidth}px`;
      targetElement.style.height = `${newHeight}px`;
      targetElement.style.objectFit = "contain";
      targetElement.style.display = "block";
      targetElement.style.margin = "auto";
      
      console.log('[Video] Canvas dimensions set (old code style):', {
        canvasWidth: newWidth,
        canvasHeight: newHeight,
        containerWidth: availableWidth,
        containerHeight: availableHeight,
        margin: "auto"
      });
    } else {
      console.warn('[Video] No target element found for mode:', currentMode);
    }

    console.log('[Video] Resized to:', { newWidth, newHeight, availableWidth, availableHeight });
  }, [currentMode]);

  // Debounced resize handler
  const debouncedResizeVideo = React.useCallback(() => {
    clearTimeout((window as any).resizeTimeout);
    (window as any).resizeTimeout = setTimeout(resizeVideo, 100);
  }, [resizeVideo]);

  // Initialize client based on mode
  const initializeClient = React.useCallback(() => {
    if (!containerRef.current) return;

    // Clean up existing client
    if (clientRef.current) {
      clientRef.current.disconnect();
      clientRef.current = null;
    }

    // Create new client based on mode
    if (currentMode === 'webrtc') {
      clientRef.current = new WebRTCClientRefactored(containerRef.current, {
        onConnectionStateChange: (state, message) => {
          setConnectionStatus(message || state);
        setIsConnected(state === 'connected');
          if (state === 'connected') {
            onConnect?.(currentDevice!);
          } else if (state === 'disconnected') {
            onDisconnect?.();
          }
        },
        onError: (error) => {
          console.error('[WebRTCClient] Error:', error);
          onError?.(error);
        },
        onStatsUpdate: (stats) => {
          setStats(stats);
        },
        enableAudio: true,
        audioCodec: 'opus',
      });
    } else {
      clientRef.current = new H264ClientRefactored(containerRef.current, {
        onConnectionStateChange: (state, message) => {
          setConnectionStatus(message || state);
          setIsConnected(state === 'connected');
          if (state === 'connected') {
            onConnect?.(currentDevice!);
        } else if (state === 'disconnected') {
          onDisconnect?.();
        }
      },
        onError: (error) => {
          console.error('[H264Client] Error:', error);
        onError?.(error);
      },
        onStatsUpdate: (stats) => {
          setStats(stats);
        },
        enableAudio: true,
        audioCodec: 'opus',
      });
    }
  }, [currentMode, currentDevice, onConnect, onDisconnect, onError]);

  // Connect to device
  const connectToDevice = React.useCallback(async (device: Device) => {
    // Initialize client if it doesn't exist
    if (!clientRef.current) {
      initializeClient();
    }

    if (!clientRef.current) {
      console.error("[AndroidLiveView] Failed to initialize client");
      return;
    }

    try {
      setConnectionStatus('Connecting...');
      await clientRef.current.connect(device.serial, apiUrl, wsUrl);
      
      // For WebRTC, set up video element when ready
      if (currentMode === 'webrtc' && videoRef.current) {
        const webrtcClient = clientRef.current as WebRTCClientRefactored;
        webrtcClient.setupVideoElementWhenReady();
      }
      
      // For H264, set up canvas element
      if (currentMode === 'h264' && canvasRef.current && clientRef.current) {
        const h264Client = clientRef.current as H264ClientRefactored;
        // Set the correct canvas container
        h264Client.setCanvasContainer(canvasRef.current);
        const canvas = h264Client.getCanvas();
        if (canvas) {
          // Clear the container and add the canvas
          canvasRef.current.innerHTML = '';
          canvasRef.current.appendChild(canvas);
        }
      }
      
      // Resize video after connection
      setTimeout(resizeVideo, 100);
    } catch (error) {
      console.error('[AndroidLiveView] Connection failed:', error);
      onError?.(error as Error);
    }
  }, [apiUrl, wsUrl, currentMode, resizeVideo, onError, initializeClient]);

  // Disconnect from device
  const disconnectFromDevice = React.useCallback(async () => {
    setUserDisconnected(true); // Mark as user-initiated disconnect
    
    if (clientRef.current) {
      await clientRef.current.disconnect();
      clientRef.current = null;
    }
    setIsConnected(false);
    setConnectionStatus('');
    onDisconnect?.();
  }, [onDisconnect]);

  // Handle mode change
  const handleModeChange = React.useCallback((newMode: 'webrtc' | 'h264') => {
    if (newMode !== currentMode) {
      setCurrentMode(newMode);
      
      // Update URL parameter
      const url = new URL(window.location.href);
      url.searchParams.set('mode', newMode);
      window.history.replaceState({}, '', url.toString());
      
      if (isConnected) {
        disconnectFromDevice();
      }
    }
  }, [currentMode, isConnected, disconnectFromDevice]);

  // Handle device selection
  const handleDeviceSelect = React.useCallback(async (device: Device) => {
    // If currently connected, disconnect first
    if (isConnected) {
      await disconnectFromDevice();
    }
    
    // Set new device and reset disconnect flag
    setCurrentDevice(device);
    setUserDisconnected(false); // Reset user disconnect flag when selecting new device
  }, [isConnected, disconnectFromDevice]);

  // Handle control actions
  const handleControlAction = React.useCallback((action: string) => {
    controlHandler.handleControlAction(action);
  }, [controlHandler]);

  // Handle IME switch
  const handleIMESwitch = React.useCallback(() => {
    controlHandler.handleIMESwitch();
  }, [controlHandler]);

  // Touch indicator handlers - calculate position relative to viewport
  const showTouchIndicator = React.useCallback((x: number, y: number, dragging: boolean = false) => {
    setTouchIndicator({ visible: true, x, y, dragging });
  }, []);

  const hideTouchIndicator = React.useCallback(() => {
    setTouchIndicator(prev => ({ ...prev, visible: false }));
  }, []);

  const updateTouchIndicator = React.useCallback((x: number, y: number, dragging: boolean = false) => {
    setTouchIndicator(prev => ({ ...prev, x, y, dragging }));
  }, []);


  // Effects
  useEffect(() => {
    initializeClient();
    return () => {
          if (clientRef.current) {
        clientRef.current.disconnect();
      }
    };
  }, [initializeClient]);

  useEffect(() => {
    if (currentDevice && !isConnected && !userDisconnected) {
      connectToDevice(currentDevice);
    }
  }, [currentDevice, isConnected, userDisconnected, connectToDevice]);

  useEffect(() => {
    window.addEventListener('resize', debouncedResizeVideo);
    return () => {
      window.removeEventListener('resize', debouncedResizeVideo);
    };
  }, [debouncedResizeVideo]);

  // Update control handler client reference
  useEffect(() => {
    // This will be handled by the control handler internally
  }, [clientRef.current]);

  return (
    <div className={`${styles.androidLiveView} ${className || ''}`}>
      {/* Content Wrapper - Sidebar Layout */}
      <div className={styles.contentWrapper}>
        {/* Sidebar - Mode Switcher and Device List */}
        {showDeviceList && (
          <div className={styles.sidebar}>
            {/* Sidebar Header */}
            <div className={styles.sidebarHeader}>
              <div className={styles.sidebarTitle}>Android Live View</div>
            </div>

            {/* Sidebar Content */}
            <div className={styles.sidebarContent}>
              {/* Streaming Mode Section */}
              <div className={styles.modeSwitcher}>
                <div className={styles.modeSwitcherTitle}>Streaming Mode</div>
                <div className={styles.modeButtonGroup}>
                  <button
                    onClick={() => handleModeChange('h264')}
                    className={`${styles.modeBtn} ${currentMode === 'h264' ? styles.active : ''}`}
                  >
                    H264+WebM+MSE
                  </button>
                  <button
                    onClick={() => handleModeChange('webrtc')}
                    className={`${styles.modeBtn} ${currentMode === 'webrtc' ? styles.active : ''}`}
                  >
                    WebRTC
                  </button>
                </div>
              </div>

              {/* Device List Section */}
              <DeviceList
                devices={devices}
                currentDevice={currentDevice}
                connectionStatus={connectionStatus}
                isConnected={isConnected}
                loading={loading}
                onConnect={handleDeviceSelect}
                onDisconnect={disconnectFromDevice}
                onRefresh={loadDevices}
              />
            </div>

            {/* Sidebar Footer - Connection Status */}
            <div className={styles.sidebarFooter}>
              <div className={`${styles.sidebarConnectionStatus} ${
                isConnected ? styles.connected : 
                connectionStatus.includes('Connecting') || connectionStatus.includes('reconnecting') || connectionStatus.includes('Reconnecting') ? styles.connecting :
                connectionStatus.includes('failed') || connectionStatus.includes('Failed') || connectionStatus.includes('error') || connectionStatus.includes('Error') || connectionStatus.includes('disconnected') ? styles.error : ''
              }`}>
                {connectionStatus || (isConnected ? 'Connected successfully' : 'Disconnected')}
              </div>
            </div>
          </div>
        )}

        {/* Main Content - Video Area */}
        <div className={styles.mainContent}>
          <div className={styles.videoContainer}>
            {/* Video Area - Simplified structure */}
            <div className={styles.videoMainArea}>
              <div
                ref={videoWrapperRef}
                className={styles.videoWrapper}
                onKeyDown={keyboardHandler.handleKeyDown}
                onKeyUp={keyboardHandler.handleKeyUp}
                onMouseDown={(e) => {
                  mouseHandler.handleMouseDown(e);
                  // Use clientX/Y directly for fixed positioning
                  showTouchIndicator(e.clientX, e.clientY, false);
                }}
                onMouseUp={(e) => {
                  mouseHandler.handleMouseUp(e);
                  hideTouchIndicator();
                }}
                onMouseMove={(e) => {
                  mouseHandler.handleMouseMove(e);
                  if (touchIndicator.visible) {
                    // Use clientX/Y directly for fixed positioning
                    updateTouchIndicator(e.clientX, e.clientY, true);
                  }
                }}
                onMouseLeave={(e) => {
                  mouseHandler.handleMouseLeave(e);
                  hideTouchIndicator();
                }}
                onTouchStart={(e) => {
                  touchHandler.handleTouchStart(e);
                  const touch = e.touches[0];
                  // Use clientX/Y directly for fixed positioning
                  showTouchIndicator(touch.clientX, touch.clientY, false);
                }}
                onTouchEnd={(e) => {
                  touchHandler.handleTouchEnd(e);
                  hideTouchIndicator();
                }}
                onTouchMove={(e) => {
                  touchHandler.handleTouchMove(e);
                  if (touchIndicator.visible) {
                    const touch = e.touches[0];
                    // Use clientX/Y directly for fixed positioning
                    updateTouchIndicator(touch.clientX, touch.clientY, true);
                  }
                }}
                onClick={clickHandler.handleClick}
                onWheel={wheelHandler.handleWheel as any}
                tabIndex={0}
              >
                {currentMode === 'webrtc' ? (
                  <video
                    ref={videoRef}
                    className={styles.video}
                    autoPlay
                    playsInline
                    controls={false}
                  />
                ) : (
                  <div ref={canvasRef} className={styles.video} />
                )}
                <div ref={containerRef} className={styles.clientContainer} />
              </div>

              {/* Android Control Buttons */}
              {showAndroidControls && (
                <ControlButtons
                  onAction={handleControlAction}
                  onIMESwitch={handleIMESwitch}
                />
              )}
            </div>

            {/* Stats */}
            <div className={styles.statsArea}>
              <div className={styles.stats}>
                <div>Resolution: {stats.resolution || 'N/A'}</div>
                <div>FPS: {stats.fps || 0}</div>
                <div>Latency: {stats.latency || 0}ms</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Touch Indicator */}
      <div
        ref={touchIndicatorRef}
        className={`${styles.touchIndicator} ${touchIndicator.visible ? styles.active : ''} ${touchIndicator.dragging ? styles.dragging : ''}`}
        style={{
          left: touchIndicator.x,
          top: touchIndicator.y,
        }}
      />
    </div>
  );
};

