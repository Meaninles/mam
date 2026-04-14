import type { AssetLifecycleState, FileTypeFilter, Severity } from '../data';
import { getRuntimeConfig } from './runtimeConfig';

export type FileCenterStatusFilter = '全部' | '完全同步' | '部分同步' | '未同步';
export type FileCenterSortValue = '修改时间' | '名称' | '大小' | '星级';
export type FileCenterSortDirection = 'asc' | 'desc';
export type FileCenterEndpointType = 'local' | 'nas' | 'cloud' | 'removable';
export type FileCenterEndpointState = '已同步' | '未同步' | '同步中' | '部分同步';
export type FileCenterColorLabel = '无' | '红标' | '黄标' | '绿标' | '蓝标' | '紫标';

export type FileCenterEndpoint = {
  name: string;
  state: FileCenterEndpointState;
  tone: Severity;
  lastSyncAt: string;
  endpointType: FileCenterEndpointType;
};

export type FileCenterIntegrationHealth = {
  cd2Online: boolean;
  aria2Online: boolean;
  cloudAuthReady: boolean;
  cd2Message?: string;
  aria2Message?: string;
  cloudAuthMessage?: string;
};

export type FileCenterSyncRouteType = 'UPLOAD' | 'DOWNLOAD' | 'COPY';
export type FileCenterSyncEngine = 'CD2_REMOTE_UPLOAD' | 'ARIA2' | 'INTERNAL_COPY';

export type FileCenterSyncExecution = {
  routeType: FileCenterSyncRouteType;
  engine: FileCenterSyncEngine;
  summary: string;
  sourceEndpointName: string;
  targetEndpointName: string;
  supportsResume: boolean;
  usesCloud: boolean;
};

export type FileCenterDeleteExecution = {
  summary: string;
  targetEndpointName: string;
  usesCloud: boolean;
};

export type FileCenterActionAvailability<TExecution> = {
  enabled: boolean;
  reason?: string;
  execution?: TExecution;
};

export type FileCenterEntry = {
  id: string;
  libraryId: string;
  parentId: string | null;
  type: 'folder' | 'file';
  lifecycleState: AssetLifecycleState;
  name: string;
  fileKind: FileTypeFilter;
  displayType: string;
  modifiedAt: string;
  createdAt: string;
  size: string;
  path: string;
  sourceLabel: string;
  lastTaskText: string;
  lastTaskTone: Severity;
  rating: number;
  colorLabel: FileCenterColorLabel;
  badges: string[];
  riskTags: string[];
  tags: string[];
  endpoints: FileCenterEndpoint[];
};

export type FileCenterTagSuggestion = {
  id: string;
  name: string;
  count: number;
  groupName: string;
  isPinned: boolean;
  libraryIds: string[];
};

export type ManagedTagGroup = {
  id: string;
  name: string;
  orderIndex: number;
  tagCount: number;
  usedTagCount: number;
};

export type ManagedTag = {
  id: string;
  name: string;
  normalizedName: string;
  groupId: string;
  groupName: string;
  orderIndex: number;
  isPinned: boolean;
  usageCount: number;
  libraryIds: string[];
  linkedLibraryIds: string[];
  outOfScopeUsageCount: number;
  createdAt: string;
  updatedAt: string;
};

export type TagManagementOverview = {
  totalTags: number;
  usedTagCount: number;
  ungroupedTagCount: number;
  crossLibraryTagCount: number;
};

export type TagManagementSnapshot = {
  overview: TagManagementOverview;
  groups: ManagedTagGroup[];
  tags: ManagedTag[];
  libraries: Array<{ id: string; name: string }>;
};

export type FileCenterDirectoryResult = {
  breadcrumbs: Array<{ id: string | null; label: string }>;
  items: FileCenterEntry[];
  total: number;
  currentPathChildren: number;
  endpointNames?: string[];
};

export type FileCenterUploadItem = {
  file: File;
  name: string;
  size: number;
  relativePath?: string;
};

export type FileCenterLoadParams = {
  libraryId: string;
  parentId: string | null;
  page: number;
  pageSize: number;
  searchText: string;
  fileTypeFilter: FileTypeFilter;
  statusFilter: FileCenterStatusFilter;
  sortValue: FileCenterSortValue;
  sortDirection?: FileCenterSortDirection;
  partialSyncEndpointNames?: string[];
};

