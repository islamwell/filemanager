#!/usr/bin/env node
'use strict';

// ==============================================================================
// Go File Manager — Production A-to-Z Linux Installer & Management Suite (Node.js)
// Zero external npm dependencies • Wizard • Express • Plesk • Diagnostics • Backups
// ==============================================================================

const fs = require('fs');
const path = require('path');
const os = require('os');
const child_process = require('child_process');
const readline = require('readline');
const http = require('http');
const net = require('net');
const crypto = require('crypto');

// ------------------------------------------------------------------------------
// Default Configuration & Application Metadata
// ------------------------------------------------------------------------------
const APP_NAME = 'filemanager';
const APP_VERSION = 'v1.0.9';
const DEFAULT_PORT = 8080;
const DEFAULT_HOST = '0.0.0.0';
const DEFAULT_DATA_DIR = '/var/lib/filemanager';
const DEFAULT_BIN_PATH = '/usr/local/bin/filemanager';
const DEFAULT_CONFIG_DIR = '/etc/filemanager';
const DEFAULT_ENV_FILE = '/etc/filemanager/filemanager.env';
const DEFAULT_SERVICE_FILE = '/etc/systemd/system/filemanager.service';
const DEFAULT_BACKUP_DIR = '/var/backups/filemanager';
const DEFAULT_MAX_FILE_MB = 100;
const SYSTEM_USER = 'filemanager';
const SYSTEM_GROUP = 'filemanager';

// Runtime configuration object
const config = {
  port: DEFAULT_PORT,
  host: DEFAULT_HOST,
  dataDir: DEFAULT_DATA_DIR,
  binPath: DEFAULT_BIN_PATH,
  configDir: DEFAULT_CONFIG_DIR,
  envFile: DEFAULT_ENV_FILE,
  serviceFile: DEFAULT_SERVICE_FILE,
  backupDir: DEFAULT_BACKUP_DIR,
  maxFileMb: DEFAULT_MAX_FILE_MB,
  secureCookie: false,
  skipService: false,
  debugMode: false,
  forceOverwrite: false,
  dryRun: false,
  interactive: process.stdin.isTTY,
};

// ------------------------------------------------------------------------------
// Terminal Styling & Logging
// ------------------------------------------------------------------------------
const c = {
  reset: '\x1b[0m',
  bold: '\x1b[1m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  gray: '\x1b[90m',
};

function timestamp() {
  const d = new Date();
  return d.toISOString().replace('T', ' ').substring(0, 19);
}

const log = {
  info: (...args) => console.log(`${c.blue}[${timestamp()}] [INFO]${c.reset}`, ...args),
  debug: (...args) => {
    if (config.debugMode) console.log(`${c.magenta}[${timestamp()}] [DEBUG]${c.reset}`, ...args);
  },
  success: (...args) => console.log(`${c.green}[${timestamp()}] [SUCCESS]${c.reset} ${c.bold}${args.join(' ')}${c.reset}`),
  warn: (...args) => console.warn(`${c.yellow}[${timestamp()}] [WARN]${c.reset}`, ...args),
  error: (...args) => console.error(`${c.red}[${timestamp()}] [ERROR]${c.reset} ${c.bold}${args.join(' ')}${c.reset}`),
  fatal: (...args) => {
    console.error(`${c.red}[${timestamp()}] [FATAL]${c.reset} ${c.bold}${args.join(' ')}${c.reset}`);
    process.exit(1);
  },
};

// ------------------------------------------------------------------------------
// Command Execution & System Utilities
// ------------------------------------------------------------------------------
function run(cmd, args = [], options = {}) {
  const fullCmd = [cmd, ...args].join(' ');
  log.debug(`Executing: ${fullCmd}`);

  if (config.dryRun) {
    log.info(`[DRY-RUN] ${fullCmd}`);
    return { status: 0, stdout: '', stderr: '' };
  }

  const result = child_process.spawnSync(cmd, args, {
    stdio: options.stdio || 'pipe',
    encoding: 'utf8',
    env: { ...process.env, ...(options.env || {}) },
    ...options,
  });

  if (result.error) {
    throw result.error;
  }
  return result;
}

function commandExists(cmd) {
  try {
    const res = child_process.spawnSync(process.platform === 'win32' ? 'where' : 'which', [cmd], {
      stdio: 'ignore',
    });
    return res.status === 0;
  } catch {
    return false;
  }
}

function isRoot() {
  return typeof process.getuid === 'function' && process.getuid() === 0;
}

function isLinux() {
  return process.platform === 'linux';
}

function isSystemdActive() {
  return isLinux() && fs.existsSync('/run/systemd/system');
}

// ------------------------------------------------------------------------------
// Interactive UI Helper (Readline with Prompts)
// ------------------------------------------------------------------------------
let rlInstance = null;
function getReadline() {
  if (!rlInstance) {
    rlInstance = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
    });
  }
  return rlInstance;
}

function closeReadline() {
  if (rlInstance) {
    rlInstance.close();
    rlInstance = null;
  }
}

function askQuestion(query, defaultValue = '') {
  return new Promise((resolve) => {
    const rl = getReadline();
    const promptText = defaultValue ? `${query} [default: ${defaultValue}]: ` : `${query}: `;
    rl.question(promptText, (answer) => {
      const trimmed = answer.trim();
      resolve(trimmed || defaultValue);
    });
  });
}

function waitForEnter(prompt = 'Press [Enter] to continue...') {
  return new Promise((resolve) => {
    const rl = getReadline();
    rl.question(`\n${c.gray}${prompt}${c.reset}`, () => {
      resolve();
    });
  });
}

