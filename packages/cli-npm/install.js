const fs = require('fs');
const path = require('path');
const axios = require('axios');
const tar = require('tar');
const unzipper = require('unzipper');

const packageJson = require('./package.json');
const releaseVersion = '0.0.15';

async function install() {
  const repo = packageJson.repository.url.match(/github\.com\/(.*)\.git/)[1];

  const platform = process.platform;
  const arch = process.arch;

  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows'
  };

  const archMap = {
    'x64': 'amd64',
    'arm64': 'arm64'
  };

  if (!platformMap[platform] || !archMap[arch]) {
    console.error(`Unsupported platform/architecture: ${platform}/${arch}`);
    process.exit(1);
  }

  const goPlatform = platformMap[platform];
  const goArch = archMap[arch];
  const isWindows = goPlatform === 'windows';
  const extension = isWindows ? 'zip' : 'tar.gz';
  const binaryName = isWindows ? 'gbox.exe' : 'gbox';
  const finalBinaryDir = path.join(__dirname, 'bin');

  const url = `https://github.com/${repo}/releases/download/v${releaseVersion}/gbox-${goPlatform}-${goArch}-${releaseVersion}.${extension}`;
  console.log(`Downloading gbox binary from ${url}`);

  try {
    const response = await axios({
      url,
      method: 'GET',
      responseType: 'stream'
    });

    if (!fs.existsSync(finalBinaryDir)) {
      fs.mkdirSync(finalBinaryDir, { recursive: true });
    }
    
    const tempDir = fs.mkdtempSync(path.join(__dirname, 'temp-'));

    if (isWindows) {
      await new Promise((resolve, reject) => {
        response.data.pipe(unzipper.Extract({ path: tempDir }))
          .on('finish', resolve)
          .on('error', reject);
      });
    } else {
        await new Promise((resolve, reject) => {
            response.data.pipe(tar.x({ C: tempDir }))
              .on('finish', resolve)
              .on('error', reject);
        });
    }
    
    const tempBinPath = path.join(tempDir, 'packages', 'cli', 'gbox');
    const finalBinPath = path.join(finalBinaryDir, binaryName);

    fs.renameSync(tempBinPath, finalBinPath);
    if (!isWindows) {
      fs.chmodSync(finalBinPath, 0o755);
    }
    
    fs.rmSync(tempDir, { recursive: true, force: true });
    
    console.log('gbox installed successfully.');

  } catch (error) {
    console.error('Failed to download or install gbox binary.');
    if (error.response) {
        console.error(`Status: ${error.response.status}`);
        if(error.response.status === 404) {
            console.error(`File not found at ${url}. Please check the version and release assets.`);
        }
    } else {
        console.error(error.message);
    }
    process.exit(1);
  }
}

install(); 