import GboxSDK, { AndroidBoxOperator } from "gbox-sdk";
import { config } from "../config.js";
import type { DeviceInfo } from "gbox-sdk/resources/v1.js";

// Initialize Gbox SDK
const gboxSDK = new GboxSDK({
  apiKey: config.gboxApiKey,
  baseURL: config.gboxBaseURL,
});

export async function attachBox(boxId: string): Promise<AndroidBoxOperator> {
  try {
    const box = (await gboxSDK.get(boxId)) as AndroidBoxOperator;
    return box;
  } catch (err) {
    throw new Error(
      `Failed to attach to box ${boxId}: ${(err as Error).message}`
    );
  }
}

export async function deviceList(
  availableOnly: boolean = true
): Promise<DeviceInfo[]> {
  try {
    const response = await gboxSDK.client.v1.devices.list();
    const devices = response.data;

    return availableOnly
      ? devices.filter(
          device =>
            device.status === "online" &&
            device.enable === "enabled" &&
            device.isIdle
        )
      : devices;
  } catch (err) {
    console.error(err);
    return [];
  }
}

export { gboxSDK };
