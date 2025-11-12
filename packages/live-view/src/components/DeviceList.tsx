import React from 'react';
import { Device } from '../types';
import styles from './DeviceList.module.css';

interface DeviceListProps {
  devices: Device[];
  currentDevice: Device | null;
  connectionStatus: string;
  isConnected: boolean;
  loading: boolean;
  onConnect: (device: Device) => Promise<void>;
  onDisconnect: () => void;
  onRefresh: () => void;
}

export const DeviceList: React.FC<DeviceListProps> = ({
  devices,
  currentDevice,
  connectionStatus,
  isConnected,
  loading,
  onConnect,
  onDisconnect,
  onRefresh: _onRefresh
}) => {
  const getDeviceStatus = (device: Device): string => {
    if (currentDevice?.serial === device.serial && connectionStatus) {
      return connectionStatus;
    }
    if (device.connected || (currentDevice?.serial === device.serial && isConnected)) {
      return device.videoWidth && device.videoHeight
        ? `Connected - ${device.videoWidth}x${device.videoHeight}`
        : 'Connected';
    }
    return device.state;
  };

  const getStatusClass = (device: Device): string => {
    if (currentDevice?.serial === device.serial && connectionStatus) {
      if (connectionStatus.includes('Connecting') ||
          connectionStatus.includes('reconnecting') ||
          connectionStatus.includes('Reconnecting')) {
        return styles.connecting;
      } else if (connectionStatus.includes('failed') ||
                 connectionStatus.includes('Failed') ||
                 connectionStatus.includes('disconnected')) {
        return styles.error;
      }
    }
    return '';
  };

  return (
    <div className={styles.deviceList}>
      <h2>Device List</h2>
      
      {loading && (
        <div className={styles.loading}>
          <div className={styles.spinner} />
          Loading devices...
        </div>
      )}

      {!loading && devices.length === 0 && (
        <div className={styles.empty}>No devices found</div>
      )}

      {devices.map((device) => {
        // Simplified connection state logic
        const isDeviceConnected = currentDevice?.serial === device.serial && isConnected;

        return (
          <div 
            key={device.serial}
            className={`${styles.deviceItem} ${isDeviceConnected ? styles.connected : ''}`}
            onClick={async () => {
              if (!isDeviceConnected && device.state === 'device') {
                await onConnect(device);
              }
            }}
          >
            <div className={styles.deviceInfo}>
              <div className={styles.deviceSerial}>{device.serial}</div>
              <div className={styles.deviceModel}>{device.model || 'Unknown'}</div>
              <div className={`${styles.deviceState} ${getStatusClass(device)}`}>
                {getDeviceStatus(device)}
              </div>
            </div>
            
            {isDeviceConnected && (
              <button 
                className={styles.disconnectBtn}
                onClick={(e) => {
                  e.stopPropagation();
                  onDisconnect();
                }}
              >
                Stop
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
};