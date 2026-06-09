import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ARTIFACTS_DIR =
  process.env.E2E_ARTIFACTS_DIR ??
  path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "artifacts");

export interface CompiledArtifact {
  abi: any[];
  bytecode: string;
  deployedBytecode: string;
}

// loadArtifact reads one of the bundled-during-setup contract artifacts.
// scripts/e2e/fetch-artifacts.sh writes these into e2e/artifacts/.
export function loadArtifact(name: string): CompiledArtifact {
  const p = path.join(ARTIFACTS_DIR, `${name}.json`);
  if (!fs.existsSync(p)) {
    throw new Error(
      `artifact not found: ${p}. Run "make e2e-fetch-artifacts" first.`,
    );
  }
  const raw = JSON.parse(fs.readFileSync(p, "utf8"));
  return {
    abi: raw.abi,
    bytecode: typeof raw.bytecode === "string" ? raw.bytecode : raw.bytecode.object,
    deployedBytecode:
      typeof raw.deployedBytecode === "string"
        ? raw.deployedBytecode
        : raw.deployedBytecode?.object ?? "0x",
  };
}
