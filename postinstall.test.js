"use strict";

const assert = require("node:assert");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { download, nextUrl, expectedHash, getAssetName } = require("./postinstall.js");

const HASH = "a".repeat(64);

function checksums(text) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "dmcp-"));
  const file = path.join(dir, "checksums.txt");
  fs.writeFileSync(file, text);
  return file;
}

test("a redirect to plain HTTP is refused", () => {
  assert.throws(() => nextUrl("https://github.com/x", "http://evil.example/a"), /non-HTTPS/);
});

test("a relative HTTPS redirect is followed", () => {
  assert.strictEqual(nextUrl("https://github.com/a/b", "/c"), "https://github.com/c");
});

test("download refuses a non-HTTPS URL without touching the network", async () => {
  await assert.rejects(download("http://example.com/x", path.join(os.tmpdir(), "never")), /non-HTTPS/);
});

test("expectedHash finds the exact asset", () => {
  const asset = getAssetName("1.2.3");
  const file = checksums(`${"b".repeat(64)}  x${asset}\n${HASH}  ${asset}\n`);
  assert.strictEqual(expectedHash(asset, file), HASH);
});

test("expectedHash fails when the asset is not listed", () => {
  const file = checksums(`${HASH}  something-else.tar.gz\n`);
  assert.throws(() => expectedHash(getAssetName("1.2.3"), file), /no entry/);
});

test("expectedHash fails when checksums.txt is missing", () => {
  assert.throws(() => expectedHash("x", path.join(os.tmpdir(), "does-not-exist", "checksums.txt")), /missing/);
});