type EntryRow = {
  id: string;
  library_id: string;
  parent_id: string | null;
  type: 'folder' | 'file';
  lifecycle_state: AssetLifecycleState;
  name: string;
  file_kind: FileTypeFilter;
  display_type: string;
  modified_at: string;
  modified_at_sort: number;
  created_at: string;
  size_label: string;
  size_bytes: number;
  path: string;
  source_label: string;
  notes: string;
  last_task_text: string;
  last_task_tone: Severity;
  rating: number;
  color_label: FileCenterColorLabel;
};

const fileCenterListeners = new Set<() => void>();
const libraryEndpointNamesCache = new Map<string, string[]>();
const fileNameCollator = new Intl.Collator('zh-CN-u-co-pinyin', { numeric: true, sensitivity: 'base' });

export const fileCenterApi = {
  listLibraryEndpointNames(libraryId: string) {
    return [...(libraryEndpointNamesCache.get(libraryId) ?? [])];
  },

  subscribe(listener: () => void) {
    fileCenterListeners.add(listener);
    return () => {
      fileCenterListeners.delete(listener);
    };
  },

  async loadDirectory(params: FileCenterLoadParams): Promise<FileCenterDirectoryResult> {
    const query = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
      searchText: params.searchText,
      fileTypeFilter: params.fileTypeFilter,
      statusFilter: params.statusFilter,
      sortValue: params.sortValue,
      sortDirection: params.sortDirection ?? 'desc',
    });
    if (params.parentId) {
      query.set('parentId', params.parentId);
    }
    for (const endpointName of params.partialSyncEndpointNames ?? []) {
      query.append('partialSyncEndpointName', endpointName);
    }

    const result = await fetchFileCenterData<FileCenterDirectoryResult>(
      `/api/libraries/${params.libraryId}/browse?${query.toString()}`,
    );
    if (!Array.isArray(result.breadcrumbs) || !Array.isArray(result.items)) {
      throw new Error('file center payload shape invalid');
    }
    result.items = result.items.map(normalizeDirectoryAnnotations);
    libraryEndpointNamesCache.set(params.libraryId, [...(result.endpointNames ?? [])]);
    return result;
  },

  async loadEntryDetail(id: string): Promise<FileCenterEntry | null> {
    try {
      const result = await fetchFileCenterData<FileCenterEntry>(`/api/file-entries/${id}`);
      if (!result || typeof result !== 'object' || !('id' in result)) {
        throw new Error('file entry payload shape invalid');
      }
      return normalizeDirectoryAnnotations(result);
    } catch (error) {
      if (error instanceof Error && error.message.includes('404')) {
        return null;
      }
      return null;
    }
  },

  async createFolder(input: {
    libraryId: string;
    parentId: string | null;
    name: string;
  }): Promise<{ message: string; item: FileCenterEntry }> {
    const result = await fetchFileCenterData<{ message: string; entry: FileCenterEntry }>(
      `/api/libraries/${input.libraryId}/directories`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          parentId: input.parentId,
          name: input.name,
        }),
      },
    );
    return {
      message: result.message,
      item: normalizeDirectoryAnnotations(result.entry),
    };
  },

  async scanDirectory(input: { libraryId: string; parentId: string | null }): Promise<{ message: string; jobId?: string }> {
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/libraries/${input.libraryId}/scan`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        parentId: input.parentId,
      }),
    });
  },

  async uploadSelection(input: {
    libraryId: string;
    parentId: string | null;
    mode: 'files' | 'folder';
    items: FileCenterUploadItem[];
  }): Promise<{ message: string; createdCount: number }> {
    if (input.items.length === 0) {
      return { message: '未选择任何上传内容', createdCount: 0 };
    }

    const formData = new FormData();
    formData.set('mode', input.mode);
    if (input.parentId) {
      formData.set('parentId', input.parentId);
    }
    const manifest = input.items.map((item, index) => {
      const field = `file${index}`;
      formData.append(field, item.file, item.name);
      return {
        field,
        name: item.name,
        relativePath: item.relativePath ?? item.name,
      };
    });
    formData.set('manifest', JSON.stringify(manifest));
    return fetchFileCenterData<{ message: string; createdCount: number }>(
      `/api/libraries/${input.libraryId}/uploads`,
      {
        method: 'POST',
        body: formData,
      },
    );
  },

  async deleteAssets(ids: string[]): Promise<{ message: string; jobId?: string }> {
    if (ids.length === 0) {
      return { message: '未找到需要删除的条目' };
    }
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/file-entries/delete-assets`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        entryIds: ids,
      }),
    });
  },

  async deleteFromEndpoint(input: {
    entryIds: string[];
    endpointName: string;
  }): Promise<{ message: string; jobId?: string }> {
    if (input.entryIds.length === 0) {
      return { message: '当前没有可删除副本' };
    }
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/file-entries/delete-replicas`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        entryIds: input.entryIds,
        endpointName: input.endpointName,
      }),
    });
  },

  async syncToEndpoint(input: {
    entryIds: string[];
    endpointName: string;
  }): Promise<{ message: string; jobId?: string }> {
    if (input.entryIds.length === 0) {
      return { message: '当前没有可同步内容' };
    }
    return fetchFileCenterData<{ message: string; jobId?: string }>(`/api/file-entries/replicate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        entryIds: input.entryIds,
        endpointName: input.endpointName,
      }),
    });
  },

  async loadTagManagementSnapshot(searchText = ''): Promise<TagManagementSnapshot> {
    const query = new URLSearchParams();
    if (searchText.trim()) {
      query.set('searchText', searchText.trim());
    }
    return fetchFileCenterData<TagManagementSnapshot>(
      `/api/tags/management${query.size > 0 ? `?${query.toString()}` : ''}`,
    );
  },

  async createTagGroup(name: string): Promise<{ message: string; groupId: string }> {
    const result = await fetchFileCenterData<{ message: string; groupId: string }>('/api/tags/groups', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name: name.trim() }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async updateTagGroup(id: string, name: string): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/groups/${id}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name: name.trim() }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async moveTagGroup(id: string, direction: 'up' | 'down'): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/groups/${id}/move`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ direction }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async createManagedTag(input: {
    groupId: string;
    isPinned: boolean;
    libraryIds: string[];
    name: string;
  }): Promise<{ message: string; tagId: string }> {
    const result = await fetchFileCenterData<{ message: string; tagId: string }>('/api/tags', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
    emitFileCenterUpdate();
    return result;
  },

  async updateManagedTag(
    id: string,
    input: {
      groupId: string;
      isPinned: boolean;
      libraryIds: string[];
      name: string;
    },
  ): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${id}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
    emitFileCenterUpdate();
    return result;
  },

  async moveManagedTag(id: string, direction: 'up' | 'down'): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${id}/move`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ direction }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async mergeManagedTag(sourceId: string, targetId: string): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${sourceId}/merge`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ targetId }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async deleteManagedTag(id: string): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/tags/${id}`, {
      method: 'DELETE',
    });
    emitFileCenterUpdate();
    return result;
  },

  async updateAnnotations(
    id: string,
    input: {
      rating: number;
      colorLabel: FileCenterColorLabel;
      tags: string[];
    },
  ): Promise<{ message: string }> {
    const result = await fetchFileCenterData<{ message: string }>(`/api/file-entries/${id}/annotations`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        rating: Math.max(0, Math.min(5, input.rating)),
        colorLabel: input.colorLabel,
        tags: input.tags,
      }),
    });
    emitFileCenterUpdate();
    return result;
  },

  async replaceEntryTagsByNames(entryId: string, tags: string[]): Promise<{ message: string }> {
    const detail = await fileCenterApi.loadEntryDetail(entryId);
    if (!detail) {
      throw new Error('未找到资产');
    }
    const result = await fileCenterApi.updateAnnotations(entryId, {
      rating: detail.rating,
      colorLabel: detail.colorLabel,
      tags,
    });
    emitFileCenterUpdate();
    return result;
  },

  async loadTagSuggestions(searchText = '', libraryId?: string): Promise<FileCenterTagSuggestion[]> {
    const query = new URLSearchParams();
    if (searchText.trim()) {
      query.set('searchText', searchText.trim());
    }
    if (libraryId?.trim()) {
      query.set('libraryId', libraryId.trim());
    }
    return fetchFileCenterData<FileCenterTagSuggestion[]>(
      `/api/tags/suggestions${query.size > 0 ? `?${query.toString()}` : ''}`,
    );
  },
};