// Graceful exit handler on SIGINT
process.on('SIGINT', () => {
  console.log(`\n\n${c.yellow}Operation cancelled by user. Exiting.${c.reset}`);
  closeReadline();
  process.exit(130);
});

// ------------------------------------------------------------------------------
// UI Banners
// ------------------------------------------------------------------------------
function printBanner() {
  if (process.stdout.isTTY) {
    try {
      process.stdout.write('\x1Bc');
    } catch {}
  }
  console.log(`${c.cyan}${c.bold}`);
  console.log(`  ███████╗██╗██╗     ███████╗███╗   ███╗ █████╗ ███╗   ██╗ █████╗  ██████╗ ███████╗██████╗ `);
  console.log(`  ██╔════╝██║██║     ██╔════╝████╗ ████║██╔══██╗████╗  ██║██╔══██╗██╔════╝ ██╔════╝██╔══██╗`);
  console.log(`  █████╗  ██║██║     █████╗  ██╔████╔██║███████║██╔██╗ ██║███████║██║  ███╗█████╗  ██████╔╝`);
  console.log(`  ██╔══╝  ██║██║     ██╔══╝  ██║╚██╔╝██║██╔══██║██║╚██╗██║██╔══██║██║   ██║██╔══╝  ██╔══██╗`);
  console.log(`  ██║     ██║███████╗███████╗██║ ╚═╝ ██║██║  ██║██║ ╚████║██║  ██║╚██████╔╝███████╗██║  ██║`);
  console.log(`  ╚═╝     ╚═╝╚══════╝╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═╝`);
  console.log(`${c.reset}`);
  console.log(`  ${c.bold}Go File Manager (${APP_VERSION}) — Production Linux Installer & Suite${c.reset}`);
  console.log(`  ${c.magenta}Pure Go (CGO-free SQLite) • Audio Range Player • 4-Level Folders • Fuzzy Search${c.reset}`);
  console.log(`  ${c.gray}Node.js Native Edition (Zero Dependencies) • Host: ${os.hostname()} (${os.platform()} ${os.arch()})${c.reset}`);
  console.log(`  ----------------------------------------------------------------------------------\n`);
}

// ------------------------------------------------------------------------------
// System Verification & Edge-Case Checks
// ------------------------------------------------------------------------------
function checkNodeVersion() {
  const majorVersion = parseInt(process.versions.node.split('.')[0], 10);
  if (majorVersion < 14) {
    log.fatal(`Node.js version 14.0.0 or higher is required (detected: ${process.version}).`);
  }
}

function checkRoot(requiredForAction = 'this operation') {
  if (config.dryRun) return;
  if (!isRoot()) {
    log.fatal(`Root privileges required for ${requiredForAction}.\nPlease re-run with: sudo node ${path.basename(__filename)}`);
  }
}

function checkDiskSpace(targetDir) {
  try {
    const parentDir = fs.existsSync(targetDir) ? targetDir : path.dirname(targetDir);
    if (typeof fs.statfsSync === 'function') {
      const stats = fs.statfsSync(parentDir);
      const freeMb = Math.floor((stats.bavail * stats.bsize) / (1024 * 1024));
      log.debug(`Available disk space at ${parentDir}: ${freeMb} MB`);
      if (freeMb < 500) {
        log.warn(`Low disk space on ${parentDir}: only ${freeMb} MB available (minimum 500 MB recommended).`);
      }
      return freeMb;
    } else {
      const dfRes = run('df', ['-k', parentDir]);
      if (dfRes.status === 0) {
        const lines = dfRes.stdout.trim().split('\n');
        if (lines.length >= 2) {
          const parts = lines[1].trim().split(/\s+/);
          const freeKb = parseInt(parts[3], 10);
          const freeMb = Math.floor(freeKb / 1024);
          if (freeMb < 500) {
            log.warn(`Low disk space on ${parentDir}: only ${freeMb} MB available.`);
          }
          return freeMb;
        }
      }
    }
  } catch (err) {
    log.debug(`Disk space check warning: ${err.message}`);
  }
  return 1024;
}

function isPortInUse(port, host = '127.0.0.1') {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once('error', (err) => {
      if (err.code === 'EADDRINUSE') {
        resolve(true);
      } else {
        resolve(false);
      }
    });
    server.once('listening', () => {
      server.close(() => resolve(false));
    });
    server.listen(port, host);
  });
}

function detectBinaryType(filePath) {
  if (!fs.existsSync(filePath)) return null;
  try {
    const fd = fs.openSync(filePath, 'r');
    const buffer = Buffer.alloc(4);
    fs.readSync(fd, buffer, 0, 4, 0);
    fs.closeSync(fd);

    // ELF magic: 7F 45 4C 46
    if (buffer[0] === 0x7f && buffer[1] === 0x45 && buffer[2] === 0x4c && buffer[3] === 0x46) {
      return 'linux-elf';
    }
    // Mach-O magic: CF FA ED FE or FE ED FA CF
    if ((buffer[0] === 0xcf && buffer[1] === 0xfa && buffer[2] === 0xed && buffer[3] === 0xfe) ||
        (buffer[0] === 0xfe && buffer[1] === 0xed && buffer[2] === 0xfa && buffer[3] === 0xcf)) {
      return 'macos-macho';
    }
    // Windows PE magic: 4D 5A (MZ)
    if (buffer[0] === 0x4d && buffer[1] === 0x5a) {
      return 'windows-pe';
    }
  } catch {
    return 'unknown';
  }
  return 'unknown';
}

