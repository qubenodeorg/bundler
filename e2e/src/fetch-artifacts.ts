// fetch-artifacts.ts downloads the v0.9 contract artifacts the e2e tests need
// (EntryPoint, SimpleAccount, SimpleAccountFactory, Simple7702Account) into
// e2e/artifacts/. It only writes the {abi, bytecode, deployedBytecode} fields,
// stripping the much larger metadata blob the npm package ships with.

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT_DIR = process.env.E2E_ARTIFACTS_DIR ?? path.join(HERE, "..", "artifacts");

const PKG_TGZ =
  "https://registry.npmjs.org/@account-abstraction/contracts/-/contracts-0.9.0-rc.1.tgz";

const TARGETS = [
  "EntryPoint",
  "SimpleAccount",
  "SimpleAccountFactory",
  "Simple7702Account",
];

async function main(): Promise<void> {
  fs.mkdirSync(OUT_DIR, { recursive: true });

  console.error(`fetching ${PKG_TGZ}`);
  const res = await fetch(PKG_TGZ);
  if (!res.ok) {
    throw new Error(`failed to download package: HTTP ${res.status}`);
  }
  const buf = Buffer.from(await res.arrayBuffer());

  // Lazy-require tar-stream-equivalent at runtime to avoid making it a hard
  // dependency. We shell out to system tar — every dev environment has it.
  const tmpTar = path.join(OUT_DIR, ".pkg.tgz");
  const tmpDir = path.join(OUT_DIR, ".pkg");

  try {
    fs.writeFileSync(tmpTar, buf);
    fs.mkdirSync(tmpDir, { recursive: true });
    // execFileSync avoids shell parsing so paths with spaces or metacharacters
    // are passed through verbatim.
    const { execFileSync } = await import("node:child_process");
    execFileSync("tar", ["-xzf", tmpTar, "-C", tmpDir]);

    for (const name of TARGETS) {
      const src = path.join(tmpDir, "package", "artifacts", `${name}.json`);
      if (!fs.existsSync(src)) {
        throw new Error(`expected artifact missing from package: ${src}`);
      }
      const raw = JSON.parse(fs.readFileSync(src, "utf8"));
      const slim = {
        contractName: raw.contractName ?? name,
        abi: raw.abi,
        bytecode: raw.bytecode,
        deployedBytecode: raw.deployedBytecode,
      };
      const dst = path.join(OUT_DIR, `${name}.json`);
      fs.writeFileSync(dst, JSON.stringify(slim, null, 2));
      console.error(`wrote ${dst}`);
    }
  } finally {
    if (fs.existsSync(tmpTar)) fs.rmSync(tmpTar, { force: true });
    if (fs.existsSync(tmpDir)) fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
