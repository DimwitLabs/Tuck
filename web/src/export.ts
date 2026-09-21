import { toBase64 } from "./crypto/encoding";
import type { CredentialItem, FileItem, HostItem } from "./types";

export interface ExportFile {
  version: 1;
  exportedAt: string;
  credentials: (CredentialItem & { ref: string })[];
  hosts: (Omit<HostItem, "credentialId"> & { credentialRef?: string })[];
  files: (FileItem & { contentBase64: string })[];
}

export async function exportVault(
  credentials: { id: string; data: CredentialItem }[],
  hosts: { id: string; data: HostItem }[],
  files: { id: string; data: FileItem }[],
  readFile: (id: string) => Promise<Uint8Array>,
): Promise<ExportFile> {
  return {
    version: 1,
    exportedAt: new Date().toISOString(),
    credentials: credentials.map(({ id, data }) => ({ ref: id, ...data })),
    hosts: hosts.map(({ data: { credentialId, ...rest } }) => ({ ...rest, credentialRef: credentialId })),
    files: await Promise.all(files.map(async ({ id, data }) => ({ ...data, contentBase64: toBase64(await readFile(id)) }))),
  };
}