function detectPackageManager() {
  const managers = [
    { name: 'apt-get', install: ['apt-get', 'update', '-qq'], pkg: ['apt-get', 'install', '-y', '-qq', 'golang'] },
    { name: 'dnf', install: [], pkg: ['dnf', 'install', '-y', '-q', 'golang'] },
    { name: 'yum', install: [], pkg: ['yum', 'install', '-y', '-q', 'golang'] },
    { name: 'apk', install: [], pkg: ['apk', 'add', '--no-cache', 'go'] },
    { name: 'pacman', install: [], pkg: ['pacman', '-Sy', '--noconfirm', 'go'] },
  ];
  for (const mgr of managers) {
    if (commandExists(mgr.name)) return mgr;
  }
  return null;
}

// ------------------------------------------------------------------------------
// Binary Preparation (Pre-built, Architecture Check, or Source Build)
// ------------------------------------------------------------------------------
async function prepareBinary(scriptDir, targetBinPath) {
  log.info('Preparing application binary...');
  const buildTmp = path.join(os.tmpdir(), `filemanager-build-${Date.now()}`);

  if (config.dryRun) {
    log.info(`[DRY-RUN] Would prepare binary and place it at ${targetBinPath}`);
    return;
  }

  fs.mkdirSync(buildTmp, { recursive: true });

  const localBinary = path.join(scriptDir, 'filemanager');
  const linuxArchBinary = path.join(scriptDir, `filemanager-linux-${os.arch() === 'x64' ? 'amd64' : os.arch()}`);
  let binarySource = null;

  // Check pre-built binary matching current OS
  if (fs.existsSync(linuxArchBinary) && isLinux()) {
    log.info(`Found pre-built Linux architecture binary: ${path.basename(linuxArchBinary)}`);
    binarySource = linuxArchBinary;
  } else if (fs.existsSync(localBinary)) {
    const binType = detectBinaryType(localBinary);
    if (isLinux() && binType === 'macos-macho') {
      log.warn(`Local binary was compiled for macOS (Mach-O) and cannot run natively on Linux.`);
    } else if (isLinux() && binType === 'linux-elf') {
      log.info('Found valid Linux ELF pre-built binary in repository.');
      binarySource = localBinary;
    } else if (!isLinux()) {
      binarySource = localBinary;
    }
  }

  // If no valid pre-built binary, build from source
  if (!binarySource) {
    const mainGo = path.join(scriptDir, 'cmd', 'server', 'main.go');
    if (!fs.existsSync(mainGo)) {
      log.fatal(`Cannot find 'cmd/server/main.go' in ${scriptDir}. Cannot compile.`);
    }

    log.info('Compiling standalone binary from Go source files...');

    if (!commandExists('go')) {
      log.warn("Go compiler ('go') not found in PATH.");
      const pm = detectPackageManager();
      if (pm && isRoot()) {
        log.info(`Attempting to install Go via system package manager (${pm.name})...`);
        try {
          if (pm.install.length > 0) run(pm.install[0], pm.install.slice(1), { stdio: 'inherit' });
          run(pm.pkg[0], pm.pkg.slice(1), { stdio: 'inherit' });
        } catch (e) {
          log.warn(`Auto-installation of Go failed: ${e.message}`);
        }
      }
    }

    if (!commandExists('go')) {
      log.fatal(
        "Could not find or install 'go'. Please install Go 1.22+ or provide a pre-compiled 'filemanager' binary."
      );
    }

    // Run Go build
    log.info("Running: CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' cmd/server/main.go");
    const buildRes = run(
      'go',
      ['build', '-trimpath', '-ldflags=-s -w', '-o', path.join(buildTmp, 'filemanager'), 'cmd/server/main.go'],
      { cwd: scriptDir, env: { CGO_ENABLED: '0' }, stdio: 'inherit' }
    );

    if (buildRes.status !== 0) {
      log.fatal('Go compilation failed. Check compiler output above.');
    }
    binarySource = path.join(buildTmp, 'filemanager');
    log.success('Go compilation completed successfully.');
  } else {
    // Copy pre-built
    const dest = path.join(buildTmp, 'filemanager');
    fs.copyFileSync(binarySource, dest);
    binarySource = dest;
  }

  // Install to target path
  const targetDir = path.dirname(targetBinPath);
  fs.mkdirSync(targetDir, { recursive: true });

  fs.copyFileSync(binarySource, targetBinPath);
  fs.chmodSync(targetBinPath, 0o755);

  // Clean up build temp
  try {
    fs.rmSync(buildTmp, { recursive: true, force: true });
  } catch {}

  log.success(`Application binary successfully installed at: ${targetBinPath}`);
}

// ------------------------------------------------------------------------------
// User & Storage Management
// ------------------------------------------------------------------------------
function setupUserAndStorage(dataDir, configDir, backupDir, user, group) {
  if (config.dryRun) {
    log.info(`[DRY-RUN] Would create system user '${user}', group '${group}', and directories.`);
    return;
  }

  log.info(`Setting up dedicated system user '${user}' and storage layout...`);

  if (isLinux()) {
    // Check group
    try {
      run('getent', ['group', group]);
    } catch {
      log.info(`Creating system group '${group}'...`);
      run('groupadd', ['--system', group]);
    }

    // Check user
    try {
      run('getent', ['passwd', user]);
    } catch {
      log.info(`Creating system user '${user}'...`);
      const shell = fs.existsSync('/usr/sbin/nologin') ? '/usr/sbin/nologin' : '/bin/false';
      run('useradd', [
        '--system',
        '--gid',
        group,
        '--home-dir',
        dataDir,
        '--no-create-home',
        '--shell',
        shell,
        '--comment',
        'Go File Manager Daemon',
        user,
      ]);
    }
  }

  // Create directories
  const subDirs = [
    path.join(dataDir, 'data'),
    path.join(dataDir, 'storage'),
    configDir,
    backupDir,
  ];

  for (const dir of subDirs) {
    fs.mkdirSync(dir, { recursive: true });
  }

  if (isLinux() && isRoot()) {
    try {
      run('chown', ['-R', `${user}:${group}`, dataDir]);
      run('chmod', ['-R', '750', dataDir]);
      run('chown', ['-R', `${user}:${group}`, configDir]);
      run('chmod', ['750', configDir]);
      run('chmod', ['700', backupDir]);
    } catch (err) {
      log.warn(`Warning setting permissions: ${err.message}`);
    }
  }

  log.success(`Storage directories configured with 0750 permissions at ${dataDir}`);
}

