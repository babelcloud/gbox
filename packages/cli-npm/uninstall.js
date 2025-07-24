const fs = require('fs');
const path = require('path');

const binDir = path.join(__dirname, 'bin');
if (fs.existsSync(binDir)) {
  try {
    fs.rmSync(binDir, { recursive: true, force: true });
    console.log('Cleaned up gbox binaries.');
  } catch (error) {
    if (error.code === 'EACCES') {
      console.error('Failed to clean up gbox binaries due to permission errors.');
      console.error('Please try running the uninstall command with sudo:');
      const pkgName = require('./package.json').name;
      console.error(`sudo npm uninstall -g ${pkgName}`);
    } else {
      console.error('An unexpected error occurred during cleanup:', error);
    }
  }
} 