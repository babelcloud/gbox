import { exec } from "child_process";

/**
 * Sanitizes result objects by truncating base64 data URIs to improve readability
 * while preserving the full data for actual image display
 */
export const sanitizeResult = (obj: any): any => {
  if (typeof obj !== 'object' || obj === null) {
    return obj;
  }
  
  if (Array.isArray(obj)) {
    return obj.map(sanitizeResult);
  }
  
  const sanitized: any = {};
  for (const [key, value] of Object.entries(obj)) {
    if (key === 'uri' && typeof value === 'string') {
      // Truncate base64 data URIs
      if (value.startsWith('data:')) {
        const match = value.match(/^(data:.+;base64,)(.*)$/);
        if (match && match[2].length > 20) {
          sanitized[key] = match[1] + match[2].substring(0, 20) + '...';
        } else {
          sanitized[key] = value;
        }
      } else if (value.length > 20) {
        // Truncate other long strings that might be base64
        sanitized[key] = value.substring(0, 20) + '...';
      } else {
        sanitized[key] = value;
      }
    } else {
      sanitized[key] = sanitizeResult(value);
    }
  }
  return sanitized;
};

export const openUrlInBrowser = (url: string) => {
  const command =
    process.platform === "darwin"
      ? `open "${url}"`
      : process.platform === "win32"
      ? `start "" "${url}"`
      : `xdg-open "${url}"`;

  // Execute the command to open the browser
  exec(command, (err) => {
    if (err) {
      console.error(`Failed to open browser for URL ${url}:`, err);
    }
  });
};