// ------------------------------------------------------------------------------
// Configuration File Writer
// ------------------------------------------------------------------------------
function writeConfiguration(envFile, opts) {
  log.info(`Writing configuration to ${envFile}...`);
  const maxBytes = opts.maxFileMb * 1024 * 1024;

  if (fs.existsSync(envFile) && !config.forceOverwrite) {
    const backupFile = `${envFile}.bak.${Date.now()}`;
    log.warn(`Configuration file already exists. Backing up to ${backupFile}`);
    if (!config.dryRun) fs.copyFileSync(envFile, backupFile);
  }

  const content = `# ==============================================================================
# Go File Manager Configuration (${APP_VERSION})
# Generated on: ${timestamp()}
# ==============================================================================
PORT=${opts.port}
HOST=${opts.host}
DATABASE_PATH=${path.join(opts.dataDir, 'data', 'filemanager.db')}
STORAGE_DIR=${path.join(opts.dataDir, 'storage')}
MAX_FILE_SIZE=${maxBytes}
SECURE_COOKIE=${opts.secureCookie}
`;

  if (config.dryRun) {
    log.info(`[DRY-RUN] Would write to ${envFile}:\n${content}`);
    return;
  }

  fs.writeFileSync(envFile, content, { encoding: 'utf8', mode: 0o600 });
  if (isLinux() && isRoot()) {
    try {
      run('chown', [`${SYSTEM_USER}:${SYSTEM_GROUP}`, envFile]);
    } catch {}
  }
  log.success(`Configuration created at ${envFile}`);
}

// ------------------------------------------------------------------------------
// Systemd Service Unit Installation
// ------------------------------------------------------------------------------
function installSystemdService(serviceFile, opts) {
  if (opts.skipService) {
    log.info('Skipping systemd service installation (flag or non-systemd environment).');
    return;
  }

  if (!isSystemdActive()) {
    log.warn('Systemd is not running as PID 1 (Docker/WSL/container detected). Skipping systemd unit.');
    return;
  }

  log.info(`Configuring hardened systemd service unit at ${serviceFile}...`);

  const unitContent = `[Unit]
Description=Go File Manager Web Application (${APP_VERSION})
Documentation=https://github.com/filemanager
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SYSTEM_USER}
Group=${SYSTEM_GROUP}
WorkingDirectory=${opts.dataDir}
EnvironmentFile=${opts.envFile}
ExecStart=${opts.binPath}
Restart=always
RestartSec=5s

# Security Sandboxing Directives
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ReadWritePaths=${opts.dataDir}
ReadOnlyPaths=${opts.configDir}

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

StandardOutput=journal
StandardError=journal
SyslogIdentifier=${APP_NAME}

[Install]
WantedBy=multi-user.target
`;

  if (config.dryRun) {
    log.info(`[DRY-RUN] Would create systemd service file at ${serviceFile}`);
    return;
  }

  fs.writeFileSync(serviceFile, unitContent, { encoding: 'utf8', mode: 0o644 });

  try {
    run('systemctl', ['daemon-reload']);
    run('systemctl', ['enable', `${APP_NAME}.service`]);
    run('systemctl', ['restart', `${APP_NAME}.service`]);
    log.success(`Systemd service '${APP_NAME}' enabled and started.`);
  } catch (err) {
    log.error(`Failed to enable/restart systemd service: ${err.message}`);
  }
}

// ------------------------------------------------------------------------------
// Live Health Check Loop
// ------------------------------------------------------------------------------
async function runHealthCheck(port, host, skipService) {
  if (skipService || config.dryRun) return true;

  const targetHost = host === '0.0.0.0' ? '127.0.0.1' : host;
  log.info(`Running live health check on http://${targetHost}:${port}/healthz...`);

  const maxAttempts = 15;
  let isHealthy = false;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    const success = await new Promise((resolve) => {
      const req = http.get(
        {
          hostname: targetHost,
          port: port,
          path: '/healthz',
          timeout: 2000,
        },
        (res) => {
          if (res.statusCode === 200) {
            resolve(true);
          } else {
            resolve(false);
          }
        }
      );
      req.on('error', () => resolve(false));
      req.on('timeout', () => {
        req.destroy();
        resolve(false);
      });
    });

    if (success) {
      isHealthy = true;
      break;
    }

    // Fallback: check /login
    const loginSuccess = await new Promise((resolve) => {
      const req = http.get(
        {
          hostname: targetHost,
          port: port,
          path: '/login',
          timeout: 1500,
        },
        (res) => {
          if (res.statusCode >= 200 && res.statusCode < 400) {
            resolve(true);
          } else {
            resolve(false);
          }
        }
      );
      req.on('error', () => resolve(false));
      req.on('timeout', () => {
        req.destroy();
        resolve(false);
      });
    });

    if (loginSuccess) {
      isHealthy = true;
      break;
    }

    await new Promise((r) => setTimeout(r, 1000));
  }

  if (isHealthy) {
    log.success('Health check PASSED! Application is actively serving requests.');
    return true;
  } else {
    log.error(`Health check FAILED! Service did not respond within ${maxAttempts}s.`);
    if (isLinux() && commandExists('journalctl')) {
      console.log(`\n${c.yellow}Recent Journal Logs for ${APP_NAME}:${c.reset}`);
      try {
        const logs = run('journalctl', ['-u', APP_NAME, '-n', '25', '--no-pager']);
        console.log(logs.stdout || logs.stderr);
      } catch {}
    }
    return false;
  }
}

