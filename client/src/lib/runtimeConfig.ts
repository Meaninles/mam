export type RuntimeConfig = {
  centerBaseUrl: string;
  runtimeStatusEnabled: boolean;
  runtimeStatusPollMs: number;
};

declare global {
  interface Window {
    __MARE_RUNTIME_CONFIG__?: Partial<RuntimeConfig>;
  }
}

export function getRuntimeConfig(): RuntimeConfig {
  const override = typeof window !== 'undefined' ? window.__MARE_RUNTIME_CONFIG__ : undefined;
  const centerBaseUrl = normalizeBaseUrl(
    override?.centerBaseUrl ?? import.meta.env.VITE_CENTER_BASE_URL ?? 'http://127.0.0.1:8080',
  );

  return {
    centerBaseUrl,
    runtimeStatusEnabled: override?.runtimeStatusEnabled ?? (import.meta.env.MODE !== 'test'),
    runtimeStatusPollMs: override?.runtimeStatusPollMs ?? 15000,
  };
}

function normalizeBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}
