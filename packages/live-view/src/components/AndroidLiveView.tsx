import React, { useEffect, useRef, useState } from 'react';
import { AndroidLiveViewProps, Stats, Device, ConnectionState } from '../types';
import { WebRTCClient } from '../lib/webrtc-client';
import { H264Client } from '../lib/separated-client';
import { MP4Client } from '../lib/muxed-client';
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
  wsUrl = 'ws://localhost:29888',
  mode = 'separated',
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
  
  // Use a polymorphic client ref so we can switch among WebRTC/H264/WebM
  const clientRef = useRef<WebRTCClient | H264Client | MP4Client | null>(null);
  
  const [connectionStatus, setConnectionStatus] = useState<string>('');
  const [isConnected, setIsConnected] = useState(false);
  const [stats, setStats] = useState<Stats>({ fps: 0, resolution: '', latency: 0 });
  const [keyboardCaptureEnabled] = useState(true);
  const [currentMode, setCurrentMode] = useState<'webrtc' | 'separated' | 'muxed'>(mode as 'webrtc' | 'separated' | 'muxed');
  const [userDisconnected, setUserDisconnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
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
    } else if (currentMode === 'separated' && canvasRef.current) {
      // Get canvas from the container
      const canvas = canvasRef.current.querySelector('canvas');
      if (canvas && canvas.width > 0 && canvas.height > 0) {
        videoWidth = canvas.width;
        videoHeight = canvas.height;
        console.log('[Video] Using H264 canvas dimensions:', { videoWidth, videoHeight });
      }
    } else if (currentMode === 'muxed' && containerRef.current) {
      // For MP4 mode, find the video element created by MP4Client
      const videoElement = containerRef.current.querySelector('video');
      if (videoElement && videoElement.videoWidth > 0 && videoElement.videoHeight > 0) {
        videoWidth = videoElement.videoWidth;
        videoHeight = videoElement.videoHeight;
        console.log('[Video] Using MP4 video dimensions:', { videoWidth, videoHeight });
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

    // For H264 mode (separated), allow video to fill more of the screen
    if (currentMode === 'separated') {
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
    } else if (currentMode === 'separated' && canvasRef.current) {
      // Get canvas from the container
      targetElement = canvasRef.current.querySelector('canvas');
    } else if (currentMode === 'muxed') {
      // For MP4 mode, find the video element created by MP4Client
      // Try multiple selectors to find the video element
      const selectors = [
        '#video-mp4-container video',
        '.video-container video',
        'video[src^="blob:"]'
      ];
      
      for (const selector of selectors) {
        targetElement = document.querySelector(selector) as HTMLVideoElement;
        if (targetElement) {
          console.log(`[Video] Found MP4 video element with selector: ${selector}`);
          break;
        }
      }
      
      if (!targetElement) {
        console.log('[Video] MP4 video element not found yet, will retry later');
      }
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
    clearTimeout((window as unknown as { resizeTimeout: number }).resizeTimeout);
    (window as unknown as { resizeTimeout: number }).resizeTimeout = setTimeout(() => {
      resizeVideo();
    }, 100);
  }, []);


  // Connect to device
  const connectToDevice = React.useCallback(async (device: Device, forceReconnect: boolean = false) => {
    console.log('[AndroidLiveView] connectToDevice called:', {
      device: device.serial,
      forceReconnect,
      hasClient: !!clientRef.current,
      isConnecting
    });
    
    if (!clientRef.current) {
      console.log('[AndroidLiveView] No client available, returning');
      return;
    }

    // Prevent multiple simultaneous connections
    if (isConnecting) {
      console.log('[AndroidLiveView] Connection already in progress, skipping');
      return;
    }

    console.log('[AndroidLiveView] Starting connection process');
    setIsConnecting(true);
    try {
      setConnectionStatus('Connecting...');
      
      if (clientRef.current instanceof MP4Client) {
        console.log("=====", device.serial, apiUrl, wsUrl, forceReconnect)
        // MP4Client needs wsUrl for control WebSocket connection
        await clientRef.current.connect(device.serial, apiUrl, wsUrl, forceReconnect);
      } else {
        // Other clients (WebRTC, H264)
        await clientRef.current.connect(device.serial, apiUrl, wsUrl);
      }
      
      // For WebRTC, set up video element when ready
      if (clientRef.current instanceof WebRTCClient && videoRef.current) {
        const webrtcClient = clientRef.current as WebRTCClient;
        webrtcClient.setupVideoElementWhenReady();
      }
      
      // Resize video after connection
      setTimeout(resizeVideo, 100);
    } catch (error) {
      console.error('[AndroidLiveView] Connection failed:', error);
      onError?.(error as Error);
    } finally {
      setIsConnecting(false);
    }
  }, [apiUrl, wsUrl, onError]);

  // Disconnect from device (for mode switching)
  const disconnectFromDevice = React.useCallback(async () => {
    setUserDisconnected(true); // Mark as user-initiated disconnect
    setIsConnecting(false); // Reset connecting state
    
    if (clientRef.current) {
      await clientRef.current.disconnect();
      clientRef.current = null;
    }
    setIsConnected(false);
    setConnectionStatus('');
    onDisconnect?.();
  }, [onDisconnect]);

  // Reset connection (for device switching)
  const resetDeviceConnection = React.useCallback(async () => {
    setIsConnecting(false); // Reset connecting state
    if (clientRef.current && clientRef.current instanceof H264Client) {
      await clientRef.current.resetConnection();
    } else if (clientRef.current) {
      // For other client types, just disconnect but keep the client reference
      // This allows for reconnection in device switching scenarios
      clientRef.current.disconnect();
    }
    setIsConnected(false);
    setConnectionStatus('');
  }, []);

  // Handle mode change
  const handleModeChange = React.useCallback((newMode: 'webrtc' | 'separated' | 'muxed') => {
    if (newMode !== currentMode) {
      console.log(`[AndroidLiveView] Mode changing from ${currentMode} to ${newMode}`);
      
      // Update URL parameter
      const url = new URL(window.location.href);
      url.searchParams.set('mode', newMode);
      window.history.replaceState({}, '', url.toString());
      
      // If we have a connected device, preserve the connection state
      const wasConnected = isConnected;
      const connectedDevice = currentDevice;
      
      // Reset connection state temporarily
      setIsConnected(false);
      setConnectionStatus('');
      
      setCurrentMode(newMode);
      
      // If we had a connected device, reconnect it in the new mode
      if (wasConnected && connectedDevice && !isConnecting) {
        console.log(`[AndroidLiveView] Reconnecting device ${connectedDevice.serial} in ${newMode} mode`);
        setTimeout(() => {
          if (clientRef.current) {
            connectToDevice(connectedDevice, false); // Mode change doesn't need force reconnect
          }
        }, 100);
      }
    }
  }, [currentMode, isConnected, currentDevice]);

  // Handle device selection
  const handleDeviceSelect = React.useCallback(async (device: Device) => {
    console.log(`[AndroidLiveView] Device selection: ${device.serial} (currently connected: ${isConnected})`);
    
    // Check if it's the same device
    const isDifferentDevice = currentDevice && currentDevice.serial !== device.serial;
    
    // If currently connected, reset connection (keep UI elements)
    if (isConnected) {
      await resetDeviceConnection();
    }
    
    // Set new device and reset disconnect flag
    setCurrentDevice(device);
    setUserDisconnected(false); // Reset user disconnect flag when selecting new device
    
    // Connect to the new device immediately
    if (clientRef.current) {
      console.log('[AndroidLiveView] Connecting to new device immediately');
      setTimeout(() => {
        if (clientRef.current) {
          connectToDevice(device, !!isDifferentDevice);
        }
      }, 50);
    }
  }, [isConnected, currentDevice, resetDeviceConnection, connectToDevice]);

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
    console.log('[AndroidLiveView] Initializing client for mode:', currentMode);
    
    // Clean up existing client
    if (clientRef.current) {
      console.log('[AndroidLiveView] Cleaning up existing client before mode change');
      clientRef.current.disconnect();
      clientRef.current = null;
    }

    // Create new client based on mode
    if (currentMode === 'webrtc' && videoRef.current) {
      clientRef.current = new WebRTCClient(videoRef.current, {
        onConnectionStateChange: (state, message) => {
          setConnectionStatus(message || state);
          setIsConnected(state === 'connected');
          if (state === 'connected' && currentDevice) {
            onConnect?.(currentDevice);
          } else if (state === 'disconnected') {
            onDisconnect?.();
          }
        },
        onError: (error) => {
          onError?.(error);
        },
        onStatsUpdate: (stats) => {
          setStats(stats);
        },
        enableAudio: true,
        audioCodec: 'opus',
      });
    } else if (currentMode === 'separated' && canvasRef.current) {
      clientRef.current = new H264Client(canvasRef.current, {
        onConnectionStateChange: (state: string, message?: string) => {
          console.log(`[AndroidLiveView] H264Client connection state change:`, { state, message });
          setConnectionStatus(message || state);
          setIsConnected(state === 'connected');
          console.log(`[AndroidLiveView] isConnected set to:`, state === 'connected');
          if (state === 'connected' && currentDevice) {
            onConnect?.(currentDevice);
          } else if (state === 'disconnected') {
            onDisconnect?.();
          }
        },
        onError: (error: Error) => {
          onError?.(error);
        },
        onStatsUpdate: (stats: Stats) => {
          setStats(stats);
        },
        enableAudio: true,
        audioCodec: 'opus',
      });
    } else if (currentMode === 'muxed' && containerRef.current) {
      clientRef.current = new MP4Client({
        onConnectionStateChange: (state: ConnectionState, message?: string) => {
          setConnectionStatus(message || state);
          setIsConnected(state === 'connected');
          if (state === 'connected' && currentDevice) {
            onConnect?.(currentDevice);
          } else if (state === 'disconnected') {
            onDisconnect?.();
          }
        },
        onError: (error: Error) => {
          onError?.(error);
        },
        onStatsUpdate: (stats: Stats) => {
          setStats(stats);
        },
      });
    }

    return () => {
      if (clientRef.current) {
        clientRef.current.disconnect();
        clientRef.current = null;
      }
    };
  }, [currentMode]);

  // Remove automatic connection useEffect to prevent multiple triggers
  // Connection will be handled manually in handleDeviceSelect and handleModeChange

  // Handle device switching - recreate client if needed
  useEffect(() => {
    if (currentDevice && !isConnected && !userDisconnected && !clientRef.current) {
      console.log('[AndroidLiveView] Device switched, need to recreate client');
      // The client creation will be handled by the mode change useEffect
      // This is just to ensure we don't miss device switches
    }
  }, [currentDevice, isConnected, userDisconnected]);

  useEffect(() => {
    window.addEventListener('resize', debouncedResizeVideo);
    return () => {
      window.removeEventListener('resize', debouncedResizeVideo);
    };
  }, []);

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
                    onClick={() => handleModeChange('separated')}
                    className={`${styles.modeBtn} ${currentMode === 'separated' ? styles.active : ''}`}
                  >
                    Separated
                  </button>
                  <button
                    onClick={() => handleModeChange('muxed')}
                    className={`${styles.modeBtn} ${currentMode === 'muxed' ? styles.active : ''}`}
                  >
                    Muxed
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
                onWheel={wheelHandler.handleWheel as unknown as React.WheelEventHandler<HTMLDivElement>}
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
                ) : currentMode === 'muxed' ? (
                  <div ref={containerRef} id="video-mp4-container" className={styles.clientContainer} />
                ) : (
                  <div ref={canvasRef} className={styles.video} />
                )}
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