// ------------------------------------------------------------------------------
// Installation Summary Banner
// ------------------------------------------------------------------------------
function printInstallSummary(opts) {
  console.log(`\n${c.green}======================================================================${c.reset}`);
  console.log(`${c.green}${c.bold}     Go File Manager (${APP_VERSION}) successfully installed and running!${c.reset}`);
  console.log(`${c.green}======================================================================${c.reset}`);
  console.log(`  ${c.bold}Listening Address${c.reset}  : http://${opts.host}:${opts.port}`);
  console.log(`  ${c.bold}Binary Location${c.reset}    : ${opts.binPath}`);
  console.log(`  ${c.bold}Data Directory${c.reset}     : ${opts.dataDir}`);
  console.log(`  ${c.bold}Config File${c.reset}        : ${opts.envFile}`);
  if (!opts.skipService && isSystemdActive()) {
    console.log(`  ${c.bold}Service Status${c.reset}     : sudo systemctl status ${APP_NAME}`);
    console.log(`  ${c.bold}Live Log Stream${c.reset}    : sudo journalctl -u ${APP_NAME} -f`);
    console.log(`  ${c.bold}Restart Service${c.reset}    : sudo systemctl restart ${APP_NAME}`);
  } else {
    console.log(`  ${c.bold}Start Command${c.reset}      : ${opts.binPath}`);
  }
  console.log(`${c.green}======================================================================${c.reset}\n`);
}

// ------------------------------------------------------------------------------
// Core Installation Runner
// ------------------------------------------------------------------------------
async function executeInstallation(opts = config) {
  checkNodeVersion();
  checkRoot('system installation');

  if (isLinux()) {
    checkDiskSpace(opts.dataDir);
  } else {
    log.warn(`Running on ${process.platform}. Linux systemd unit and system users will be bypassed.`);
    opts.skipService = true;
  }

  const portBusy = await isPortInUse(opts.port, opts.host);
  if (portBusy) {
    log.warn(`Port ${opts.port} is already listening. If an older instance of ${APP_NAME} is active, it will be safely replaced.`);
  }

  const scriptDir = __dirname;
  await prepareBinary(scriptDir, opts.binPath);
  setupUserAndStorage(opts.dataDir, opts.configDir, opts.backupDir, SYSTEM_USER, SYSTEM_GROUP);
  writeConfiguration(opts.envFile, opts);
  installSystemdService(opts.serviceFile, opts);
  await runHealthCheck(opts.port, opts.host, opts.skipService);
  printInstallSummary(opts);
}

// ------------------------------------------------------------------------------
// Interactive Wizard Mode
// ------------------------------------------------------------------------------
async function runWizard() {
  printBanner();
  console.log(`${c.bold}${c.cyan}=== Step-by-Step Installation Wizard ===${c.reset}\n`);

  // 1. Profile selection
  console.log(`${c.bold}Select Deployment Profile:${c.reset}`);
  console.log(`  1) Standalone Direct HTTP (Listens publicly on 0.0.0.0)`);
  console.log(`  2) Behind Reverse Proxy / Plesk / Nginx / Caddy (127.0.0.1 with HTTPS Secure Cookies)`);
  console.log(`  3) Custom advanced parameters`);

  const profileChoice = await askQuestion('Select [1-3]', '1');

  if (profileChoice === '2') {
    config.host = '127.0.0.1';
    config.secureCookie = true;
  } else if (profileChoice === '3') {
    config.host = await askQuestion('Enter bind host', config.host);
    const secChoice = await askQuestion('Enable Secure HTTPS Cookies? (y/N)', 'N');
    config.secureCookie = /^y(es)?$/i.test(secChoice);
  } else {
    config.host = '0.0.0.0';
    config.secureCookie = false;
  }

  // 2. Port selection
  let portInput = await askQuestion('Enter HTTP listening port', String(config.port));
  let portNum = parseInt(portInput, 10);
  if (isNaN(portNum) || portNum < 1 || portNum > 65535) {
    log.warn('Invalid port. Defaulting to 8080.');
    portNum = 8080;
  }
  config.port = portNum;

  // Port collision check
  const busy = await isPortInUse(config.port, config.host);
  if (busy) {
    log.warn(`Notice: Port ${config.port} appears to already be in use.`);
    const continueChoice = await askQuestion('Continue anyway (e.g. replacing existing instance)? [Y/n]', 'Y');
    if (/^n(o)?$/i.test(continueChoice)) {
      log.info('Aborting setup to allow selecting another port.');
      return;
    }
  }

  // 3. Max upload size
  const maxMbInput = await askQuestion('Enter Maximum Upload File Size in MB', String(config.maxFileMb));
  const maxMb = parseInt(maxMbInput, 10);
  config.maxFileMb = isNaN(maxMb) || maxMb <= 0 ? DEFAULT_MAX_FILE_MB : maxMb;

  // 4. Data directory
  config.dataDir = await askQuestion('Enter Persistent Storage directory', config.dataDir);

  // Summary confirmation
  console.log(`\n${c.yellow}------------------------------------------------------------${c.reset}`);
  console.log(`${c.bold}Configuration Summary:${c.reset}`);
  console.log(`  Host           : ${config.host}`);
  console.log(`  Port           : ${config.port}`);
  console.log(`  Storage Path   : ${config.dataDir}`);
  console.log(`  Max File Size  : ${config.maxFileMb} MB`);
  console.log(`  Secure Cookie  : ${config.secureCookie}`);
  console.log(`${c.yellow}------------------------------------------------------------${c.reset}\n`);

  const proceed = await askQuestion('Proceed with installation? [Y/n]', 'Y');
  if (/^n(o)?$/i.test(proceed)) {
    console.log('Installation cancelled.');
    return;
  }

  config.forceOverwrite = true;
  await executeInstallation(config);
}

