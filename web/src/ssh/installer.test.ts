import { execFileSync } from "node:child_process";
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import type { CredentialItem, HostItem } from "../types";
import { buildInstaller } from "./installer";
import { generateKey } from "./keygen";

const root = mkdtempSync(join(tmpdir(), "tuck-install-"));
afterAll(() => rmSync(root, { recursive: true, force: true }));

const home = join(root, "home");
const sshDir = join(home, ".ssh");
const tuckDir = join(sshDir, "tuck");
const run = (script: string, ...args: string[]) =>
  execFileSync("sh", ["-c", script, "tuck-install.sh", ...args], { env: { PATH: process.env.PATH!, HOME: home }, encoding: "utf8" });
const mode = (p: string) => (statSync(p).mode & 0o777).toString(8);

function resolve(alias: string): Record<string, string> {
  const out = execFileSync("ssh", ["-G", "-F", join(tuckDir, "config"), alias], { encoding: "utf8" });
  return Object.fromEntries(out.trim().split("\n").map((l) => [l.split(" ")[0], l.slice(l.indexOf(" ") + 1)]));
}

let credentials: { id: string; data: CredentialItem }[];
let hosts: { id: string; data: HostItem }[];

beforeAll(async () => {
  const workshop = await generateKey("ed25519", "workshop");
  const db = await generateKey("ed25519", "db");
  credentials = [
    { id: "c1", data: { name: "Workshop", tags: [], username: "alex", ...workshop } },
    { id: "c2", data: { name: "Database box", tags: [], username: "alex", ...db } },
    { id: "c3", data: { name: "Password only", tags: [], username: "admin", password: "x" } },
  ];
  hosts = [
    { id: "h1", data: { name: "Workshop", aliases: ["workshop", "shop"], hostname: "gw.example.com", port: 2201, credentialId: "c1", tags: [], options: "ServerAliveInterval 30" } },
    { id: "h2", data: { name: "DB 1", aliases: [], hostname: "gw.example.com", port: 2202, credentialId: "c2", tags: [] } },
    { id: "h3", data: { name: "Behind the gateway", aliases: ["inner"], hostname: "10.0.0.5", user: "root", proxyJump: "db-1", tags: [] } },
    { id: "h4", data: { name: "Evil newline", aliases: ["evil"], hostname: "a.com\n  ProxyCommand sh", tags: [] } },
    { id: "h5", data: { name: "Evil option", aliases: ["evil2"], hostname: "b.com", options: "Host *\n  ProxyCommand x", tags: [] } },
    { id: "h6", data: { name: "Dup", aliases: ["shop"], hostname: "c.com", tags: [] } },
    { id: "h7", data: { name: "Runs things", aliases: ["runs"], hostname: "d.com", options: "ProxyCommand nc %h %p", tags: [] } },
    { id: "h8", data: { name: "Fake GitHub", aliases: ["github.com"], hostname: "evil.example", tags: [] } },
    { id: "h9", data: { name: "Agent thief", aliases: ["thief"], hostname: "e.com", options: "ForwardAgent yes", tags: [] } },
    { id: "h10", data: { name: "Library loader", aliases: ["lib"], hostname: "f.com", options: "pkcs11provider /tmp/x.so", tags: [] } },
    { id: "h11", data: { name: "Flag host", aliases: ["flag"], hostname: "-oProxyCommand=x", tags: [] } },
    { id: "h12", data: { name: "Sneaky jump", aliases: ["jumpy"], hostname: "g.com", options: "ProxyJump -oProxyCommand=x", tags: [] } },
  ];

  mkdirSync(sshDir, { recursive: true });
  writeFileSync(join(sshDir, "config"), "Host *\n  AddKeysToAgent yes\n", { mode: 0o600 });
});

