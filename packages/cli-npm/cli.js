#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const pkg = require('./package.json');

async function main() {
  // Check for updates and notify if a new version is available
  try {
    const { default: updateNotifier } = await import('update-notifier');
    
    // Normal update check
    const notifier = updateNotifier({ pkg });
    notifier.notify();
  } catch (error) {
    // Silently ignore update check errors
  }

  const binaryName = process.platform === 'win32' ? 'gbox.exe' : 'gbox';
  const binaryPath = path.join(__dirname, 'bin', binaryName);

  const child = spawn(binaryPath, process.argv.slice(2), {
    stdio: 'inherit'
  });

  child.on('error', (err) => {
    console.error(`Error executing gbox binary: ${err}`);
    process.exit(1);
  });

  child.on('exit', (code) => {
    if (code !== null) {
      process.exit(code);
    }
  });
}

main(); 