export async function resetFileCenterMock() {
  libraryEndpointNamesCache.clear();
}

async function fetchFileCenterData<T>(path: string, init?: RequestInit): Promise<T> {
  const { centerBaseUrl } = getRuntimeConfig();
  const response = await fetch(`${centerBaseUrl}${path}`, init);
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message ?? `center service returned status ${response.status}`);
  }

  const payload = (await response.json()) as { data: T };
  return payload.data;
}

function emitFileCenterUpdate() {
  fileCenterListeners.forEach((listener) => listener());
}

function normalizeDirectoryAnnotations<T extends FileCenterEntry>(entry: T): T {
  if (entry.type !== 'folder') {
    return entry;
  }
  return {
    ...entry,
    rating: 0,
    colorLabel: '无',
  };
}

export function normalizeFileCenterEndpointState(state: string): FileCenterEndpointState {
  if (state === '已同步' || state === '已存在') {
    return '已同步';
  }
  if (state === '部分同步') {
    return '部分同步';
  }
  if (state === '同步中') {
    return '同步中';
  }
  return '未同步';
}

export function resolveFileCenterEndpointTone(state: string): Severity {
  const normalizedState = normalizeFileCenterEndpointState(state);
  if (normalizedState === '已同步') {
    return 'success';
  }
  if (normalizedState === '部分同步' || normalizedState === '同步中') {
    return 'warning';
  }
  return 'critical';
}

