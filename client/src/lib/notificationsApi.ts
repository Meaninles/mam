import type { NoticeRecord, Severity } from '../data';
import { getRuntimeConfig } from './runtimeConfig';

type NotificationListResponse = {
  items: NotificationDto[];
  total: number;
  page: number;
  pageSize: number;
};

type NotificationDto = {
  id: string;
  kind: 'ACTION_REQUIRED' | 'REMINDER';
  sourceType: 'ISSUE' | 'JOB';
  sourceId: string;
  jobId?: string;
  issueId?: string;
  libraryId?: string;
  lifecycleStatus: 'ACTIVE' | 'STALE';
  defaultTargetKind: NoticeRecord['jumpParams']['kind'];
  title: string;
  summary: string;
  severity: 'CRITICAL' | 'WARNING' | 'INFO' | 'SUCCESS';
  objectLabel: string;
  createdAt: string;
  updatedAt: string;
  source: {
    sourceDomain?: string;
    issueCategory?: string;
    issueNature?: string;
    issueSourceDomain?: string;
    taskId?: string;
    taskItemId?: string;
    fileNodeId?: string;
    endpointId?: string;
    path?: string;
    sourceLabel?: string;
    routeLabel?: string;
  };
  capabilities: NoticeRecord['capabilities'];
  jumpParams: {
    kind: NoticeRecord['jumpParams']['kind'];
    issueId?: string;
    taskId?: string;
    taskItemId?: string;
    libraryId?: string;
    endpointId?: string;
    fileNodeId?: string;
    path?: string;
    sourceDomain?: string;
    label?: string;
  };
};

export type NotificationStreamEvent = {
  eventId: string;
  topic: string;
  eventType: string;
  notificationId: string;
  lifecycleStatus?: 'ACTIVE' | 'STALE';
  createdAt: string;
};

export const notificationsApi = {
  async list(params: {
    page?: number;
    pageSize?: number;
    kind?: string;
    searchText?: string;
    includeStale?: boolean;
  } = {}) {
    const query = new URLSearchParams();
    if (params.page) query.set('page', String(params.page));
    if (params.pageSize) query.set('pageSize', String(params.pageSize));
    if (params.kind) query.set('kind', params.kind);
    if (params.searchText) query.set('searchText', params.searchText);
    if (params.includeStale) query.set('includeStale', 'true');

    const response = await fetchNotificationsData<NotificationListResponse>(
      `/api/notifications${query.toString() ? `?${query.toString()}` : ''}`,
    );
    return {
      ...response,
      items: response.items.map(mapNotificationDtoToNoticeRecord),
    };
  },

  async listAll() {
    const items: NoticeRecord[] = [];
    let page = 1;

    while (true) {
      const response = await notificationsApi.list({ page, pageSize: 100, includeStale: true });
      items.push(...response.items);
      if (items.length >= response.total || response.items.length === 0) {
        return items;
      }
      page += 1;
    }
  },

  subscribe(onEvent: (event: NotificationStreamEvent) => void) {
    const { centerBaseUrl } = getRuntimeConfig();
    const source = new EventSource(`${centerBaseUrl}/api/notifications/stream`);

    const handleMessage = (event: MessageEvent<string>) => {
      if (!event.data) {
        return;
      }
      onEvent(JSON.parse(event.data) as NotificationStreamEvent);
    };

    source.onmessage = handleMessage;
    ['NOTIFICATION_CREATED', 'NOTIFICATION_UPDATED', 'NOTIFICATION_STALE'].forEach((eventType) => {
      source.addEventListener(eventType, handleMessage as EventListener);
    });

    return () => {
      source.close();
    };
  },
};

async function fetchNotificationsData<T>(path: string, init?: RequestInit): Promise<T> {
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

function mapNotificationDtoToNoticeRecord(input: NotificationDto): NoticeRecord {
  return {
    id: input.id,
    kind: input.kind,
    sourceType: input.sourceType,
    sourceId: input.sourceId,
    issueId: input.issueId,
    title: input.title,
    summary: input.summary,
    severity: mapSeverity(input.severity),
    libraryId: input.libraryId,
    objectLabel: input.objectLabel,
    status: input.lifecycleStatus === 'STALE' ? 'STALE' : 'UNREAD',
    createdAt: input.createdAt,
    updatedAt: input.updatedAt,
    sortKey: Date.parse(input.updatedAt) || Date.now(),
    source: {
      sourceDomain: mapNoticeSourceDomain(input.source.sourceDomain),
      issueCategory: mapIssueCategory(input.source.issueCategory),
      issueNature: mapIssueNature(input.source.issueNature),
      issueSourceDomain: mapIssueSourceDomain(input.source.issueSourceDomain),
      taskId: input.source.taskId,
      taskItemId: input.source.taskItemId,
      fileNodeId: input.source.fileNodeId,
      endpointId: input.source.endpointId,
      path: input.source.path,
      sourceLabel: input.source.sourceLabel,
      routeLabel: input.source.routeLabel,
    },
    capabilities: input.capabilities,
    jumpParams: {
      kind: input.jumpParams.kind,
      issueId: input.jumpParams.issueId,
      taskId: input.jumpParams.taskId,
      taskItemId: input.jumpParams.taskItemId,
      libraryId: input.jumpParams.libraryId,
      endpointId: input.jumpParams.endpointId,
      fileNodeId: input.jumpParams.fileNodeId,
      path: input.jumpParams.path,
      sourceDomain: mapIssueSourceDomain(input.jumpParams.sourceDomain),
      label: input.jumpParams.label,
    },
  };
}

function mapSeverity(value: NotificationDto['severity']): Severity {
  if (value === 'CRITICAL') return 'critical';
  if (value === 'WARNING') return 'warning';
  if (value === 'SUCCESS') return 'success';
  return 'info';
}

function mapNoticeSourceDomain(value?: string): NoticeRecord['source']['sourceDomain'] {
  if (value === 'TASK_CENTER') return '任务中心';
  if (value === 'IMPORT_CENTER') return '导入中心';
  if (value === 'STORAGE_NODES') return '存储节点';
  if (value === 'SYSTEM_NOTICE') return '系统提醒';
  return '异常中心';
}

function mapIssueCategory(value?: string): NoticeRecord['source']['issueCategory'] {
  if (value === 'CONFLICT') return '冲突';
  if (value === 'TRANSFER') return '传输';
  if (value === 'VERIFY') return '校验';
  if (value === 'NODE_PERMISSION') return '节点与权限';
  if (value === 'CAPACITY_RESOURCE') return '容量与资源';
  if (value === 'CLEANUP_GOVERNANCE') return '清理与治理';
  if (value === 'SCAN_PARSE') return '扫描与解析';
  return undefined;
}

function mapIssueNature(value?: string): NoticeRecord['source']['issueNature'] {
  if (value === 'BLOCKING' || value === 'RISK') return value;
  return undefined;
}

function mapIssueSourceDomain(value?: string): NoticeRecord['source']['issueSourceDomain'] {
  if (value === 'TRANSFER_JOB') return '传输任务';
  if (value === 'FILE_CENTER') return '文件中心';
  if (value === 'STORAGE_DOMAIN') return '存储节点';
  if (value === 'SYSTEM_GOVERNANCE') return '系统治理';
  if (value === 'MAINTENANCE_JOB') return '其他任务';
  if (value === 'IMPORT_DOMAIN') return '其他任务';
  return value === undefined ? undefined : '其他任务';
}
