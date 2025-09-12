import { useState, useCallback, useEffect } from 'react';
import { Device } from '../types';

interface UseDeviceManagerProps {
  apiUrl: string;
  showDeviceList: boolean;
  autoConnect: boolean;
  deviceSerial?: string;
  isConnected: boolean;
  onError?: (error: Error) => void;
}

export const useDeviceManager = ({
  apiUrl,
  showDeviceList,
  autoConnect,
  deviceSerial,
  isConnected,
  onError,
}: UseDeviceManagerProps) => {
  const [devices, setDevices] = useState<Device[]>([]);
  const [currentDevice, setCurrentDevice] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Load devices
  const loadDevices = useCallback(async () => {
    setLoading(true);
    try {
      const response = await fetch(`${apiUrl}/devices`);
      const data = await response.json();
      // Transform device data to match our interface
      // The API returns 'id' but we use 'serial' internally
      const transformedDevices = (data.devices || []).map((device: any) => ({
        serial: device.id || device.serial || device.udid,
        state: device.state,
        model: device['ro.product.model'] || device.model || 'Unknown',
        connected: device.connected,
      }));
      setDevices(transformedDevices);
    } catch (error) {
      console.error('Failed to load devices:', error);
      onError?.(error as Error);
    } finally {
      setLoading(false);
    }
  }, [apiUrl, onError]);

  useEffect(() => {
    if (showDeviceList) {
      loadDevices();
    }
  }, [showDeviceList, loadDevices]);

  return {
    devices,
    currentDevice,
    loading,
    setCurrentDevice,
    loadDevices,
  };
};