// ------------------------------------------------------------------------------
// Plesk Preset Mode
// ------------------------------------------------------------------------------
async function runPleskPreset() {
  printBanner();
  console.log(`${c.cyan}${c.bold}=== Plesk / Nginx Reverse Proxy Setup Preset ===${c.reset}\n`);
  config.host = '127.0.0.1';
  config.port = 8080;
  config.secureCookie = true;
  config.forceOverwrite = true;

  log.info('Configuring optimized preset for Plesk (127.0.0.1:8080 with HTTPS Secure Cookies)...');
  await executeInstallation(config);

  console.log(`${c.cyan}${c.bold}======================================================================${c.reset}`);
  console.log(`${c.bold}📋 Paste this into your Plesk Domain 'Additional nginx directives':${c.reset}`);
  console.log(`${c.cyan}----------------------------------------------------------------------${c.reset}`);
  console.log(`location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # High speed streaming for 100MB+ uploads & byte-range MP3 audio
    proxy_request_buffering off;
    proxy_buffering off;
    client_max_body_size ${config.maxFileMb}M;
}`);
  console.log(`${c.cyan}======================================================================${c.reset}\n`);
}

// ------------------------------------------------------------------------------
// Live Backup Routine
// ------------------------------------------------------------------------------
function performBackup() {
  checkRoot('performing backup');
  log.info('Initiating live backup of Go File Manager database and storage...');

  fs.mkdirSync(config.backupDir, { recursive: true });

  const timestampStr = new Date()
    .toISOString()
    .replace(/[-:T]/g, '')
    .substring(0, 14);
  const backupFile = path.join(config.backupDir, `filemanager-backup-${timestampStr}.tar.gz`);

  if (!fs.existsSync(config.dataDir)) {
    log.fatal(`Data directory ${config.dataDir} does not exist. Nothing to backup.`);
  }

  log.info(`Compressing ${config.dataDir}/data and ${config.dataDir}/storage...`);

  // Build tar command
  const tarRes = run('tar', ['-czf', backupFile, '-C', config.dataDir, 'data', 'storage']);
  if (tarRes.status !== 0) {
    log.fatal(`Backup archiving failed: ${tarRes.stderr}`);
  }

  fs.chmodSync(backupFile, 0o600);

  // Compute size and checksum
  const stats = fs.statSync(backupFile);
  const sizeMb = (stats.size / (1024 * 1024)).toFixed(2);

  const hash = crypto.createHash('sha256');
  const buffer = fs.readFileSync(backupFile);
  hash.update(buffer);
  const sha256 = hash.digest('hex');

  log.success('Backup archive created successfully!');
  console.log(`  ${c.bold}Archive File${c.reset}   : ${backupFile}`);
  console.log(`  ${c.bold}Archive Size${c.reset}   : ${sizeMb} MB`);
  console.log(`  ${c.bold}SHA256 Checksum${c.reset}: ${sha256}\n`);
}

// ------------------------------------------------------------------------------
// Rebuild / Update Binary
// ------------------------------------------------------------------------------
async function rebuildBinary() {
  checkRoot('rebuilding binary');
  log.info('Rebuilding latest application binary from source...');
  await prepareBinary(__dirname, config.binPath);

  if (isSystemdActive()) {
    log.info(`Restarting ${APP_NAME} service...`);
    try {
      run('systemctl', ['restart', `${APP_NAME}.service`]);
      log.success('Service restarted with updated binary.');
    } catch (err) {
      log.warn(`Could not restart service: ${err.message}`);
    }
  }
}

// ------------------------------------------------------------------------------
// Service Status & Live Logs Viewer
// ------------------------------------------------------------------------------
function viewStatusAndLogs() {
  checkRoot('inspecting service status');
  printBanner();
  console.log(`${c.bold}--- Systemd Service Status ---${c.reset}`);
  try {
    run('systemctl', ['status', APP_NAME, '--no-pager'], { stdio: 'inherit' });
  } catch {}

  console.log(`\n${c.bold}--- Last 25 Journal Log Entries ---${c.reset}`);
  try {
    run('journalctl', ['-u', APP_NAME, '-n', '25', '--no-pager'], { stdio: 'inherit' });
  } catch {}
}