export function canSyncFileCenterEndpoint(endpoint: Pick<FileCenterEndpoint, 'state'> | string) {
  const state = typeof endpoint === 'string' ? endpoint : endpoint.state;
  return ['未同步', '部分同步'].includes(normalizeFileCenterEndpointState(state));
}

export function canDeleteFileCenterEndpoint(endpoint: Pick<FileCenterEndpoint, 'state'> | string) {
  const state = typeof endpoint === 'string' ? endpoint : endpoint.state;
  return ['已同步', '部分同步'].includes(normalizeFileCenterEndpointState(state));
}

export function resolveFileCenterSyncExecution(
  entry: Pick<FileCenterEntry, 'endpoints'>,
  endpointName: string,
): FileCenterSyncExecution | null {
  const targetEndpoint = entry.endpoints.find((endpoint) => endpoint.name === endpointName);
  if (!targetEndpoint) {
    return null;
  }

  const sourceEndpoint = chooseBestSourceEndpoint(entry.endpoints, endpointName);
  if (!sourceEndpoint) {
    return null;
  }

  if (targetEndpoint.endpointType === 'cloud') {
    return {
      routeType: 'UPLOAD',
      engine: 'CD2_REMOTE_UPLOAD',
      summary: `通过 CloudDrive2 同步到 ${targetEndpoint.name}，支持断点续传`,
      sourceEndpointName: sourceEndpoint.name,
      targetEndpointName: targetEndpoint.name,
      supportsResume: true,
      usesCloud: true,
    };
  }

  if (sourceEndpoint.endpointType === 'cloud') {
    return {
      routeType: 'DOWNLOAD',
      engine: 'ARIA2',
      summary: `通过 aria2 从 ${sourceEndpoint.name} 同步到 ${targetEndpoint.name}，支持断点续传`,
      sourceEndpointName: sourceEndpoint.name,
      targetEndpointName: targetEndpoint.name,
      supportsResume: true,
      usesCloud: true,
    };
  }

  return {
    routeType: 'COPY',
    engine: 'INTERNAL_COPY',
    summary: `直接同步到 ${targetEndpoint.name}`,
    sourceEndpointName: sourceEndpoint.name,
    targetEndpointName: targetEndpoint.name,
    supportsResume: false,
    usesCloud: false,
  };
}

export function resolveFileCenterSyncAvailability(
  entry: Pick<FileCenterEntry, 'endpoints'>,
  endpointName: string,
  integrationHealth?: FileCenterIntegrationHealth,
): FileCenterActionAvailability<FileCenterSyncExecution> {
  const targetEndpoint = entry.endpoints.find((endpoint) => endpoint.name === endpointName);
  if (!targetEndpoint) {
    return { enabled: false, reason: '未找到目标端点' };
  }
  if (!canSyncFileCenterEndpoint(targetEndpoint)) {
    return { enabled: false, reason: '当前端点副本已存在或正在同步' };
  }

  const execution = resolveFileCenterSyncExecution(entry, endpointName);
  if (!execution) {
    return { enabled: false, reason: '当前没有可用源副本' };
  }

  if (execution.routeType === 'UPLOAD') {
    if (!integrationHealth?.cd2Online) {
      return { enabled: false, reason: 'CloudDrive2 当前不可用，请先前往设置页处理', execution };
    }
    if (!integrationHealth?.cloudAuthReady) {
      return {
        enabled: false,
        reason: integrationHealth?.cloudAuthMessage ?? '115 节点鉴权未就绪，请先前往存储节点页处理',
        execution,
      };
    }
  }

  if (execution.routeType === 'DOWNLOAD') {
    if (!integrationHealth?.cd2Online) {
      return { enabled: false, reason: 'CloudDrive2 当前不可用，请先前往设置页处理', execution };
    }
    if (!integrationHealth?.aria2Online) {
      return { enabled: false, reason: 'aria2 当前不可用，请先前往设置页处理', execution };
    }
    if (!integrationHealth?.cloudAuthReady) {
      return {
        enabled: false,
        reason: integrationHealth?.cloudAuthMessage ?? '115 节点鉴权未就绪，请先前往存储节点页处理',
        execution,
      };
    }
  }

  return { enabled: true, execution };
}

