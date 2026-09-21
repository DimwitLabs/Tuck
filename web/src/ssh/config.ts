import type { CredentialItem, HostItem } from "../types";

export const TUCK_DIR = "~/.ssh/tuck";

export function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "key";
}

const ALIAS = /^[A-Za-z0-9_][A-Za-z0-9_-]*$/;
const HOSTNAME = /^[A-Za-z0-9._:%][A-Za-z0-9._:%-]*$/;
const USER = /^[A-Za-z0-9._@\\][A-Za-z0-9._@\\-]*$/;
const JUMP = /^[A-Za-z0-9._@:[\]][A-Za-z0-9._@:,[\]-]*$/;
const OPTION = /^([A-Za-z][A-Za-z0-9]*)\s+(\S.*)$/;
const SAFE_OPTIONS = new Set(
  [
    "AddKeysToAgent", "AddressFamily", "BatchMode", "CheckHostIP", "Ciphers", "Compression", "ConnectionAttempts",
    "ConnectTimeout", "EscapeChar", "HostKeyAlgorithms", "IdentitiesOnly", "IPQoS", "KbdInteractiveAuthentication",
    "KexAlgorithms", "LogLevel", "MACs", "NumberOfPasswordPrompts", "PasswordAuthentication", "PreferredAuthentications",
    "PubkeyAcceptedAlgorithms", "PubkeyAuthentication", "RekeyLimit", "RequestTTY", "ServerAliveCountMax",
    "ServerAliveInterval", "TCPKeepAlive", "VisualHostKey",
  ].map((o) => o.toLowerCase()),
);

export interface KeyFile {
  credentialId: string;
  file: string;
  privateKey: string;
  publicKey?: string;
}

export interface RenderedConfig {
  config: string;
  keys: KeyFile[];
  hosts: { name: string; aliases: string[] }[];
  warnings: string[];
}

export function aliasesFor(host: HostItem): string[] {
  return host.aliases.length ? host.aliases : [slugify(host.name)];
}

export function renderConfig(
  hosts: { id: string; data: HostItem }[],
  credentials: { id: string; data: CredentialItem }[],
): RenderedConfig {
  const warnings: string[] = [];
  const keys: KeyFile[] = [];
  const keyFileById = new Map<string, string>();
  const taken = new Set<string>();

  for (const { id, data } of credentials) {
    if (!data.privateKey?.trim()) continue;
    let file = slugify(data.name);
    for (let n = 2; taken.has(file); n++) file = `${slugify(data.name)}-${n}`;
    taken.add(file);
    keyFileById.set(id, file);
    keys.push({ credentialId: id, file, privateKey: data.privateKey, publicKey: data.publicKey });
  }

  const credentialById = new Map(credentials.map((c) => [c.id, c.data]));
  const seenAliases = new Set<string>();
  const blocks: string[] = [];
  const rendered: RenderedConfig["hosts"] = [];

  for (const { data: host } of hosts) {
    const skip = (why: string) => warnings.push(`skipped ${host.name}: ${why}`);
    const aliases = aliasesFor(host);
    const credential = host.credentialId ? credentialById.get(host.credentialId) : undefined;
    const user = host.user?.trim() || credential?.username?.trim();

    const badAlias = aliases.find((a) => !ALIAS.test(a));
    if (badAlias) { skip(`alias "${badAlias}" may only contain letters, digits, dash and underscore, with no dots, so it can't stand in for a real host name`); continue; }
    const duplicate = aliases.find((a) => seenAliases.has(a));
    if (duplicate) { skip(`alias "${duplicate}" is already used by another host`); continue; }
    if (!HOSTNAME.test(host.hostname)) { skip(`"${host.hostname}" is not a valid hostname`); continue; }
    if (host.port !== undefined && !(Number.isInteger(host.port) && host.port > 0 && host.port < 65536)) { skip(`port ${host.port} is out of range`); continue; }
    if (user && !USER.test(user)) { skip(`user "${user}" contains characters ssh_config cannot hold`); continue; }
    if (host.proxyJump && !JUMP.test(host.proxyJump)) { skip(`jump host "${host.proxyJump}" is not valid`); continue; }

    const lines = [`Host ${aliases.join(" ")}`, `  HostName ${host.hostname}`];
    if (host.port && host.port !== 22) lines.push(`  Port ${host.port}`);
    if (user) lines.push(`  User ${user}`);
    const keyFile = host.credentialId ? keyFileById.get(host.credentialId) : undefined;
    if (keyFile) {
      lines.push(`  IdentityFile ${TUCK_DIR}/keys/${keyFile}`, "  IdentitiesOnly yes");
    } else if (host.credentialId) {
      warnings.push(`${host.name} uses a key with no private half, so ssh will fall back to your agent`);
    }
    if (host.proxyJump) lines.push(`  ProxyJump ${host.proxyJump}`);

    let badOption = false;
    for (const raw of (host.options ?? "").split("\n").map((l) => l.trim()).filter((l) => l && !l.startsWith("#"))) {
      const m = raw.match(OPTION);
      if (!m) { skip(`option "${raw}" is not a single "keyword value" line`); badOption = true; break; }
      if (!SAFE_OPTIONS.has(m[1].toLowerCase())) { skip(`tuck doesn't write ${m[1]}; it only writes options that tune a connection, not ones that run things, forward things or relax host-key checks`); badOption = true; break; }
      lines.push(`  ${m[1]} ${m[2]}`);
    }
    if (badOption) continue;

    aliases.forEach((a) => seenAliases.add(a));
    rendered.push({ name: host.name, aliases });
    blocks.push(lines.join("\n"));
  }

  const header = "# Managed by Tuck. This file is replaced on every install; edit hosts in Tuck instead.\n";
  return { config: `${header}\n${blocks.join("\n\n")}\n`, keys, hosts: rendered, warnings };
}
