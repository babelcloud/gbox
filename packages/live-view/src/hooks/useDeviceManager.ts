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
      // The new API returns devices with serialno, metadata.model/hostname, and isConnected
      // Filter to only include mobile devices (platform === "mobile")
      const transformedDevices = (data.devices || [])
        .filter((device: Record<string, unknown>) => {
          // Only include mobile devices
          return device.platform === "mobile";
        })
        .map((device: Record<string, unknown>) => {
          const metadata = (device.metadata || {}) as Record<string, unknown>;
          // Get model: for mobile devices use metadata.model
          const model =
            (metadata.model as string) || (device.model as string) || "Unknown";

          // Get serial from serialno field
          const serial = (device.serialno ||
            device.id ||
            device.serial ||
            device.udid) as string;

          // Get connected status from isConnected field
          const connected = (device.isConnected || device.connected) as boolean;

          // Determine state: all available devices should have state 'device' to allow connection
          // The API returns devices that are available, so we use 'device' as the state
          const state = "device";

          return {
            serial,
            state,
            model,
            connected,
          };
        });
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
