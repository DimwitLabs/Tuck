export type ItemKind = "credential" | "host" | "file";

export interface CredentialItem {
  name: string;
  tags: string[];
  username?: string;
  password?: string;
  privateKey?: string;
  publicKey?: string;
  passphrase?: string;
  keyType?: string;
  fingerprint?: string;
  notes?: string;
}

export interface HostItem {
  name: string;
  aliases: string[];
  hostname: string;
  port?: number;
  user?: string;
  credentialId?: string;
  proxyJump?: string;
  tags: string[];
  notes?: string;
  options?: string;
}

export interface FileItem {
  name: string;
  tags: string[];
  size: number;
  mime: string;
  notes?: string;
}

export interface Item<T = CredentialItem | HostItem | FileItem> {
  id: string;
  kind: ItemKind;
  data: T;
  updatedAt: string;
}
