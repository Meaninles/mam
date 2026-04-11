import { getRuntimeConfig } from './runtimeConfig';

export type ImportDashboard = {
  libraries: Array<{ id: string; name: string }>;
  devices: any[];
  drafts: any[];
  reports: any[];
  targetEndpoints: any[];
};

export type ImportBrowseResponse = {
  sessionId: string;
  currentPath: string;
  items: any[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
};

export const importsApi = {
  async loadDashboard() {
    return fetchImportsData<ImportDashboard>('/api/import-center');
  },

  async refreshDashboard() {
    return fetchImportsData<ImportDashboard>('/api/import-center/refresh', { method: 'POST' });
  },

  async setDraftLibrary(draftId: string, libraryId: string) {
    return fetchImportsData<{ message: string }>(`/api/import-drafts/${draftId}/library`, {
      method: 'PATCH',
      body: JSON.stringify({ libraryId }),
    });
  },

  async applyTargetToAll(sessionId: string, targetId: string) {
    return fetchImportsData<{ message: string }>(`/api/import-sessions/${sessionId}/targets/${targetId}/apply-all`, {
      method: 'POST',
    });
  },

  async removeTargetFromAll(sessionId: string, targetId: string) {
    return fetchImportsData<{ message: string }>(`/api/import-sessions/${sessionId}/targets/${targetId}/remove-all`, {
      method: 'POST',
    });
  },

  async setSourceTargets(sourceNodeId: string, targetEndpointIds: string[]) {
    return fetchImportsData<{ message: string }>(`/api/import-sessions/${sourceNodeId}/selections`, {
      method: 'PATCH',
      body: JSON.stringify(targetEndpointIds),
    });
  },

  async browseSession(sessionId: string, params: { path?: string; limit?: number; offset?: number } = {}) {
    const query = new URLSearchParams();
    if (params.path) query.set('path', params.path);
    if (typeof params.limit === 'number') query.set('limit', String(params.limit));
    if (typeof params.offset === 'number') query.set('offset', String(params.offset));
    return fetchImportsData<ImportBrowseResponse>(
      `/api/import-sessions/${sessionId}/browse${query.toString() ? `?${query.toString()}` : ''}`,
    );
  },

  async saveSelectionTargets(
    sessionId: string,
    payload: { entryType: string; name: string; relativePath: string; targetEndpointIds: string[] },
  ) {
    return fetchImportsData<{ message: string }>(`/api/import-sessions/${sessionId}/selections`, {
      method: 'PATCH',
      body: JSON.stringify(payload),
    });
  },

  async refreshPrecheck(draftId: string) {
    return fetchImportsData<{ message: string }>(`/api/import-drafts/${draftId}/precheck`, { method: 'POST' });
  },

  async submit(sessionId: string) {
    return fetchImportsData<{ message: string; reportId: string; report: any }>(`/api/import-sessions/${sessionId}/submit`, {
      method: 'POST',
    });
  },
};

async function fetchImportsData<T>(path: string, init?: RequestInit): Promise<T> {
  const { centerBaseUrl } = getRuntimeConfig();
  const response = await fetch(`${centerBaseUrl}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(body?.error?.message ?? `HTTP ${response.status}`);
  }

  const body = (await response.json()) as { data: T };
  return body.data;
}
