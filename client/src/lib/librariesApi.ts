import type { Library } from '../data';
import { getRuntimeConfig } from './runtimeConfig';

export async function loadLibraries(): Promise<Library[]> {
  const { centerBaseUrl } = getRuntimeConfig();
  const response = await fetch(`${centerBaseUrl}/api/libraries`);
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message ?? `center service returned status ${response.status}`);
  }

  const payload = (await response.json()) as { data?: unknown };
  if (!Array.isArray(payload.data)) {
    throw new Error('资产库响应格式无效');
  }

  return payload.data
    .map((item) => {
      const record = item as Record<string, unknown>;
      return {
        id: String(record.id ?? ''),
        name: String(record.name ?? ''),
        rootLabel: String(record.rootLabel ?? '/'),
        itemCount: String(record.itemCount ?? '0'),
        health: String(record.health ?? '—'),
        storagePolicy: String(record.storagePolicy ?? '—'),
      } satisfies Library;
    })
    .filter((item) => item.id !== '' && item.name !== '');
}

export async function createLibrary(name: string): Promise<{ message: string; library: Library }> {
  const { centerBaseUrl } = getRuntimeConfig();
  const response = await fetch(`${centerBaseUrl}/api/libraries`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ name }),
  });
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message ?? `center service returned status ${response.status}`);
  }

  const payload = (await response.json()) as { data?: { message?: string; library?: unknown } };
  const library = normalizeLibraryRecord(payload.data?.library);
  return {
    message: String(payload.data?.message ?? '资产库已创建'),
    library,
  };
}

function normalizeLibraryRecord(input: unknown): Library {
  const record = (input ?? {}) as Record<string, unknown>;
  return {
    id: String(record.id ?? ''),
    name: String(record.name ?? ''),
    rootLabel: String(record.rootLabel ?? '/'),
    itemCount: String(record.itemCount ?? '0'),
    health: String(record.health ?? '—'),
    storagePolicy: String(record.storagePolicy ?? '—'),
  };
}