export function resolveFileCenterDeleteAvailability(
  entry: Pick<FileCenterEntry, 'endpoints'>,
  endpointName: string,
  integrationHealth?: FileCenterIntegrationHealth,
): FileCenterActionAvailability<FileCenterDeleteExecution> {
  const targetEndpoint = entry.endpoints.find((endpoint) => endpoint.name === endpointName);
  if (!targetEndpoint) {
    return { enabled: false, reason: '未找到目标端点' };
  }

  const execution: FileCenterDeleteExecution =
    targetEndpoint.endpointType === 'cloud'
      ? {
          summary: `通过 CloudDrive2 从 ${targetEndpoint.name} 删除副本`,
          targetEndpointName: targetEndpoint.name,
          usesCloud: true,
        }
      : {
          summary: `从 ${targetEndpoint.name} 删除副本`,
          targetEndpointName: targetEndpoint.name,
          usesCloud: false,
        };

  if (!canDeleteFileCenterEndpoint(targetEndpoint)) {
    return { enabled: false, reason: '当前端点上没有可删除的副本', execution };
  }

  if (targetEndpoint.endpointType === 'cloud') {
    if (!integrationHealth?.cd2Online) {
      return { enabled: false, reason: 'CloudDrive2 当前不可用，请先前往设置页处理', execution };
    }
    if (!integrationHealth?.cloudAuthReady) {
      return {
        enabled: false,
        reason: integrationHealth?.cloudAuthMessage ?? '115 节点鉴权未就绪，请先前往存储节点页处理',
        execution,
      };
    }
  }

  return { enabled: true, execution };
}

function chooseBestSourceEndpoint(
  endpoints: FileCenterEntry['endpoints'],
  targetEndpointName: string,
): FileCenterEndpoint | null {
  const candidates = endpoints
    .filter(
      (endpoint) =>
        endpoint.name !== targetEndpointName &&
        ['已同步', '部分同步'].includes(normalizeFileCenterEndpointState(endpoint.state)),
    )
    .sort((left, right) => endpointSourceWeight(left.endpointType) - endpointSourceWeight(right.endpointType));
  return candidates[0] ?? null;
}

function endpointSourceWeight(endpointType: FileCenterEndpointType) {
  switch (endpointType) {
    case 'local':
      return 0;
    case 'nas':
      return 1;
    case 'cloud':
      return 2;
    default:
      return 3;
  }
}

function sortEntryRows(
  rows: EntryRow[],
  sortValue: FileCenterSortValue,
  sortDirection: FileCenterSortDirection,
) {
  const direction = sortDirection === 'asc' ? 1 : -1;

  return [...rows].sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === 'folder' ? -1 : 1;
    }

    if (sortValue === '名称') {
      const compared = fileNameCollator.compare(left.name, right.name) * direction;
      if (compared !== 0) {
        return compared;
      }
      return right.modified_at_sort - left.modified_at_sort;
    }

    if (sortValue === '大小') {
      const compared = (left.size_bytes - right.size_bytes) * direction;
      if (compared !== 0) {
        return compared;
      }
      return right.modified_at_sort - left.modified_at_sort;
    }

    if (sortValue === '星级') {
      const compared = (left.rating - right.rating) * direction;
      if (compared !== 0) {
        return compared;
      }
      return right.modified_at_sort - left.modified_at_sort;
    }

    const compared = (left.modified_at_sort - right.modified_at_sort) * direction;
    if (compared !== 0) {
      return compared;
    }
    return fileNameCollator.compare(left.name, right.name);
  });
}

function sortEntryRowsForDisplay(
  rows: EntryRow[],
  sortValue: FileCenterSortValue,
  sortDirection: FileCenterSortDirection,
) {
  if (sortValue !== '星级') {
    return sortEntryRows(rows, sortValue, sortDirection);
  }

  const direction = sortDirection === 'asc' ? 1 : -1;
  return [...rows].sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === 'folder' ? -1 : 1;
    }
    if (left.type === 'folder') {
      return 0;
    }

    const compared = (left.rating - right.rating) * direction;
    if (compared !== 0) {
      return compared;
    }
    return right.modified_at_sort - left.modified_at_sort;
  });
}

export const __FILE_CENTER_TESTING__ = {
  sortEntryRowsForDisplay,
  resolveSyncPlan: resolveFileCenterSyncExecution,
  resolveSyncAvailability: resolveFileCenterSyncAvailability,
  resolveDeleteAvailability: resolveFileCenterDeleteAvailability,
};