// ------------------------------------------------------------------------------
// View & Edit Configuration
// ------------------------------------------------------------------------------
async function manageConfig() {
  checkRoot('viewing/editing configuration');
  if (!fs.existsSync(config.envFile)) {
    log.warn(`No configuration file found at ${config.envFile}`);
    return;
  }

  console.log(`\n${c.cyan}${c.bold}Current Configuration (${config.envFile}):${c.reset}`);
  console.log(`${c.yellow}------------------------------------------------------------${c.reset}`);
  console.log(fs.readFileSync(config.envFile, 'utf8'));
  console.log(`${c.yellow}------------------------------------------------------------${c.reset}\n`);

  const edit = await askQuestion('Open in nano or system text editor? [y/N]', 'N');
  if (/^y(es)?$/i.test(edit)) {
    const editor = process.env.EDITOR || (commandExists('nano') ? 'nano' : 'vi');
    child_process.spawnSync(editor, [config.envFile], { stdio: 'inherit' });
    if (isSystemdActive()) {
      run('systemctl', ['restart', `${APP_NAME}.service`]);
      log.success('Service restarted with updated configuration.');
    }
  }
}

// ------------------------------------------------------------------------------
// Pre-Flight System Diagnostics
// ------------------------------------------------------------------------------
async function runDiagnostics() {
  printBanner();
  console.log(`${c.bold}${c.cyan}=== Pre-Flight Diagnostics Check ===${c.reset}\n`);

  console.log(`${c.bold}Operating System & Environment:${c.reset}`);
  console.log(`  Platform         : ${os.platform()} (${os.type()} ${os.release()})`);
  console.log(`  Architecture     : ${os.arch()}`);
  console.log(`  Total Memory     : ${(os.totalmem() / (1024 * 1024 * 1024)).toFixed(2)} GB`);
  console.log(`  Free Memory      : ${(os.freemem() / (1024 * 1024 * 1024)).toFixed(2)} GB`);
  console.log(`  Node.js Version  : ${process.version}`);
  console.log(`  Root Privileges  : ${isRoot() ? `${c.green}YES (uid 0)${c.reset}` : `${c.yellow}NO (non-root)${c.reset}`}`);
  console.log(`  Systemd Active   : ${isSystemdActive() ? `${c.green}YES${c.reset}` : `${c.yellow}NO (Container/WSL/Other)${c.reset}`}`);

  const diskMb = checkDiskSpace(config.dataDir);
  console.log(`  Disk Space Free  : ${diskMb} MB on target partition`);

  console.log(`\n${c.bold}Toolchain & Binaries:${c.reset}`);
  const hasGo = commandExists('go');
  let goVer = 'Not found';
  if (hasGo) {
    try {
      goVer = run('go', ['version']).stdout.trim();
    } catch {}
  }
  console.log(`  Go Compiler      : ${hasGo ? `${c.green}${goVer}${c.reset}` : `${c.yellow}Not found in PATH${c.reset}`}`);

  const localBin = path.join(__dirname, 'filemanager');
  if (fs.existsSync(localBin)) {
    const type = detectBinaryType(localBin);
    console.log(`  Local Binary     : Found (${type})`);
  } else {
    console.log(`  Local Binary     : None (will compile from Go source)`);
  }

  console.log(`\n${c.bold}Network Ports:${c.reset}`);
  const port8080InUse = await isPortInUse(8080);
  console.log(`  Port 8080        : ${port8080InUse ? `${c.yellow}IN USE${c.reset}` : `${c.green}AVAILABLE${c.reset}`}`);

  console.log(`\n${c.bold}Installed Service:${c.reset}`);
  const serviceInstalled = fs.existsSync(config.serviceFile);
  console.log(`  Service File     : ${serviceInstalled ? `${c.green}Installed at ${config.serviceFile}${c.reset}` : 'Not installed'}`);

  console.log(`\n${c.green}Diagnostics scan complete.${c.reset}\n`);
}

// ------------------------------------------------------------------------------
// Clean Uninstallation Routine
// ------------------------------------------------------------------------------
async function performUninstall() {
  checkRoot('uninstallation');
  log.warn(`Starting uninstallation of ${APP_NAME}...`);

  if (isSystemdActive() && fs.existsSync(config.serviceFile)) {
    log.info('Stopping and disabling systemd service...');
    try {
      run('systemctl', ['stop', `${APP_NAME}.service`]);
      run('systemctl', ['disable', `${APP_NAME}.service`]);
      fs.rmSync(config.serviceFile, { force: true });
      run('systemctl', ['daemon-reload']);
      log.success('Systemd service removed.');
    } catch (e) {
      log.warn(`Systemd cleanup error: ${e.message}`);
    }
  }

  if (fs.existsSync(config.binPath)) {
    log.info(`Removing executable binary at ${config.binPath}...`);
    fs.rmSync(config.binPath, { force: true });
  }

  if (fs.existsSync(config.configDir)) {
    log.info(`Removing configuration directory at ${config.configDir}...`);
    fs.rmSync(config.configDir, { recursive: true, force: true });
  }

  console.log('');
  const purgeData = await askQuestion(
    `Do you also want to permanently delete all file storage and database in ${config.dataDir}? [y/N]`,
    'N'
  );

  if (/^y(es)?$/i.test(purgeData)) {
    log.warn(`Purging data directory ${config.dataDir}...`);
    fs.rmSync(config.dataDir, { recursive: true, force: true });
    log.success('Data directory deleted.');
  } else {
    log.info(`Data directory preserved at ${config.dataDir}`);
  }

  if (isLinux()) {
    try {
      run('getent', ['passwd', SYSTEM_USER]);
      log.info(`Removing system user '${SYSTEM_USER}'...`);
      run('userdel', [SYSTEM_USER]);
    } catch {}
  }

  log.success('Uninstallation complete.');
  closeReadline();
  process.exit(0);
}

