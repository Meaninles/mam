import { getRuntimeConfig } from './runtimeConfig';

export type GatewayRecord = {
  id: string;
  gatewayType: string;
  displayName: string;
  baseUrl: string;
  enabled: boolean;
  runtimeStatus: string;
  lastTestAt?: string;
  lastErrorCode?: string;
  lastErrorMessage?: string;
  hasPassword: boolean;
  username?: string;
};

export type RuntimeComponentRecord = {
  name: string;
  status: string;
  message: string;
  lastCheckedAt?: string;
  lastErrorCode?: string;
  lastErrorMessage?: string;
};

export const integrationsApi = {
  async loadGateways(): Promise<GatewayRecord[]> {
    const response = await fetchIntegrationData<{ items: GatewayRecord[] }>('/api/integrations/gateways');
    return response.items ?? [];
  },

  async saveCD2Gateway(input: {
    baseUrl: string;
    username: string;
    password: string;
    enabled: boolean;
  }): Promise<{ message: string; record: GatewayRecord }> {
    return fetchIntegrationData<{ message: string; record: GatewayRecord }>('/api/integrations/gateways/cd2', {
      method: 'PUT',
      body: JSON.stringify(input),
    });
  },

  async testCD2Gateway(input: {
    baseUrl?: string;
    username?: string;
    password?: string;
    enabled?: boolean;
  }): Promise<{ message: string; record: GatewayRecord }> {
    return fetchIntegrationData<{ message: string; record: GatewayRecord }>('/api/integrations/gateways/cd2/test', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },

  async loadRuntime(): Promise<RuntimeComponentRecord[]> {
    const response = await fetchIntegrationData<{ components: RuntimeComponentRecord[] }>('/api/integrations/runtime');
    return response.components ?? [];
  },
};

async function fetchIntegrationData<T>(path: string, init?: RequestInit): Promise<T> {
  const { centerBaseUrl } = getRuntimeConfig();
  let response: Response;
  try {
    response = await fetch(`${centerBaseUrl}${path}`, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        ...(init?.headers ?? {}),
      },
    });
  } catch (error) {
    if (error instanceof TypeError) {
      throw new Error('无法连接中心服务，请检查中心服务地址或跨域配置');
    }
    throw error;
  }
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message ?? `center service returned status ${response.status}`);
  }
  const payload = (await response.json()) as { data: T };
  return payload.data;
}
