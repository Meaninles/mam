import { getRuntimeConfig } from './runtimeConfig';

export type JobPriority = 'LOW' | 'NORMAL' | 'HIGH';
export type JobStatus =
  | 'PENDING'
  | 'QUEUED'
  | 'RUNNING'
  | 'PAUSED'
  | 'WAITING_CONFIRMATION'
  | 'WAITING_RETRY'
  | 'PARTIAL_SUCCESS'
  | 'FAILED'
  | 'COMPLETED'
  | 'CANCELED';

export type JobRecord = {
  id: string;
  code: string;
  libraryId?: string;
  jobFamily: string;
  jobIntent: string;
  routeType?: string;
  status: JobStatus;
  priority: JobPriority;
  title: string;
  summary: string;
  sourceDomain: string;
  sourceRefId?: string;
  sourceSnapshot?: unknown;
  progressPercent: number;
  speedBps?: number;
  etaSeconds?: number;
  totalItems: number;
  successItems: number;
  failedItems: number;
  skippedItems: number;
  issueCount: number;
  latestErrorCode?: string;
  latestErrorMessage?: string;
  outcomeSummary?: string;
  createdByType: string;
  createdByRef?: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  canceledAt?: string;
  updatedAt: string;
};

export type JobItemRecord = {
  id: string;
  jobId: string;
  parentItemId?: string;
  itemKey: string;
  itemType: string;
  routeType?: string;
  status: string;
  phase?: string;
  title: string;
  summary: string;
  sourcePath?: string;
  targetPath?: string;
  progressPercent: number;
  speedBps?: number;
  etaSeconds?: number;
  bytesTotal?: number;
  bytesDone?: number;
  externalTaskEngine?: string;
  externalTaskId?: string;
  externalTaskStatus?: string;
  attemptCount: number;
  issueCount: number;
  latestErrorCode?: string;
  latestErrorMessage?: string;
  resultSummary?: string;
  startedAt?: string;
  finishedAt?: string;
  canceledAt?: string;
  updatedAt: string;
  createdAt: string;
};

export type JobEventRecord = {
  id: string;
  jobId: string;
  jobItemId?: string;
  jobAttemptId?: string;
  eventType: string;
  message: string;
  payload?: unknown;
  createdAt: string;
};

export type JobDetail = {
  job: JobRecord;
  items: JobItemRecord[];
  links: Array<{
    id: string;
    jobId: string;
    jobItemId?: string;
    linkRole: string;
    objectType: string;
    assetId?: string;
    assetReplicaId?: string;
    directoryId?: string;
    mountId?: string;
    storageNodeId?: string;
    createdAt: string;
  }>;
};

export type JobStreamEvent = {
  eventId: string;
  topic: string;
  eventType: string;
  jobId: string;
  jobItemId?: string;
  jobStatus?: JobStatus;
  itemStatus?: string;
  message: string;
  createdAt: string;
};

export type JobItemMutationResponse = {
  message: string;
  job: JobRecord;
  item: JobItemRecord;
};

export const jobsApi = {
  async list(params: {
    page?: number;
    pageSize?: number;
    searchText?: string;
    status?: string;
    jobFamily?: string;
    sourceDomain?: string;
    libraryId?: string;
  } = {}) {
    const query = new URLSearchParams();
    if (params.page) query.set('page', String(params.page));
    if (params.pageSize) query.set('pageSize', String(params.pageSize));
    if (params.searchText) query.set('searchText', params.searchText);
    if (params.status) query.set('status', params.status);
    if (params.jobFamily) query.set('jobFamily', params.jobFamily);
    if (params.sourceDomain) query.set('sourceDomain', params.sourceDomain);
    if (params.libraryId) query.set('libraryId', params.libraryId);
    return fetchJobsData<{ items: JobRecord[]; total: number; page: number; pageSize: number }>(
      `/api/jobs${query.toString() ? `?${query.toString()}` : ''}`,
    );
  },

  async detail(id: string) {
    return fetchJobsData<JobDetail>(`/api/jobs/${id}`);
  },

  async events(id: string) {
    return fetchJobsData<{ items: JobEventRecord[] }>(`/api/jobs/${id}/events`);
  },

  async pause(id: string) {
    return fetchJobsData<{ message: string; job: JobRecord }>(`/api/jobs/${id}/pause`, { method: 'POST' });
  },

  async resume(id: string) {
    return fetchJobsData<{ message: string; job: JobRecord }>(`/api/jobs/${id}/resume`, { method: 'POST' });
  },

  async cancel(id: string) {
    return fetchJobsData<{ message: string; job: JobRecord }>(`/api/jobs/${id}/cancel`, { method: 'POST' });
  },

  async pauseItem(id: string) {
    return fetchJobsData<JobItemMutationResponse>(`/api/job-items/${id}/pause`, { method: 'POST' });
  },

  async resumeItem(id: string) {
    return fetchJobsData<JobItemMutationResponse>(`/api/job-items/${id}/resume`, { method: 'POST' });
  },

  async cancelItem(id: string) {
    return fetchJobsData<JobItemMutationResponse>(`/api/job-items/${id}/cancel`, { method: 'POST' });
  },

  async retry(id: string) {
    return fetchJobsData<{ message: string; job: JobRecord }>(`/api/jobs/${id}/retry`, { method: 'POST' });
  },

  async updatePriority(id: string, priority: JobPriority) {
    return fetchJobsData<{ message: string; job: JobRecord }>(`/api/jobs/${id}/priority`, {
      method: 'PATCH',
      body: JSON.stringify({ priority }),
    });
  },

  subscribe(onEvent: (event: JobStreamEvent) => void, options?: { jobId?: string }) {
    const { centerBaseUrl } = getRuntimeConfig();
    const query = new URLSearchParams();
    if (options?.jobId) {
      query.set('jobId', options.jobId);
    }
    const source = new EventSource(`${centerBaseUrl}/api/events/stream${query.toString() ? `?${query.toString()}` : ''}`);

    const handleMessage = (event: MessageEvent<string>) => {
      if (!event.data) {
        return;
      }
      onEvent(JSON.parse(event.data) as JobStreamEvent);
    };

    source.onmessage = handleMessage;
    [
      'JOB_CREATED',
      'JOB_QUEUED',
      'JOB_STARTED',
      'JOB_PAUSED',
      'JOB_RESUMED',
      'JOB_CANCELED',
      'JOB_RETRIED',
      'JOB_COMPLETED',
      'JOB_FAILED',
      'JOB_PARTIAL_SUCCESS',
      'JOB_PRIORITY_CHANGED',
      'JOB_ITEM_PROGRESS',
      'JOB_ITEM_STARTED',
      'JOB_ITEM_PAUSED',
      'JOB_ITEM_RESUMED',
      'JOB_ITEM_COMPLETED',
      'JOB_ITEM_FAILED',
      'JOB_ITEM_CANCELED',
    ].forEach((eventType) => {
      source.addEventListener(eventType, handleMessage as EventListener);
    });

    return () => {
      source.close();
    };
  },
};

async function fetchJobsData<T>(path: string, init?: RequestInit): Promise<T> {
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