// ------------------------------------------------------------------------------
// Main Interactive Menu Loop
// ------------------------------------------------------------------------------
async function mainMenu() {
  while (true) {
    printBanner();
    console.log(`${c.bold}Select an action:${c.reset}\n`);
    console.log(`  ${c.green}${c.bold}1)${c.reset} 🧙 Step-by-Step Installation Wizard ${c.green}(Recommended)${c.reset}`);
    console.log(`  ${c.cyan}${c.bold}2)${c.reset} ⚡ Express 1-Click Install (Standard Production Defaults)`);
    console.log(`  ${c.magenta}${c.bold}3)${c.reset} 🌐 Plesk / Nginx Reverse-Proxy Preset`);
    console.log(`  ${c.blue}${c.bold}4)${c.reset} 🔄 Update / Rebuild Binary (Zero Data Loss)`);
    console.log(`  ${c.yellow}${c.bold}5)${c.reset} 📊 Check Service Status & Recent Live Logs`);
    console.log(`  ${c.cyan}${c.bold}6)${c.reset} ⚙️ View / Edit Configuration`);
    console.log(`  ${c.green}${c.bold}7)${c.reset} 💾 Backup Database & Storage to Archive (.tar.gz)`);
    console.log(`  ${c.blue}${c.bold}8)${c.reset} 🩺 Run Pre-Flight Diagnostics Check`);
    console.log(`  ${c.red}${c.bold}9)${c.reset} 🗑️ Uninstall Service & Application`);
    console.log(`  ${c.bold}0)${c.reset} ❌ Exit\n`);

    const choice = await askQuestion('Enter choice [0-9]', '1');

    switch (choice) {
      case '1':
        await runWizard();
        await waitForEnter();
        break;
      case '2':
        config.port = DEFAULT_PORT;
        config.host = DEFAULT_HOST;
        config.secureCookie = false;
        await executeInstallation(config);
        await waitForEnter();
        break;
      case '3':
        await runPleskPreset();
        await waitForEnter();
        break;
      case '4':
        await rebuildBinary();
        await waitForEnter();
        break;
      case '5':
        viewStatusAndLogs();
        await waitForEnter();
        break;
      case '6':
        await manageConfig();
        await waitForEnter();
        break;
      case '7':
        performBackup();
        await waitForEnter();
        break;
      case '8':
        await runDiagnostics();
        await waitForEnter();
        break;
      case '9':
        await performUninstall();
        break;
      case '0':
      case 'q':
      case 'Q':
        console.log('Exiting installer.');
        closeReadline();
        process.exit(0);
        break;
      default:
        console.log(`${c.red}Invalid selection. Please choose a number from 0 to 9.${c.reset}`);
        await new Promise((r) => setTimeout(r, 1000));
        break;
    }
  }
}

// ------------------------------------------------------------------------------
// CLI Flag Parser
// ------------------------------------------------------------------------------
async function parseCliArgs() {
  const args = process.argv.slice(2);

  if (args.length === 0) {
    if (process.stdin.isTTY) {
      await mainMenu();
      return;
    } else {
      // Non-interactive automated execution fallback
      await executeInstallation(config);
      return;
    }
  }

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    switch (arg) {
      case '--wizard':
        await runWizard();
        closeReadline();
        return;
      case '--express':
        await executeInstallation(config);
        closeReadline();
        return;
      case '--plesk':
        await runPleskPreset();
        closeReadline();
        return;
      case '--backup':
        performBackup();
        closeReadline();
        return;
      case '--status':
        viewStatusAndLogs();
        closeReadline();
        return;
      case '--diagnostics':
        await runDiagnostics();
        closeReadline();
        return;
      case '--uninstall':
        await performUninstall();
        closeReadline();
        return;
      case '-p':
      case '--port':
        config.port = parseInt(args[++i], 10);
        break;
      case '-h':
      case '--host':
        config.host = args[++i];
        break;
      case '-s':
      case '--max-size-mb':
        config.maxFileMb = parseInt(args[++i], 10);
        break;
      case '-d':
      case '--data-dir':
        config.dataDir = args[++i];
        break;
      case '-b':
      case '--bin-path':
        config.binPath = args[++i];
        break;
      case '--secure-cookie':
        config.secureCookie = args[++i] === 'true';
        break;
      case '--no-service':
        config.skipService = true;
        break;
      case '--force':
        config.forceOverwrite = true;
        break;
      case '--debug':
        config.debugMode = true;
        break;
      case '--dry-run':
        config.dryRun = true;
        break;
      case '-v':
      case '--version':
        console.log(`Go File Manager Installer ${APP_VERSION}`);
        process.exit(0);
        break;
      case '--help':
        printBanner();
        console.log(`Usage:`);
        console.log(`  Interactive Menu : sudo node install.js`);
        console.log(`  Step Wizard      : sudo node install.js --wizard`);
        console.log(`  Express Setup    : sudo node install.js --express`);
        console.log(`  Plesk Preset     : sudo node install.js --plesk`);
        console.log(`  Diagnostics      : node install.js --diagnostics`);
        console.log(`  Backup Archive   : sudo node install.js --backup`);
        console.log(`  Uninstall        : sudo node install.js --uninstall`);
        console.log(`  Custom Flags     : sudo node install.js --port 9000 --host 127.0.0.1 --max-size-mb 200`);
        process.exit(0);
        break;
      default:
        log.error(`Unknown argument: ${arg}`);
        console.log(`Run 'node install.js --help' for available flags.`);
        process.exit(1);
    }
  }

  await executeInstallation(config);
  closeReadline();
}

// ------------------------------------------------------------------------------
// Entrypoint Execution
// ------------------------------------------------------------------------------
parseCliArgs().catch((err) => {
  log.fatal(`Unexpected error: ${err.message}`);
});
