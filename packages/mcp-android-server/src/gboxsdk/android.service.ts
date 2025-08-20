import GboxSDK, { AndroidBoxOperator } from "gbox-sdk";
import { config } from "../config.js";
import axios from "axios";

// Initialize Gbox SDK
const gboxSDK = new GboxSDK({
  apiKey: config.gboxApiKey,
  baseURL: config.gboxBaseURL,
});

export async function attachBox(boxId: string): Promise<AndroidBoxOperator> {
  try {
    const box = await gboxSDK.get(boxId) as AndroidBoxOperator;
    return box;
  } catch (err) {
    throw new Error(
      `Failed to attach to box ${boxId}: ${(err as Error).message}`
    );
  }
}

export type AndroidDevice = {
  id: string;
  model: string;
  status: "online" | "offline";
  enabled: boolean;
  isIdle: boolean;
}

export async function deviceList(availableOnly: boolean = true): Promise<AndroidDevice[]> {
  // TODO: use gbox-sdk to get device list and should be able to change baseUrl
  const apiUrl = `https://gbox.ai/api/dashboard/v1/device/device_list`;
  const response = await axios.post(
    apiUrl,
    {},
    {
      headers: {
        "x-api-key": config.gboxApiKey
      }
    }
  );

  try {
    const devices = response.data.devices.map((device: any) => ({
      id: device.deviceId,
      model: device.deviceData["ro.product.model"],
      status: device.status,
      enabled: device.enable,
      isIdle: device.isIdle
    }));

    if (availableOnly) {
      return devices.filter((device: AndroidDevice) => device.status === "online" && device.enabled && device.isIdle);
    }

    return devices;
  } catch (err) {
    console.error(err);
    return [];
  }
}

export { gboxSDK };