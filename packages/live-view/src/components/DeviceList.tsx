import React from 'react';
import { Device } from '../types';
import styles from './DeviceList.module.css';

interface DeviceListProps {
  devices: Device[];
  currentDevice: string | null;
  connectionStatus: string;
  isConnected: boolean;
  loading: boolean;
  onConnect: (serial: string) => void;
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
  onRefresh,
}) => {
  const getDeviceStatus = (device: Device): string => {
    if (currentDevice === device.serial && connectionStatus) {
      return connectionStatus;
    }
    if (device.connected || (currentDevice === device.serial && isConnected)) {
      return device.videoWidth && device.videoHeight
        ? `Connected - ${device.videoWidth}x${device.videoHeight}`
        : 'Connected';
    }
    return device.state;
  };

  const getStatusClass = (device: Device): string => {
    if (currentDevice === device.serial && connectionStatus) {
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
        const isDeviceConnected = device.connected ||
          (currentDevice === device.serial && isConnected) ||
          (currentDevice === device.serial && connectionStatus && !connectionStatus.includes('failed') && !connectionStatus.includes('disconnected'));


        return (
          <div 
            key={device.serial}
            className={`${styles.deviceItem} ${isDeviceConnected ? styles.connected : ''}`}
            onClick={() => {
              console.log('[DeviceList] Device clicked:', {
                serial: device.serial,
                state: device.state,
                isDeviceConnected,
                canConnect: !isDeviceConnected && device.state === 'device'
              });
              if (!isDeviceConnected && device.state === 'device') {
                console.log('[DeviceList] Calling onConnect for device:', device.serial);
                onConnect(device.serial);
              } else {
                console.log('[DeviceList] Cannot connect to device - conditions not met');
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