import type { IssueCapabilities, IssueHistoryRecord, IssueRecord } from '../data';
import { getRuntimeConfig } from './runtimeConfig';

type IssueListResponse = {
  items: IssueDto[];
  total: number;
  page: number;
  pageSize: number;
};

type IssueDto = {
  id: string;
  code: string;
  libraryId?: string;
  taskId?: string;
  taskItemId?: string;
  issueCategory: string;
  issueType: string;
  nature: string;
  sourceDomain: string;
  severity: string;
  status: string;
  title: string;
  summary: string;
  assetLabel?: string;
  objectLabel: string;
  suggestedAction?: string;
  suggestedActionLabel?: string;
  suggestion?: string;
  detail?: string;
  occurrenceCount: number;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
  archivedAt?: string;
  source: {
    taskId?: string;
    taskTitle?: string;
    taskItemId?: string;
    taskItemTitle?: string;
    assetId?: string;
    entryId?: string;
    endpointId?: string;
    endpointLabel?: string;
    path?: string;
    sourceLabel?: string;
    routeLabel?: string;
  };
  impact: {
    assetCount: number;
    replicaCount: number;
    directoryCount: number;
    endpointCount: number;
    blocksStatusCommit: boolean;
    blocksTaskExecution: boolean;
  };
  capabilities: IssueCapabilities;
  histories: IssueHistoryRecord[];
};

export const issuesApi = {
  async list(params: {
    page?: number;
    pageSize?: number;
    searchText?: string;
    issueCategory?: string;
    sourceDomain?: string;
    libraryId?: string;
    status?: string;
    severity?: string;
    nature?: string;
    sortValue?: string;
    endpointId?: string;
    path?: string;
    jobIds?: string[];
  } = {}) {
    const query = new URLSearchParams();
    if (params.page) query.set('page', String(params.page));
    if (params.pageSize) query.set('pageSize', String(params.pageSize));
    if (params.searchText) query.set('searchText', params.searchText);
    if (params.issueCategory) query.set('issueCategory', params.issueCategory);
    if (params.sourceDomain) query.set('sourceDomain', params.sourceDomain);
    if (params.libraryId) query.set('libraryId', params.libraryId);
    if (params.status) query.set('status', params.status);
    if (params.severity) query.set('severity', params.severity);
    if (params.nature) query.set('nature', params.nature);
    if (params.sortValue) query.set('sortValue', params.sortValue);
    if (params.endpointId) query.set('endpointId', params.endpointId);
    if (params.path) query.set('path', params.path);
    for (const jobId of params.jobIds ?? []) {
      query.append('jobId', jobId);
    }

    const response = await fetchIssuesData<IssueListResponse>(`/api/issues${query.toString() ? `?${query.toString()}` : ''}`);
    return {
      ...response,
      items: response.items.map(mapIssueDtoToRecord),
    };
  },

  async listAll() {
    const items: IssueRecord[] = [];
    let page = 1;

    while (true) {
      const response = await issuesApi.list({ page, pageSize: 100 });
      items.push(...response.items);
      if (items.length >= response.total || response.items.length === 0) {
        return items;
      }
      page += 1;
    }
  },

  async listByJobIds(jobIds: string[]) {
    if (jobIds.length === 0) {
      return [] as IssueRecord[];
    }
    const query = new URLSearchParams();
    for (const jobId of jobIds) {
      query.append('jobId', jobId);
    }
    const response = await fetchIssuesData<{ items: IssueDto[] }>(`/api/issues/by-jobs?${query.toString()}`);
    return response.items.map(mapIssueDtoToRecord);
  },

  async applyAction(ids: string[], action: 'retry' | 'confirm' | 'ignore' | 'archive') {
    return fetchIssuesData<{ message: string; ids: string[] }>('/api/issues/actions', {
      method: 'POST',
      body: JSON.stringify({ ids, action }),
    });
  },

  async clearHistory(ids: string[]) {
    return fetchIssuesData<{ message: string; ids: string[] }>('/api/issues/history/clear', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
  },
};

async function fetchIssuesData<T>(path: string, init?: RequestInit): Promise<T> {
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

function mapIssueDtoToRecord(input: IssueDto): IssueRecord {
  return {
    id: input.id,
    libraryId: input.libraryId ?? '',
    taskId: input.taskId ?? input.source.taskId,
    taskItemId: input.taskItemId ?? input.source.taskItemId,
    category: mapCategory(input.issueCategory),
    type: input.issueType,
    nature: input.nature === 'RISK' ? 'RISK' : 'BLOCKING',
    sourceDomain: mapSourceDomain(input.sourceDomain),
    severity: mapSeverity(input.severity),
    title: input.title,
    summary: input.summary,
    asset: input.assetLabel ?? input.objectLabel,
    objectLabel: input.objectLabel,
    action: input.suggestedAction ?? '',
    actionLabel: input.suggestedActionLabel ?? '查看详情',
    suggestion: input.suggestion ?? '',
    detail: input.detail ?? '',
    occurrenceCount: input.occurrenceCount || 1,
    status: mapStatus(input.status),
    createdAt: input.createdAt,
    updatedAt: input.updatedAt,
    resolvedAt: input.resolvedAt,
    archivedAt: input.archivedAt,
    source: {
      sourceDomain: mapSourceDomain(input.sourceDomain),
      taskId: input.source.taskId,
      taskTitle: input.source.taskTitle,
      taskItemId: input.source.taskItemId,
      taskItemTitle: input.source.taskItemTitle,
      assetId: input.source.assetId,
      fileNodeId: input.source.entryId,
      endpointId: input.source.endpointId,
      endpointLabel: input.source.endpointLabel,
      path: input.source.path,
      sourceLabel: input.source.sourceLabel,
      routeLabel: input.source.routeLabel,
    },
    impact: {
      assetCount: input.impact.assetCount,
      replicaCount: input.impact.replicaCount,
      folderCount: input.impact.directoryCount,
      endpointCount: input.impact.endpointCount,
      blocksStatusCommit: input.impact.blocksStatusCommit,
      blocksTaskExecution: input.impact.blocksTaskExecution,
    },
    capabilities: input.capabilities,
    histories: input.histories,
  };
}

function mapCategory(value: string): IssueRecord['category'] {
  if (value === 'CONFLICT') return '冲突';
  if (value === 'TRANSFER') return '传输';
  if (value === 'VERIFY') return '校验';
  if (value === 'NODE_PERMISSION') return '节点与权限';
  if (value === 'CAPACITY_RESOURCE') return '容量与资源';
  if (value === 'CLEANUP_GOVERNANCE') return '清理与治理';
  return '扫描与解析';
}

function mapSourceDomain(value: string): IssueRecord['sourceDomain'] {
  if (value === 'TRANSFER_JOB') return '传输任务';
  if (value === 'FILE_CENTER') return '文件中心';
  if (value === 'STORAGE_DOMAIN') return '存储节点';
  if (value === 'SYSTEM_GOVERNANCE') return '系统治理';
  return '其他任务';
}

function mapSeverity(value: string): IssueRecord['severity'] {
  if (value === 'CRITICAL') return 'critical';
  if (value === 'WARNING') return 'warning';
  return 'info';
}

function mapStatus(value: string): IssueRecord['status'] {
  if (value === 'AWAITING_CONFIRMATION') return '待确认';
  if (value === 'IN_PROGRESS') return '处理中';
  if (value === 'IGNORED') return '已忽略';
  if (value === 'RESOLVED') return '已解决';
  if (value === 'ARCHIVED') return '已归档';
  return '待处理';
}
