import GboxSDK, { AndroidBoxOperator } from "gbox-sdk";
import { config } from "../config.js";

// Initialize Gbox SDK
const gboxSDK = new GboxSDK({ apiKey: config.gboxApiKey });

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

export { gboxSDK };