describe("installer", () => {
  it("installs keys with strict permissions and prepends the Include line", () => {
    const { script, warnings, hostCount, keyCount } = buildInstaller(hosts, credentials);
    expect(hostCount).toBe(3);
    expect(keyCount).toBe(2);
    expect(warnings).toHaveLength(9);
    expect(warnings.find((w) => w.startsWith("skipped Runs things:"))).toMatch(/doesn't write ProxyCommand/);
    expect(warnings.find((w) => w.startsWith("skipped Fake GitHub:"))).toMatch(/no dots/);

    const out = run(script);
    expect(out).toContain("ssh workshop   (also: shop)");

    expect(mode(tuckDir)).toBe("700");
    expect(mode(join(tuckDir, "keys", "workshop"))).toBe("600");
    expect(mode(join(tuckDir, "keys", "workshop.pub"))).toBe("644");
    expect(readFileSync(join(tuckDir, "keys", "workshop"), "utf8")).toBe(credentials[0].data.privateKey);

    const config = readFileSync(join(sshDir, "config"), "utf8");
    expect(config.startsWith("Include ~/.ssh/tuck/config\n\nHost *\n  AddKeysToAgent yes\n")).toBe(true);
    expect(readdirSync(sshDir).some((f) => f.startsWith("config.tuck-backup."))).toBe(true);
    expect(readdirSync(sshDir).filter((f) => f.startsWith(".tuck-stage"))).toEqual([]);
  });

  it("resolves every alias the way ssh will actually connect", () => {
    for (const alias of ["workshop", "shop"]) {
      const c = resolve(alias);
      expect([c.hostname, c.port, c.user]).toEqual(["gw.example.com", "2201", "alex"]);
      expect(c.identityfile).toBe("~/.ssh/tuck/keys/workshop");
      expect(c.identitiesonly).toBe("yes");
    }
    const db = resolve("db-1");
    expect([db.port, db.user]).toEqual(["2202", "alex"]);
    const inner = resolve("inner");
    expect([inner.hostname, inner.user, inner.proxyjump]).toEqual(["10.0.0.5", "root", "db-1"]);
  });

  it("never lets a hostile field inject config", () => {
    const config = readFileSync(join(tuckDir, "config"), "utf8");
    expect(config).not.toMatch(/ProxyCommand|ForwardAgent|pkcs11|github\.com|ProxyJump -/i);
    expect(config).not.toContain("evil");
    expect(config).toContain("ServerAliveInterval 30");
    expect(config.match(/^Host /gm)).toHaveLength(3);
  });

  it("is idempotent and drops keys removed from Tuck", () => {
    const { script } = buildInstaller(hosts.slice(0, 1), credentials.slice(0, 1));
    run(script);
    run(script);
    const config = readFileSync(join(sshDir, "config"), "utf8");
    expect(config.match(/^Include ~\/\.ssh\/tuck\/config$/gm)).toHaveLength(1);
    expect(existsSync(join(tuckDir, "keys", "database-box"))).toBe(false);
    expect(existsSync(join(tuckDir, "keys", "workshop"))).toBe(true);
  });

  it("--remove takes out the directory and the Include line, keeping the rest", () => {
    const { script } = buildInstaller(hosts, credentials);
    const out = run(script, "--remove");
    expect(out).toContain("Removed");
    expect(existsSync(tuckDir)).toBe(false);
    expect(readFileSync(join(sshDir, "config"), "utf8")).toBe("Host *\n  AddKeysToAgent yes\n");
  });

  it("works on a machine with no ~/.ssh at all", () => {
    rmSync(sshDir, { recursive: true, force: true });
    run(buildInstaller(hosts, credentials).script);
    expect(mode(sshDir)).toBe("700");
    expect(readFileSync(join(sshDir, "config"), "utf8")).toBe("Include ~/.ssh/tuck/config\n\n");
    run(buildInstaller(hosts, credentials).script, "--remove");
    expect(existsSync(join(sshDir, "config"))).toBe(false);
  });

  it("moves an Include line found lower down to the top", () => {
    writeFileSync(join(sshDir, "config"), "Host *\n  AddKeysToAgent yes\n\nInclude ~/.ssh/tuck/config\n", { mode: 0o600 });
    run(buildInstaller(hosts, credentials).script);
    expect(readFileSync(join(sshDir, "config"), "utf8")).toBe("Include ~/.ssh/tuck/config\n\nHost *\n  AddKeysToAgent yes\n\n");
  });

  it("writes through a symlinked config instead of replacing the link", () => {
    const dotfiles = join(root, "dotfiles");
    mkdirSync(dotfiles, { recursive: true });
    writeFileSync(join(dotfiles, "ssh_config"), "Host *\n  ServerAliveInterval 30\n", { mode: 0o600 });
    rmSync(join(sshDir, "config"), { force: true });
    symlinkSync(join(dotfiles, "ssh_config"), join(sshDir, "config"));

    run(buildInstaller(hosts, credentials).script);
    expect(lstatSync(join(sshDir, "config")).isSymbolicLink()).toBe(true);
    expect(readFileSync(join(dotfiles, "ssh_config"), "utf8")).toBe("Include ~/.ssh/tuck/config\n\nHost *\n  ServerAliveInterval 30\n");

    run(buildInstaller(hosts, credentials).script, "--remove");
    expect(lstatSync(join(sshDir, "config")).isSymbolicLink()).toBe(true);
    expect(readFileSync(join(dotfiles, "ssh_config"), "utf8")).toBe("Host *\n  ServerAliveInterval 30\n");
  });

  it("deletes itself after installing, since it holds private keys", () => {
    const file = join(root, "tuck-install.sh");
    writeFileSync(file, buildInstaller(hosts, credentials).script);
    const out = execFileSync("sh", [file], { env: { PATH: process.env.PATH!, HOME: home }, encoding: "utf8" });
    expect(out).toContain("Deleted");
    expect(existsSync(file)).toBe(false);
    expect(existsSync(join(tuckDir, "keys", "workshop"))).toBe(true);
  });

  it("leaves other files alone when run through a pipe", () => {
    const bystander = join(root, "sh");
    writeFileSync(bystander, "not tuck's");
    const out = execFileSync("sh", ["-s"], { cwd: root, input: buildInstaller(hosts, credentials).script, env: { PATH: process.env.PATH!, HOME: home }, encoding: "utf8" });
    expect(out).toContain("Delete this script yourself");
    expect(readFileSync(bystander, "utf8")).toBe("not tuck's");
  });
});
