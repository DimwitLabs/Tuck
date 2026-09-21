import { randomBytes } from "../crypto/encoding";
import type { CredentialItem, HostItem } from "../types";
import { renderConfig, TUCK_DIR } from "./config";

export interface Installer {
  script: string;
  hostCount: number;
  keyCount: number;
  warnings: string[];
}

const hex = (n: number) => [...randomBytes(n)].map((b) => b.toString(16).padStart(2, "0")).join("");

export function buildInstaller(
  hosts: { id: string; data: HostItem }[],
  credentials: { id: string; data: CredentialItem }[],
  generatedAt = new Date(),
): Installer {
  const rendered = renderConfig(hosts, credentials);

  const everything = [rendered.config, ...rendered.keys.flatMap((k) => [k.privateKey, k.publicKey ?? ""])].join("\n");
  let eof = `TUCK_${hex(8)}`;
  while (everything.includes(eof)) eof = `TUCK_${hex(8)}`;

  const heredoc = (path: string, body: string, mode: string) =>
    `cat > "$stage/${path}" <<'${eof}'\n${body.replace(/\n*$/, "")}\n${eof}\nchmod ${mode} "$stage/${path}"`;

  const keyWrites = rendered.keys.flatMap((k) => [
    heredoc(`keys/${k.file}`, k.privateKey, "600"),
    ...(k.publicKey ? [heredoc(`keys/${k.file}.pub`, k.publicKey, "644")] : []),
  ]);

  const summary = rendered.hosts.map((h) => `echo "  ssh ${h.aliases[0]}${h.aliases.length > 1 ? `   (also: ${h.aliases.slice(1).join(", ")})` : ""}"`);

  const script = `#!/bin/sh
# Tuck SSH installer, generated ${generatedAt.toISOString()}
#
# This file contains private keys. It deletes itself once it has installed them.
#
#   sh tuck-install.sh            install or update ${TUCK_DIR}
#   sh tuck-install.sh --remove   remove ${TUCK_DIR} and its Include line
#
# Everything Tuck manages lives in ${TUCK_DIR}. The only change outside it is one Include line at the top of ~/.ssh/config, and a backup is taken first.
set -eu
umask 077

SSH_DIR="$HOME/.ssh"
TUCK_DIR="$SSH_DIR/tuck"
SSH_CONFIG="$SSH_DIR/config"
INCLUDE_LINE="Include ${TUCK_DIR}/config"

BACKUP=""
backup_config() {
  if [ -f "$SSH_CONFIG" ]; then
    BACKUP="$SSH_CONFIG.tuck-backup.$(date +%Y%m%d%H%M%S)"
    cp -p "$SSH_CONFIG" "$BACKUP"
  fi
}

restore_config() {
  if [ -n "$BACKUP" ]; then
    cat "$BACKUP" > "$SSH_CONFIG"
  elif [ ! -L "$SSH_CONFIG" ]; then
    rm -f "$SSH_CONFIG"
  fi
  rm -f "$SSH_CONFIG.tuck-tmp"
}

strip_include() {
  awk -v inc="$INCLUDE_LINE" 'skip && $0 == "" { skip = 0; next } { skip = 0 } $0 == inc { skip = 1; next } { print }' "$SSH_CONFIG"
}

replace_config() {
  trap restore_config EXIT
  trap 'exit 1' INT TERM
  cat "$SSH_CONFIG.tuck-tmp" > "$SSH_CONFIG"
  trap - EXIT INT TERM
  rm -f "$SSH_CONFIG.tuck-tmp"
  chmod 600 "$SSH_CONFIG"
}

if [ "\${1:-}" = "--remove" ]; then
  if [ -f "$SSH_CONFIG" ] && grep -qxF "$INCLUDE_LINE" "$SSH_CONFIG"; then
    backup_config
    strip_include > "$SSH_CONFIG.tuck-tmp"
    replace_config
    if [ ! -L "$SSH_CONFIG" ] && ! grep -q '[^[:space:]]' "$SSH_CONFIG"; then
      rm -f "$SSH_CONFIG"
    fi
  fi
  rm -rf "$TUCK_DIR" "$TUCK_DIR.previous" "$SSH_DIR"/.tuck-stage.*
  echo "Removed $TUCK_DIR and its Include line."
  exit 0
fi

mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"

stage=$(mktemp -d "$SSH_DIR/.tuck-stage.XXXXXX")
cleanup() {
  rm -rf "$stage"
  if [ ! -d "$TUCK_DIR" ] && [ -d "$TUCK_DIR.previous" ]; then
    mv "$TUCK_DIR.previous" "$TUCK_DIR"
  fi
}
trap cleanup EXIT
trap 'exit 1' INT TERM
mkdir "$stage/keys"
chmod 700 "$stage" "$stage/keys"

${keyWrites.join("\n\n")}

${heredoc("config", rendered.config, "600")}

rm -rf "$TUCK_DIR.previous"
if [ -d "$TUCK_DIR" ]; then
  mv "$TUCK_DIR" "$TUCK_DIR.previous"
fi
mv "$stage" "$TUCK_DIR"
trap - EXIT INT TERM
rm -rf "$TUCK_DIR.previous"

if [ ! -f "$SSH_CONFIG" ] || [ "$(head -n 1 "$SSH_CONFIG")" != "$INCLUDE_LINE" ]; then
  backup_config
  {
    printf '%s\\n\\n' "$INCLUDE_LINE"
    if [ -f "$SSH_CONFIG" ]; then strip_include; fi
  } > "$SSH_CONFIG.tuck-tmp"
  replace_config
fi

echo "Tuck installed ${rendered.keys.length} key(s) and ${rendered.hosts.length} host(s) into $TUCK_DIR."
${summary.join("\n")}

case "$(basename -- "$0")" in
  tuck-install*.sh)
    if [ -f "$0" ]; then
      rm -f -- "$0"
      echo "Deleted $0, since it held your private keys."
    fi
    ;;
  *)
    echo "Delete this script yourself now: it holds your private keys."
    ;;
esac
`;

  return { script, hostCount: rendered.hosts.length, keyCount: rendered.keys.length, warnings: rendered.warnings };
}
