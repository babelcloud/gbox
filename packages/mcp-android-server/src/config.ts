import "dotenv/config";
import { readFileSync } from "fs";
import { join } from "path";
import { homedir } from "os";

interface Profile {
  api_key: string;
  name: string;
  organization_name: string;
  current: boolean;
}

function getApiKeyFromProfile(): string | null {
  try {
    // Get profile path following the same logic as CLI
    const profilePath = join(homedir(), ".gbox", "profile.json");
    
    // Read and parse profile file
    const profileData = readFileSync(profilePath, "utf-8");
    const profiles: Profile[] = JSON.parse(profileData);
    
    // Find the current active profile
    const currentProfile = profiles.find(profile => profile.current === true);
    
    if (currentProfile && currentProfile.api_key) {
      // Use stderr for logging to avoid interfering with MCP JSON protocol on stdout
      console.error(`[INFO] Using API key from profile: ${currentProfile.name} (${currentProfile.organization_name})`);
      return currentProfile.api_key;
    }
    
    // If we have profiles but none is current, or current profile has no API key
    if (profiles.length > 0) {
      console.warn("Profile file found but no current active profile with API key. Use 'gbox profile use' to set an active profile.");
    }
    
    return null;
  } catch (error) {
    // Profile file doesn't exist or can't be read
    if (process.env.DEBUG) {
      console.debug("Profile file not found or cannot be read:", error);
    }
    return null;
  }
}

// Try to get API key from environment variable first, then from profile
const apiKey = process.env.GBOX_API_KEY || getApiKeyFromProfile();

if (!apiKey) {
  throw new Error(
    "No API key found. Please configure one of the following:\n" +
    "1. Configure a profile using gbox CLI 'gbox profile add --key YOUR_API_KEY --name YOUR_PROFILE_NAME'\n" +
    "   Then set it as current with 'gbox profile use'\n" +
    "2. Set GBOX_API_KEY environment variable or create a .env file with GBOX_API_KEY=YOUR_API_KEY"
  );
}

export const config = {
  gboxApiKey: apiKey,
  mode: process.env.MODE?.toLowerCase() || "stdio",
};