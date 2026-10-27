#!/usr/bin/env node
"use strict";

// Downloads the release binary for this platform, verifies its SHA-256
// against the checksums.txt shipped inside this npm package (so the hash is
// covered by npm's provenance, not fetched from the mutable GitHub release),
// and installs it atomically into bin/.
//
// npm runs this as postinstall. npm 12, pnpm and Bun skip install scripts by
// default, so run.js also calls install() on first start. All logging goes
// to stderr because, when run.js calls it, stdout carries MCP messages.

const { execFileSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const path = require("path");
const { pipeline, Transform } = require("stream");

const VERSION = require("./package.json").version;
const REPO = "GeiserX/duplicacy-mcp";
const BIN_NAME = process.platform === "win32" ? "duplicacy-mcp.exe" : "duplicacy-mcp";
const BIN_DIR = path.join(__dirname, "bin");
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);
const CHECKSUMS_PATH = path.join(__dirname, "checksums.txt");

const MAX_REDIRECTS = 5;
const REQUEST_TIMEOUT_MS = 30_000;
const MAX_DOWNLOAD_BYTES = 100 * 1024 * 1024;

const log = (msg) => console.error(msg);

function getPlatformArch() {
  const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };
  const p = platformMap[process.platform];
  const a = archMap[process.arch];
  if (!p || !a) {
    throw new Error(`Unsupported platform: ${process.platform}-${process.arch}`);
  }
  return { platform: p, arch: a };
}

function getAssetName(version = VERSION) {
  const { platform, arch } = getPlatformArch();
  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `duplicacy-mcp_${version}_${platform}_${arch}.${ext}`;
}

// expectedHash returns the SHA-256 that checksums.txt lists for assetName.
function expectedHash(assetName, checksumsPath = CHECKSUMS_PATH) {
  if (!fs.existsSync(checksumsPath)) {
    throw new Error("checksums.txt is missing from this package, so the download cannot be verified");
  }
  for (const line of fs.readFileSync(checksumsPath, "utf8").split(/\r?\n/)) {
    const [hash, name] = line.trim().split(/\s+/);
    if (name === assetName && /^[a-f0-9]{64}$/.test(hash)) {
      return hash;
    }
  }
  throw new Error(`checksums.txt has no entry for ${assetName}`);
}

// nextUrl resolves a redirect and refuses anything that is not HTTPS.
function nextUrl(current, location) {
  const next = new URL(location, current);
  if (next.protocol !== "https:") {
    throw new Error(`refusing a redirect to a non-HTTPS URL (${next.protocol})`);
  }
  return next.toString();
}

// download streams url to dest with a size cap and resolves to its SHA-256.
function download(url, dest, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (!url.startsWith("https://")) {
      return reject(new Error("refusing to download over a non-HTTPS URL"));
    }
    const req = https.get(url, { timeout: REQUEST_TIMEOUT_MS }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        if (redirects >= MAX_REDIRECTS) {
          return reject(new Error(`Too many redirects (>${MAX_REDIRECTS})`));
        }
        let next;
        try {
          next = nextUrl(url, res.headers.location);
        } catch (err) {
          return reject(err);
        }
        return download(next, dest, redirects + 1).then(resolve, reject);
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`Download failed: HTTP ${res.statusCode}`));
      }
      if (Number(res.headers["content-length"]) > MAX_DOWNLOAD_BYTES) {
        res.resume();
        return reject(new Error("Download is larger than the size cap"));
      }
      const hash = crypto.createHash("sha256");
      let size = 0;
      const meter = new Transform({
        transform(chunk, _enc, cb) {
          size += chunk.length;
          if (size > MAX_DOWNLOAD_BYTES) {
            return cb(new Error("Download is larger than the size cap"));
          }
          hash.update(chunk);
          cb(null, chunk);
        },
      });
      pipeline(res, meter, fs.createWriteStream(dest, { mode: 0o600 }), (err) => {
        if (err) {
          return reject(err);
        }
        resolve(hash.digest("hex"));
      });
    });
    req.on("timeout", () => req.destroy(new Error(`Request timed out after ${REQUEST_TIMEOUT_MS}ms`)));
    req.on("error", reject);
  });
}

// extract unpacks only the binary from the archive, without a shell.
function extract(archive, assetName, destDir) {
  if (assetName.endsWith(".zip")) {
    if (process.platform === "win32") {
      execFileSync(
        "powershell",
        [
          "-NoProfile",
          "-NonInteractive",
          "-Command",
          "Expand-Archive -Force -LiteralPath $env:DMCP_ARCHIVE -DestinationPath $env:DMCP_DEST",
        ],
        { stdio: "ignore", env: { ...process.env, DMCP_ARCHIVE: archive, DMCP_DEST: destDir } }
      );
    } else {
      execFileSync("unzip", ["-o", archive, BIN_NAME, "-d", destDir], { stdio: "ignore" });
    }
  } else {
    execFileSync("tar", ["-xzf", archive, "-C", destDir, BIN_NAME], { stdio: "ignore" });
  }
}

async function install() {
  if (fs.existsSync(BIN_PATH)) {
    return;
  }
  const assetName = getAssetName();
  const want = expectedHash(assetName);
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${assetName}`;

  fs.mkdirSync(BIN_DIR, { recursive: true });
  const work = fs.mkdtempSync(path.join(BIN_DIR, ".install-"));
  try {
    const archive = path.join(work, assetName);
    log(`Downloading duplicacy-mcp v${VERSION} for ${process.platform}-${process.arch}...`);
    const got = await download(url, archive);
    if (got !== want) {
      throw new Error(`Checksum mismatch for ${assetName}: expected ${want}, got ${got}`);
    }
    log("SHA-256 checksum verified.");
    extract(archive, assetName, work);
    const extracted = path.join(work, BIN_NAME);
    if (process.platform !== "win32") {
      fs.chmodSync(extracted, 0o755);
    }
    fs.renameSync(extracted, BIN_PATH);
    log(`Installed duplicacy-mcp to ${BIN_PATH}`);
  } finally {
    fs.rmSync(work, { recursive: true, force: true });
  }
}

module.exports = { install, download, nextUrl, expectedHash, getAssetName, BIN_PATH };

if (require.main === module) {
  install().catch((err) => {
    log(`Failed to install duplicacy-mcp: ${err.message}`);
    process.exit(1);
  });
}
