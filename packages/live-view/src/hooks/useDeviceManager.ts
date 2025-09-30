import { useState, useCallback, useEffect } from "react";
import { Device } from "../types";

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
  autoConnect: _autoConnect,
  deviceSerial: _deviceSerial,
  isConnected: _isConnected,
  onError,
}: UseDeviceManagerProps) => {
  const [devices, setDevices] = useState<Device[]>([]);
  const [currentDevice, setCurrentDevice] = useState<Device | null>(null);
  const [loading, setLoading] = useState(false);

  // Load devices
  const loadDevices = useCallback(async () => {
    console.log(
      "[useDeviceManager] Loading devices from:",
      `${apiUrl}/devices`
    );
    setLoading(true);
    try {
      const response = await fetch(`${apiUrl}/devices`);
      console.log("[useDeviceManager] Response status:", response.status);
      const data = await response.json();
      console.log("[useDeviceManager] Raw response data:", data);
      console.log("[useDeviceManager] Raw devices array:", data.devices);
      if (data.devices && data.devices.length > 0) {
        console.log("[useDeviceManager] First raw device:", data.devices[0]);
      }
      // Transform device data to match our interface
      // The API returns 'id' but we use 'serial' internally
      const transformedDevices = (data.devices || []).map(
        (device: Record<string, unknown>) => ({
          serial: (device.id || device.serial || device.udid) as string,
          state: (device.status || device.state) as string, // API returns 'status', frontend expects 'state'
          model: (device["ro.product.model"] ||
            device.model ||
            "Unknown") as string,
          connected: device.connected as boolean,
        })
      );
      console.log(
        "[useDeviceManager] Transformed devices:",
        transformedDevices
      );
      setDevices(transformedDevices);
    } catch (error) {
      console.error("[useDeviceManager] Failed to load devices:", error);
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
