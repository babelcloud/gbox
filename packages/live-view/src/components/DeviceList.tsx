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
        ? `已连接 - ${device.videoWidth}x${device.videoHeight}`
        : '已连接';
    }
    return device.state;
  };

  const getStatusClass = (device: Device): string => {
    if (currentDevice === device.serial && connectionStatus) {
      if (connectionStatus.includes('正在连接') || 
          connectionStatus.includes('重连中') || 
          connectionStatus.includes('秒后重试')) {
        return styles.connecting;
      } else if (connectionStatus.includes('失败') || 
                 connectionStatus.includes('断开')) {
        return styles.error;
      }
    }
    return '';
  };

  return (
    <div className={styles.deviceList}>
      <h2>设备列表</h2>
      
      {loading && (
        <div className={styles.loading}>
          <div className={styles.spinner} />
          正在加载设备...
        </div>
      )}

      {!loading && devices.length === 0 && (
        <div className={styles.empty}>没有找到设备</div>
      )}

      {devices.map((device) => {
        const isDeviceConnected = device.connected || 
          (currentDevice === device.serial && isConnected);
        
        return (
          <div 
            key={device.serial}
            className={`${styles.deviceItem} ${isDeviceConnected ? styles.connected : ''}`}
            onClick={() => {
              if (!isDeviceConnected && device.state === 'device') {
                onConnect(device.serial);
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
                断开
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
};