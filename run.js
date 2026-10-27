#!/usr/bin/env node
"use strict";

const { spawn } = require("child_process");
const fs = require("fs");
const { install, BIN_PATH } = require("./postinstall.js");

async function main() {
  if (!fs.existsSync(BIN_PATH)) {
    // npm 12, pnpm and Bun skip postinstall by default, so fetch it now.
    try {
      await install();
    } catch (err) {
      console.error(`duplicacy-mcp: could not install the binary: ${err.message}`);
      process.exit(1);
    }
  }

  const env = Object.assign({}, process.env, { TRANSPORT: "stdio" });
  const child = spawn(BIN_PATH, process.argv.slice(2), { env, stdio: "inherit" });
  for (const sig of ["SIGINT", "SIGTERM"]) {
    process.on(sig, () => child.kill(sig));
  }
  child.on("error", (err) => {
    console.error(`duplicacy-mcp: ${err.message}`);
    process.exit(1);
  });
  child.on("exit", (code) => process.exit(code === null ? 1 : code));
}

main();
