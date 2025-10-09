import { useEffect, useRef, useState } from "react";
import { ConnectionState, Device, Stats } from "../src/types";
import {
  WebRTCClient,
  H264Client,
  MP4Client,
  useKeyboardHandler,
  useClipboardHandler,
  useMouseHandler,
  useTouchHandler,
  useClickHandler,
  useWheelHandler,
  useControlHandler,
} from "../src";
import styles from "../src/components/AndroidLiveView.module.css";
import React from "react";
import { ControlButtons } from "../src/components/ControlButtons";

export interface AndroidLiveviewComponentProps {
  onConnectionStateChange?: (state: ConnectionState, message?: string) => void;
  onError?: (error: Error) => void;
  onStatsUpdate?: (stats: Stats) => void;
  onConnect?: (device: Device) => void;
  onDisconnect?: () => void;
  connectaParams: {
    deviceSerial: string;
    apiUrl: string;
    wsUrl: string;
  };
}

export function AndroidLiveviewComponent(props: AndroidLiveviewComponentProps) {
  const {
    onConnectionStateChange,
    onError,
    onStatsUpdate,
    onConnect,
    onDisconnect,
    connectaParams,
  } = props;

  const clientRef = useRef<WebRTCClient | H264Client | MP4Client | null>(null);
  const videoWrapperRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const touchIndicatorRef = useRef<HTMLDivElement>(null);


  const [connectionStatus, setConnectionStatus] = useState<string>("");
  const [isConnected, setIsConnected] = useState(false);
  const [stats, setStats] = useState<Stats>({
    fps: 0,
    resolution: "",
    latency: 0,
  });
  const [keyboardCaptureEnabled] = useState(true);
  const [touchIndicator, setTouchIndicator] = useState<{
    visible: boolean;
    x: number;
    y: number;
    dragging: boolean;
  }>({
    visible: false,
    x: 0,
    y: 0,
    dragging: false,
  });

  const clipboardHandler = useClipboardHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
    onError,
  });

  const keyboardHandler = useKeyboardHandler({
    client: clientRef.current,
    enabled: isConnected,
    keyboardCaptureEnabled,
    isConnected,
    onClipboardPaste: clipboardHandler.handleClipboardPaste,
    onClipboardCopy: clipboardHandler.handleClipboardCopy,
  });

  const touchHandler = useTouchHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  const wheelHandler = useWheelHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  // const controlHandler = useControlHandler({
  //   client: clientRef.current,
  //   enabled: isConnected,
  //   isConnected,
  // });

  // const handleControlAction = React.useCallback((action: string) => {
  //   controlHandler.handleControlAction(action);
  // }, [controlHandler]);

  const { deviceSerial, apiUrl, wsUrl } = connectaParams;

  useEffect(() => {
    console.log("====");
    clientRef.current = new MP4Client({
      onConnectionStateChange: (state: ConnectionState, message?: string) => {
        console.log("====", state, message);
        setConnectionStatus(message || state);
        setIsConnected(state === "connected");
        if (state === "connected") {
          clientRef.current?.connect(deviceSerial, apiUrl, wsUrl);
        } else if (state === "disconnected") {
          clientRef.current?.disconnect();
        }
      },
      onError: (error: Error) => {
        onError?.(error);
      },
      onStatsUpdate: (stats: Stats) => {
        setStats(stats);
      },
    });

    setTimeout(() => {
      clientRef.current?.connect(deviceSerial, apiUrl, wsUrl);
    }, 1000);
  }, []);

  const mouseHandler = useMouseHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  // Touch indicator handlers - calculate position relative to viewport
  const showTouchIndicator = React.useCallback(
    (x: number, y: number, dragging: boolean = false) => {
      setTouchIndicator({ visible: true, x, y, dragging });
    },
    []
  );

  const hideTouchIndicator = React.useCallback(() => {
    setTouchIndicator((prev) => ({ ...prev, visible: false }));
  }, []);

  const updateTouchIndicator = React.useCallback(
    (x: number, y: number, dragging: boolean = false) => {
      setTouchIndicator((prev) => ({ ...prev, x, y, dragging }));
    },
    []
  );

  const clickHandler = useClickHandler({
    client: clientRef.current,
    enabled: isConnected,
    isConnected,
  });

  // const handleIMESwitch = React.useCallback(() => {
  //   controlHandler.handleIMESwitch();
  // }, [controlHandler])

  return (
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
            onWheel={
              wheelHandler.handleWheel as unknown as React.WheelEventHandler<HTMLDivElement>
            }
            tabIndex={0}
          >
            <div
              ref={containerRef}
              id="video-mp4-container"
              className={styles.clientContainer}
            />
          </div>

          {/* Android Control Buttons */}
           {/* {showAndroidControls && ( */}
          {/* <ControlButtons
            onAction={handleControlAction}
            onIMESwitch={handleIMESwitch}
          /> */}
        {/* )}  */}
        </div>

        {/* Stats */}
        <div className={styles.statsArea}>
          <div className={styles.stats}>
            <div>Resolution: {stats.resolution || "N/A"}</div>
            <div>FPS: {stats.fps || 0}</div>
            <div>Latency: {stats.latency || 0}ms</div>
            <div>ConnectionStatus: {connectionStatus}</div>
          </div>
        </div>
      </div>
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
}
