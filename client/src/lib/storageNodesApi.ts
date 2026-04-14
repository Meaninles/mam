import { getRuntimeConfig } from './runtimeConfig';

export type StorageTone = 'success' | 'warning' | 'critical' | 'info';
export type StorageHeartbeatPolicy = '从不' | '每周（深夜）' | '每日（深夜）' | '每小时';
export type StorageMountMode = '只读' | '可写';
export type MountFolderType = '本地' | 'NAS' | '网盘';
export type StorageCloudAccessMethod = '填入 Token' | '扫码登录获取 Token';
export type StorageCloudQrChannel = '微信小程序' | '支付宝小程序' | '电视端';

export type MountRecord = {
  id: string;
  name: string;
  libraryId: string;
  libraryName: string;
  folderType: MountFolderType;
  nodeId: string;
  nodeName: string;
  nodeRootPath: string;
  relativePath: string;
  sourceRefId?: string;
  sourceName?: string;
  address: string;
  mountMode: StorageMountMode;
  enabled: boolean;
  scanStatus: string;
  scanTone: StorageTone;
  lastScanAt: string;
  heartbeatPolicy: StorageHeartbeatPolicy;
  nextHeartbeatAt: string;
  capacitySummary: string;
  freeSpaceSummary: string;
  capacityPercent: number;
  riskTags: string[];
  badges: string[];
  authStatus: string;
  authTone: StorageTone;
  notes: string;
};

export type MountFolderRecord = MountRecord;

export type LocalNodeRecord = {
  id: string;
  name: string;
  rootPath: string;
  enabled: boolean;
  healthStatus: string;
  healthTone: StorageTone;
  lastCheckAt: string;
  capacitySummary: string;
  freeSpaceSummary: string;
  capacityPercent: number;
  mountCount: number;
  notes: string;
};

export type NasRecord = {
  id: string;
  name: string;
  address: string;
  accessMode: string;
  username: string;
  passwordHint: string;
  lastTestAt?: string;
  status: string;
  tone: StorageTone;
  mountCount: number;
  notes: string;
};

export type CloudRecord = {
  id: string;
  name: string;
  vendor: '115';
  accessMethod: StorageCloudAccessMethod;
  qrChannel?: StorageCloudQrChannel;
  mountDirectory: string;
  tokenStatus: string;
  token?: string;
  accountAlias?: string;
  lastAuthAt?: string;
  lastAuthResult?: string;
  lastErrorCode?: string;
  lastErrorMessage?: string;
  lastTestAt?: string;
  status: string;
  tone: StorageTone;
  mountCount: number;
  notes: string;
};

type CloudRecordResponse = Omit<CloudRecord, 'mountDirectory'> & {
  mountDirectory?: string;
  mountPath?: string;
};

export type CloudQRCodeSession = {
  uid: string;
  time: number;
  sign: string;
  qrcode: string;
  channel: StorageCloudQrChannel;
  codeVerifier?: string;
};

export type CloudQRCodeStatusResponse = {
  status: string;
  message: string;
};

export type StorageNodesDashboard = {
  localNodes: LocalNodeRecord[];
  nasNodes: NasRecord[];
  cloudNodes: CloudRecord[];
  mounts: MountRecord[];
  mountFolders: MountRecord[];
};

export type MountDraft = {
  id?: string;
  name: string;
  libraryId: string;
  libraryName?: string;
  nodeId: string;
  mountMode: StorageMountMode;
  heartbeatPolicy: StorageHeartbeatPolicy;
  relativePath: string;
  notes: string;
  folderType?: MountFolderType;
  localPath?: string;
  nasId?: string;
  cloudId?: string;
  targetFolder?: string;
};

export type MountFolderDraft = MountDraft;

export type LocalNodeDraft = {
  id?: string;
  name: string;
  rootPath: string;
  notes: string;
};

export type NasDraft = {
  id?: string;
  name: string;
  address: string;
  username: string;
  password: string;
  notes: string;
};

export type CloudDraft = {
  id?: string;
  name: string;
  vendor: '115';
  accessMethod: StorageCloudAccessMethod;
  qrChannel: StorageCloudQrChannel;
  mountDirectory: string;
  token: string;
  tokenStatus?: string;
  accountAlias?: string;
  lastAuthAt?: string;
  lastAuthResult?: string;
  lastErrorCode?: string;
  lastErrorMessage?: string;
  lastTestAt?: string;
  status?: string;
  tone?: StorageTone;
  qrSession?: CloudQRCodeSession;
  notes: string;
};

export type StorageConnectionTestResult = {
  id: string;
  name: string;
  overallTone: StorageTone;
  summary: string;
  checks: Array<{
    label: string;
    status: StorageTone;
    detail: string;
  }>;
  suggestion?: string;
  testedAt: string;
};

export type StorageScanHistoryItem = {
  id: string;
  startedAt: string;
  finishedAt: string;
  status: '成功' | '失败' | '进行中';
  summary: string;
  trigger: string;
};

type StorageBrowserDb = {
  cloudNodes: CloudRecord[];
  histories: Record<string, StorageScanHistoryItem[]>;
  localNodes: LocalNodeRecord[];
  mounts: MountRecord[];
  nasNodes: NasRecord[];
};

const BROWSER_STORAGE_KEY = 'mare-storage-nodes-browser-v3';
const LOCAL_FOLDERS_API_PATH = '/api/storage/local-folders';

export const storageNodesApi = {
  async loadDashboard(): Promise<StorageNodesDashboard> {
    const fallback = runBrowserFallback<StorageNodesDashboard>('storage_nodes_load_dashboard', {});
    const [localNodesResult, mountsResult, nasNodesResult, cloudNodesResult] = await Promise.allSettled([
      fetchCenterDataWithRetry<LocalNodeRecord[]>('/api/storage/local-nodes'),
      fetchCenterDataWithRetry<MountRecord[]>(LOCAL_FOLDERS_API_PATH),
      fetchCenterDataWithRetry<NasRecord[]>('/api/storage/nas-nodes'),
      fetchCenterDataWithRetry<CloudRecordResponse[]>('/api/storage/cloud-nodes'),
    ]);

    const localNodes = localNodesResult.status === 'fulfilled' ? localNodesResult.value : [];
    const mounts = mountsResult.status === 'fulfilled' ? mountsResult.value : [];
    const nasNodes = nasNodesResult.status === 'fulfilled' ? nasNodesResult.value : [];
    const cloudNodes =
      cloudNodesResult.status === 'fulfilled'
        ? cloudNodesResult.value.map((item) => ({
            ...item,
            mountDirectory: item.mountDirectory ?? item.mountPath ?? '',
            token: item.token,
          }))
        : fallback.cloudNodes;

    if (
      localNodesResult.status === 'rejected' &&
      mountsResult.status === 'rejected' &&
      nasNodesResult.status === 'rejected' &&
      cloudNodesResult.status === 'rejected'
    ) {
      throw localNodesResult.reason;
    }

    return {
      localNodes,
      nasNodes,
      cloudNodes,
      mounts,
      mountFolders: mounts,
    };
  },

  async saveLocalNode(draft: LocalNodeDraft): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string; record: LocalNodeRecord }>('/api/storage/local-nodes', {
      method: 'POST',
      body: JSON.stringify({
        id: draft.id,
        name: draft.name,
        rootPath: draft.rootPath,
        notes: draft.notes,
      }),
    });
    return { message: result.message };
  },

  async saveMount(draft: MountDraft): Promise<{ message: string; record: MountRecord }> {
    const result = await fetchCenterData<{ message: string; record: MountRecord }>(LOCAL_FOLDERS_API_PATH, {
      method: 'POST',
      body: JSON.stringify({
        id: draft.id,
        name: draft.name,
        nodeId: draft.nodeId,
        libraryId: draft.libraryId,
        libraryName: draft.libraryName ?? draft.libraryId,
        mountMode: draft.mountMode,
        heartbeatPolicy: draft.heartbeatPolicy,
        relativePath: draft.relativePath,
        notes: draft.notes,
      }),
    });
    return result;
  },

  async saveMountFolder(draft: MountDraft): Promise<{ message: string }> {
    return this.saveMount(draft);
  },

  async saveNasNode(draft: NasDraft): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string; record: NasRecord }>('/api/storage/nas-nodes', {
      method: 'POST',
      body: JSON.stringify({
        id: draft.id,
        name: draft.name,
        address: draft.address,
        username: draft.username,
        password: draft.password,
        notes: draft.notes,
      }),
    });
    return { message: result.message };
  },

  async saveCloudNode(draft: CloudDraft): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string; record: CloudRecord }>('/api/storage/cloud-nodes', {
      method: 'POST',
      body: JSON.stringify({
        id: draft.id,
        name: draft.name,
        vendor: draft.vendor,
        accessMethod: draft.accessMethod,
        qrChannel: draft.accessMethod === '扫码登录获取 Token' ? draft.qrChannel : '',
        mountPath: draft.mountDirectory,
        token: draft.accessMethod === '填入 Token' ? draft.token : '',
        qrSession: draft.accessMethod === '扫码登录获取 Token' ? draft.qrSession : undefined,
        notes: draft.notes,
      }),
    });
    return { message: result.message };
  },

  async createCloudQrSession(channel: StorageCloudQrChannel): Promise<CloudQRCodeSession> {
    return fetchCenterData<CloudQRCodeSession>('/api/storage/cloud-nodes/qr-session', {
      method: 'POST',
      body: JSON.stringify({ channel }),
    });
  },

  async getCloudQrSessionStatus(session: CloudQRCodeSession): Promise<CloudQRCodeStatusResponse> {
    return fetchCenterData<CloudQRCodeStatusResponse>('/api/storage/cloud-nodes/qr-session/status', {
      method: 'POST',
      body: JSON.stringify(session),
    });
  },

  getCloudQrImageUrl(session: CloudQRCodeSession) {
    const { centerBaseUrl } = getRuntimeConfig();
    const payload = encodeURIComponent(JSON.stringify(session));
    return `${centerBaseUrl}/api/storage/cloud-nodes/qr-session/image?payload=${payload}`;
  },

  async runMountScan(ids: string[]): Promise<{ message: string; jobId?: string }> {
    const result = await fetchCenterData<{ message: string; jobId?: string }>(`${LOCAL_FOLDERS_API_PATH}/scan`, {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
    return { message: result.message, jobId: result.jobId };
  },

  async runMountConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return fetchCenterData<{ message: string; results: StorageConnectionTestResult[] }>(
      `${LOCAL_FOLDERS_API_PATH}/connection-test`,
      {
        method: 'POST',
        body: JSON.stringify({ ids }),
      },
    );
  },

  async runLocalNodeConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return fetchCenterData<{ message: string; results: StorageConnectionTestResult[] }>(
      '/api/storage/local-nodes/connection-test',
      {
        method: 'POST',
        body: JSON.stringify({ ids }),
      },
    );
  },

  async deleteLocalNode(id: string): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string }>(`/api/storage/local-nodes/${id}`, {
      method: 'DELETE',
    });
    return { message: result.message };
  },

  async runNasConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return fetchCenterData<{ message: string; results: StorageConnectionTestResult[] }>(
      '/api/storage/nas-nodes/connection-test',
      {
        method: 'POST',
        body: JSON.stringify({ ids }),
      },
    );
  },

  async runCloudConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return fetchCenterData<{ message: string; results: StorageConnectionTestResult[] }>(
      '/api/storage/cloud-nodes/connection-test',
      {
        method: 'POST',
        body: JSON.stringify({ ids }),
      },
    );
  },

  async updateMountHeartbeat(ids: string[], heartbeatPolicy: StorageHeartbeatPolicy): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string; record: MountFolderRecord }>(LOCAL_FOLDERS_API_PATH, {
      method: 'PATCH',
      body: JSON.stringify({ ids, heartbeatPolicy }),
    });
    return { message: result.message };
  },

  async loadMountScanHistory(id: string): Promise<{ id: string; items: StorageScanHistoryItem[] }> {
    return fetchCenterData<{ id: string; items: StorageScanHistoryItem[] }>(`${LOCAL_FOLDERS_API_PATH}/${id}/scan-history`);
  },

  async deleteMountFolder(id: string): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string }>(`${LOCAL_FOLDERS_API_PATH}/${id}`, {
      method: 'DELETE',
    });
    return { message: result.message };
  },

  async deleteNasNode(id: string): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string }>(`/api/storage/nas-nodes/${id}`, {
      method: 'DELETE',
    });
    return { message: result.message };
  },

  async deleteCloudNode(id: string): Promise<{ message: string }> {
    const result = await fetchCenterData<{ message: string }>(`/api/storage/cloud-nodes/${id}`, {
      method: 'DELETE',
    });
    return { message: result.message };
  },

  async browseLocalFolder(): Promise<{ path: string | null }> {
    return invokeStorageCommand<{ path: string | null }>('storage_nodes_browse_local_folder', {});
  },
};

async function invokeStorageCommand<T>(command: string, payload: Record<string, unknown>): Promise<T> {
  if (isTauriRuntime()) {
    try {
      const { invoke } = await import('@tauri-apps/api/core');
      return await invoke<T>(command, payload);
    } catch {
      return runBrowserFallback<T>(command, payload);
    }
  }

  return runBrowserFallback<T>(command, payload);
}

async function fetchCenterData<T>(path: string, init?: RequestInit): Promise<T> {
  const { centerBaseUrl } = getRuntimeConfig();
  const response = await fetch(`${centerBaseUrl}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message ?? `center service returned status ${response.status}`);
  }

  const payload = (await response.json()) as { data: T };
  return payload.data;
}

async function fetchCenterDataWithRetry<T>(path: string, attempts = 6, delayMs = 400): Promise<T> {
  let lastError: unknown;
  for (let index = 0; index < attempts; index += 1) {
    try {
      return await fetchCenterData<T>(path);
    } catch (error) {
      lastError = error;
      if (index === attempts - 1) {
        break;
      }
      await new Promise((resolve) => window.setTimeout(resolve, delayMs));
    }
  }
  throw lastError instanceof Error ? lastError : new Error('center service request failed');
}

function isTauriRuntime() {
  return typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window;
}

function runBrowserFallback<T>(command: string, payload: Record<string, unknown>): T {
  const db = loadBrowserDb();

  switch (command) {
    case 'storage_nodes_load_dashboard':
      return structuredClone({
        localNodes: db.localNodes,
        nasNodes: db.nasNodes,
        cloudNodes: db.cloudNodes,
        mounts: db.mounts,
        mountFolders: db.mounts,
      }) as T;
    case 'storage_nodes_save_mount_folder':
      saveMountFolderInBrowser(db, payload.draft as MountDraft);
      persistBrowserDb(db);
      return { message: '挂载文件夹已保存' } as T;
    case 'storage_nodes_save_nas_node':
      saveNasNodeInBrowser(db, payload.draft as NasDraft);
      persistBrowserDb(db);
      return { message: 'NAS 已保存' } as T;
    case 'storage_nodes_save_cloud_node':
      saveCloudNodeInBrowser(db, payload.draft as CloudDraft);
      persistBrowserDb(db);
      return { message: '网盘已保存' } as T;
    case 'storage_nodes_run_mount_scan':
      runMountScanInBrowser(db, payload.ids as string[]);
      persistBrowserDb(db);
      return { message: `已为 ${(payload.ids as string[]).length} 个挂载文件夹创建扫描任务` } as T;
    case 'storage_nodes_run_mount_connection_test':
      return {
        message:
          (payload.ids as string[]).length > 1
            ? `已完成 ${(payload.ids as string[]).length} 个挂载文件夹的连接测试`
            : '连接测试已完成',
        results: buildMountConnectionResults(db, payload.ids as string[]),
      } as T;
    case 'storage_nodes_run_nas_connection_test':
      return runNasConnectionTestInBrowser(db, payload.ids as string[]) as T;
    case 'storage_nodes_run_cloud_connection_test':
      return runCloudConnectionTestInBrowser(db, payload.ids as string[]) as T;
    case 'storage_nodes_update_mount_heartbeat':
      updateMountHeartbeatInBrowser(db, payload.ids as string[], payload.heartbeatPolicy as StorageHeartbeatPolicy);
      persistBrowserDb(db);
      return { message: '心跳策略已更新' } as T;
    case 'storage_nodes_load_mount_scan_history':
      return {
        id: payload.id as string,
        items: structuredClone(db.histories[payload.id as string] ?? []),
      } as T;
    case 'storage_nodes_delete_mount_folder':
      db.mounts = db.mounts.filter((item) => item.id !== payload.id);
      delete db.histories[payload.id as string];
      persistBrowserDb(db);
      return { message: '挂载文件夹已删除' } as T;
    case 'storage_nodes_delete_nas_node':
      db.nasNodes = db.nasNodes.filter((item) => item.id !== payload.id);
      db.mounts = db.mounts.filter((item) => item.sourceRefId !== payload.id);
      persistBrowserDb(db);
      return { message: 'NAS 已删除' } as T;
    case 'storage_nodes_delete_cloud_node':
      db.cloudNodes = db.cloudNodes.filter((item) => item.id !== payload.id);
      db.mounts = db.mounts.filter((item) => item.sourceRefId !== payload.id);
      persistBrowserDb(db);
      return { message: '网盘已删除' } as T;
    case 'storage_nodes_browse_local_folder':
      return { path: null } as T;
    default:
      throw new Error(`未实现的命令：${command}`);
  }
}

function loadBrowserDb(): StorageBrowserDb {
  if (typeof window === 'undefined') {
    return createSeedBrowserDb();
  }

  const raw = window.localStorage.getItem(BROWSER_STORAGE_KEY);
  if (!raw) {
    const seeded = createSeedBrowserDb();
    persistBrowserDb(seeded);
    return seeded;
  }

  try {
    const parsed = JSON.parse(raw) as StorageBrowserDb;
    if (!Array.isArray(parsed.localNodes) || !Array.isArray(parsed.mounts) || !Array.isArray(parsed.nasNodes) || !Array.isArray(parsed.cloudNodes)) {
      throw new Error('invalid storage db');
    }
    return parsed;
  } catch {
    const seeded = createSeedBrowserDb();
    persistBrowserDb(seeded);
    return seeded;
  }
}

function persistBrowserDb(db: StorageBrowserDb) {
  if (typeof window === 'undefined') {
    return;
  }

  window.localStorage.setItem(BROWSER_STORAGE_KEY, JSON.stringify(db));
}

function saveMountFolderInBrowser(db: StorageBrowserDb, draft: MountDraft) {
  const record = draftToMountFolderRecord(db, draft);
  db.mounts = draft.id ? db.mounts.map((item) => (item.id === draft.id ? record : item)) : [record, ...db.mounts];

  if (!db.histories[record.id]) {
    db.histories[record.id] = [];
  }
}

function saveNasNodeInBrowser(db: StorageBrowserDb, draft: NasDraft) {
  const record: NasRecord = {
    id: draft.id ?? `nas-${Math.random().toString(36).slice(2, 8)}`,
    name: draft.name,
    address: draft.address,
    accessMode: 'SMB',
    username: draft.username,
    passwordHint: draft.password ? '刚刚更新' : '未更新',
    lastTestAt: draft.id ? db.nasNodes.find((item) => item.id === draft.id)?.lastTestAt : undefined,
    status: '鉴权正常',
    tone: 'success',
    mountCount: draft.id ? (db.nasNodes.find((item) => item.id === draft.id)?.mountCount ?? 0) : 0,
    notes: draft.notes,
  };

  db.nasNodes = draft.id
    ? db.nasNodes.map((item) => (item.id === draft.id ? record : item))
    : [record, ...db.nasNodes];
}

function saveCloudNodeInBrowser(db: StorageBrowserDb, draft: CloudDraft) {
  const record: CloudRecord = {
    id: draft.id ?? `cloud-${Math.random().toString(36).slice(2, 8)}`,
    name: draft.name,
    vendor: draft.vendor,
    accessMethod: draft.accessMethod,
    qrChannel: draft.accessMethod === '扫码登录获取 Token' ? draft.qrChannel : undefined,
    mountDirectory: draft.mountDirectory,
    tokenStatus: draft.token ? '已配置' : '未配置',
    token: draft.token || (draft.id ? db.cloudNodes.find((item) => item.id === draft.id)?.token : '') || '',
    lastTestAt: draft.id ? db.cloudNodes.find((item) => item.id === draft.id)?.lastTestAt : undefined,
    status: draft.token ? '鉴权正常' : '待鉴权',
    tone: draft.token ? 'success' : 'warning',
    mountCount: draft.id ? (db.cloudNodes.find((item) => item.id === draft.id)?.mountCount ?? 0) : 0,
    notes: draft.notes,
  };

  db.cloudNodes = draft.id
    ? db.cloudNodes.map((item) => (item.id === draft.id ? record : item))
    : [record, ...db.cloudNodes];
}

function runMountScanInBrowser(db: StorageBrowserDb, ids: string[]) {
  db.mounts = db.mounts.map((item) =>
    ids.includes(item.id)
      ? {
          ...item,
          scanStatus: '扫描中',
          scanTone: 'warning',
          lastScanAt: '正在执行',
        }
      : item,
  );

  ids.forEach((id) => {
    const item = db.mounts.find((mount) => mount.id === id);
    const next = db.histories[id] ?? [];
    db.histories[id] = [
      {
        id: `history-${Math.random().toString(36).slice(2, 8)}`,
        startedAt: '刚刚',
        finishedAt: '进行中',
        status: '进行中',
        summary: `${item?.name ?? '挂载文件夹'} 扫描任务已创建。`,
        trigger: ids.length > 1 ? '批量扫描' : '手动扫描',
      },
      ...next,
    ];
  });
}

function updateMountHeartbeatInBrowser(
  db: StorageBrowserDb,
  ids: string[],
  heartbeatPolicy: StorageHeartbeatPolicy,
) {
  db.mounts = db.mounts.map((item) =>
    ids.includes(item.id)
      ? {
          ...item,
          heartbeatPolicy,
          nextHeartbeatAt:
            heartbeatPolicy === '从不'
              ? '—'
              : heartbeatPolicy === '每小时'
                ? '1 小时后'
                : heartbeatPolicy === '每日（深夜）'
                  ? '今晚 02:00'
                  : '周六 02:00',
        }
      : item,
  );
}

function buildMountConnectionResults(db: StorageBrowserDb, ids: string[]): StorageConnectionTestResult[] {
  return ids
    .map((id) => db.mounts.find((item) => item.id === id))
    .filter((item): item is MountRecord => Boolean(item))
    .map((item) => ({
      id: item.id,
      name: item.name,
      overallTone: item.authTone === 'critical' ? 'critical' : item.riskTags.length > 0 ? 'warning' : 'success',
      summary: item.riskTags.length > 0 ? '当前挂载目录可达，但建议先处理风险提示。' : '挂载目录可达且当前配置可继续使用。',
      checks: [
        { label: '可达性', status: 'success', detail: `${item.address} 可访问。` },
        { label: '鉴权状态', status: item.authTone, detail: item.authStatus },
        { label: '读权限', status: 'success', detail: '可读取挂载目录。' },
        { label: '写权限', status: item.mountMode === '只读' ? 'warning' : 'success', detail: item.mountMode === '只读' ? '当前为只读挂载。' : '可写入挂载目录。' },
        { label: '目标目录可访问', status: 'success', detail: '目录检查通过。' },
      ],
      suggestion: item.riskTags.length > 0 ? '检查配置' : '可立即执行扫描',
      testedAt: '刚刚',
    }));
}

function runNasConnectionTestInBrowser(
  db: StorageBrowserDb,
  ids: string[],
): { message: string; results: StorageConnectionTestResult[] } {
  const results = ids
    .map((id) => db.nasNodes.find((item) => item.id === id))
    .filter((item): item is NasRecord => Boolean(item))
    .map((item, index): StorageConnectionTestResult => ({
      id: item.id,
      name: item.name,
      overallTone: index % 2 === 0 ? 'success' : 'warning',
      summary: index % 2 === 0 ? 'NAS 连接测试通过，可继续使用当前配置。' : 'NAS 可达，但鉴权检查未完全通过。',
      checks: [
        { label: '可达性', status: 'success', detail: `${item.address} 可达。` },
        { label: '鉴权状态', status: index % 2 === 0 ? 'success' : 'warning', detail: index % 2 === 0 ? '账号密码验证通过。' : '账号密码需要重新确认。' },
      ],
      suggestion: index % 2 === 0 ? '可继续挂载' : '检查账号密码',
      testedAt: '刚刚',
    }));

  db.nasNodes = db.nasNodes.map((item) =>
    ids.includes(item.id)
      ? {
          ...item,
          status: results.find((result) => result.id === item.id)?.overallTone === 'success' ? '鉴权正常' : '鉴权异常',
          tone: (results.find((result) => result.id === item.id)?.overallTone ?? 'warning') as StorageTone,
          lastTestAt: '刚刚',
        }
      : item,
  );
  persistBrowserDb(db);

  return {
    message: ids.length > 1 ? `已完成 ${ids.length} 个 NAS 的连接测试` : '连接测试已完成',
    results,
  };
}

function runCloudConnectionTestInBrowser(
  db: StorageBrowserDb,
  ids: string[],
): { message: string; results: StorageConnectionTestResult[] } {
  const results = ids
    .map((id) => db.cloudNodes.find((item) => item.id === id))
    .filter((item): item is CloudRecord => Boolean(item))
    .map((item, index): StorageConnectionTestResult => ({
      id: item.id,
      name: item.name,
      overallTone: index % 2 === 0 ? 'success' : 'warning',
      summary: index % 2 === 0 ? '网盘连接测试通过，Token 当前可继续使用。' : '网盘可达，但 Token 需要重新确认。',
      checks: [
        { label: '可达性', status: 'success', detail: `${item.mountDirectory} 可访问。` },
        { label: '鉴权状态', status: index % 2 === 0 ? 'success' : 'warning', detail: index % 2 === 0 ? 'Token 验证通过。' : 'Token 已失效或即将过期。' },
      ],
      suggestion: index % 2 === 0 ? '可继续挂载' : '重新获取 Token',
      testedAt: '刚刚',
    }));

  db.cloudNodes = db.cloudNodes.map((item) =>
    ids.includes(item.id)
      ? {
          ...item,
          status: results.find((result) => result.id === item.id)?.overallTone === 'success' ? '鉴权正常' : '鉴权异常',
          tone: (results.find((result) => result.id === item.id)?.overallTone ?? 'warning') as StorageTone,
          lastTestAt: '刚刚',
        }
      : item,
  );
  persistBrowserDb(db);

  return {
    message: ids.length > 1 ? `已完成 ${ids.length} 个网盘的连接测试` : '连接测试已完成',
    results,
  };
}

function draftToMountFolderRecord(db: StorageBrowserDb, draft: MountDraft): MountRecord {
  const libraryMap = new Map(
    [
      ['photo', '商业摄影资产库'],
      ['video', '视频工作流资产库'],
      ['family', '家庭照片资产库'],
    ] as Array<[string, string]>,
  );

  const localNode = db.localNodes.find((item) => item.id === draft.nodeId);
  const folder = draft.relativePath.trim().replace(/^[/\\]+/, '');
  const address = localNode
    ? `${localNode.rootPath.replace(/[\\/]+$/, '')}${folder ? `\\${folder}` : ''}`
    : folder;
  return {
    id: draft.id ?? `mount-${Math.random().toString(36).slice(2, 8)}`,
    name: draft.name,
    libraryId: draft.libraryId,
    libraryName: libraryMap.get(draft.libraryId) ?? draft.libraryId,
    folderType: '本地',
    nodeId: draft.nodeId,
    nodeName: localNode?.name ?? '',
    nodeRootPath: localNode?.rootPath ?? '',
    relativePath: draft.relativePath,
    address,
    mountMode: draft.mountMode,
    enabled: true,
    scanStatus: '未扫描',
    scanTone: 'info',
    lastScanAt: '未扫描',
    heartbeatPolicy: draft.heartbeatPolicy,
    nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
    capacitySummary: '远端容量待检测',
    freeSpaceSummary: '待首次检测',
    capacityPercent: 0,
    riskTags: [],
    badges: ['本地', draft.mountMode],
    authStatus: '无需鉴权',
    authTone: 'info',
    notes: draft.notes,
  };
}

function createSeedBrowserDb(): StorageBrowserDb {
  const baseNasNodes: NasRecord[] = [
    {
      id: 'nas-main',
      name: '影像 NAS 01',
      address: '\\\\192.168.10.20\\media',
      accessMode: 'SMB',
      username: 'mare-sync',
      passwordHint: '已保存，最近更新于 3 天前',
      lastTestAt: '今天 10:12',
      status: '鉴权正常',
      tone: 'success',
      mountCount: 1,
      notes: '主 NAS',
    },
    {
      id: 'nas-backup',
      name: '影像 NAS 备份柜',
      address: '\\\\192.168.10.36\\backup_media',
      accessMode: 'SMB',
      username: 'mare-archive',
      passwordHint: '已保存，最近更新于 7 天前',
      lastTestAt: '昨天 22:18',
      status: '鉴权正常',
      tone: 'success',
      mountCount: 0,
      notes: '备份 NAS',
    },
  ];
  const extraNasNodes: NasRecord[] = Array.from({ length: 22 }, (_, index): NasRecord => ({
      id: `nas-extra-${index + 1}`,
      name: `项目 NAS ${index + 1}`,
      address: `\\\\192.168.20.${100 + index}\\share_${index + 1}`,
      accessMode: 'SMB',
      username: `project-user-${index + 1}`,
      passwordHint: `已保存，最近更新于 ${index + 1} 天前`,
      lastTestAt: `昨天 ${String((index % 9) + 10).padStart(2, '0')}:00`,
      status: index % 4 === 0 ? '鉴权异常' : '鉴权正常',
      tone: index % 4 === 0 ? 'warning' : 'success',
      mountCount: index % 3 === 0 ? 1 : 0,
      notes: `项目 NAS ${index + 1}`,
    }));
  const nasNodes: NasRecord[] = baseNasNodes.concat(extraNasNodes);

  const baseCloudNodes: CloudRecord[] = [
    {
      id: 'cloud-archive',
      name: '115 云归档',
      vendor: '115',
      accessMethod: '填入 Token',
      mountDirectory: '/MareArchive',
      tokenStatus: '48 小时内过期',
      lastTestAt: '今天 08:40',
      status: '鉴权异常',
      tone: 'warning',
      mountCount: 1,
      notes: '云归档空间',
    },
    {
      id: 'cloud-exchange',
      name: '115 项目交换区',
      vendor: '115',
      accessMethod: '扫码登录获取 Token',
      qrChannel: '微信小程序',
      mountDirectory: '/ProjectExchange',
      tokenStatus: '已配置',
      lastTestAt: '今天 09:05',
      status: '鉴权正常',
      tone: 'success',
      mountCount: 0,
      notes: '项目交换空间',
    },
  ];
  const extraCloudNodes: CloudRecord[] = Array.from({ length: 22 }, (_, index): CloudRecord => ({
      id: `cloud-extra-${index + 1}`,
      name: `115 分区 ${index + 1}`,
      vendor: '115' as const,
      accessMethod: index % 2 === 0 ? '填入 Token' : '扫码登录获取 Token',
      qrChannel: index % 2 === 0 ? undefined : (index % 3 === 0 ? '支付宝小程序' : '微信小程序'),
      mountDirectory: `/ProjectSpace/${index + 1}`,
      tokenStatus: index % 5 === 0 ? '即将过期' : '已配置',
      lastTestAt: `今天 ${String((index % 10) + 8).padStart(2, '0')}:30`,
      status: index % 5 === 0 ? '鉴权异常' : '鉴权正常',
      tone: index % 5 === 0 ? 'warning' : 'success',
      mountCount: index % 3,
      notes: `网盘空间 ${index + 1}`,
    }));
  const cloudNodes: CloudRecord[] = baseCloudNodes.concat(extraCloudNodes);

  const localNodes: LocalNodeRecord[] = [
    {
      id: 'local-node-main',
      name: '商业摄影本地素材根',
      rootPath: 'D:\\Mare\\Assets',
      enabled: true,
      healthStatus: '可用',
      healthTone: 'success',
      lastCheckAt: '今天 09:12',
      capacitySummary: '已用 64% · 3.4 TB 可用',
      freeSpaceSummary: '3.4 TB 可用',
      capacityPercent: 64,
      mountCount: 1,
      notes: '商业摄影本地素材根目录',
    },
  ];

  const baseMountFolders: MountRecord[] = [
    {
      id: 'mount-local-main',
      name: '商业摄影原片库',
      libraryId: 'photo',
      libraryName: '商业摄影资产库',
      nodeId: 'local-node-main',
      nodeName: '商业摄影本地素材根',
      nodeRootPath: 'D:\\Mare\\Assets',
      relativePath: 'PhotoRaw',
      folderType: '本地',
      address: 'D:\\Mare\\Assets\\PhotoRaw',
      mountMode: '可写',
      enabled: true,
      scanStatus: '最近扫描成功',
      scanTone: 'success',
      lastScanAt: '今天 09:12',
      heartbeatPolicy: '从不',
      nextHeartbeatAt: '—',
      capacitySummary: '已用 64% · 3.4 TB 可用',
      freeSpaceSummary: '3.4 TB 可用',
      capacityPercent: 64,
      riskTags: [],
      badges: ['可写', '本地'],
      authStatus: '无需鉴权',
      authTone: 'info',
      notes: '商业摄影本地主挂载目录',
    },
  ];
  const extraMountFolders: MountFolderRecord[] = Array.from({ length: 24 }, (_, index): MountFolderRecord => ({
      id: `mount-extra-${index + 1}`,
      name: `扩展挂载 ${index + 1}`,
      libraryId: index % 3 === 0 ? 'photo' : index % 3 === 1 ? 'video' : 'family',
      libraryName: index % 3 === 0 ? '商业摄影资产库' : index % 3 === 1 ? '视频工作流资产库' : '家庭照片资产库',
      folderType: index % 3 === 0 ? '本地' as const : index % 3 === 1 ? 'NAS' as const : '网盘' as const,
      nodeId: 'local-node-main',
      nodeName: '商业摄影本地素材根',
      nodeRootPath: 'D:\\Mare\\Assets',
      relativePath: `Library_${index + 1}`,
      sourceRefId: undefined,
      sourceName: undefined,
      address:
        index % 3 === 0
          ? `D:\\Mare\\Assets\\Library_${index + 1}`
          : index % 3 === 1
            ? `\\\\192.168.20.${100 + (index % 22)}\\share_${(index % 22) + 1}\\folder_${index + 1}`
            : `/ProjectSpace/${(index % 22) + 1}/folder_${index + 1}`,
      mountMode: index % 4 === 0 ? '只读' : '可写',
      enabled: true,
      scanStatus: index % 5 === 0 ? '最近扫描失败' : index % 4 === 0 ? '等待队列' : '最近扫描成功',
      scanTone: index % 5 === 0 ? 'critical' : index % 4 === 0 ? 'info' : 'success',
      lastScanAt: index % 4 === 0 ? `${index + 1} 分钟前` : `今天 ${String((index % 10) + 8).padStart(2, '0')}:${String((index % 6) * 10).padStart(2, '0')}`,
      heartbeatPolicy: index % 3 === 0 ? '从不' : index % 3 === 1 ? '每日（深夜）' : '每周（深夜）',
      nextHeartbeatAt: index % 3 === 0 ? '—' : index % 3 === 1 ? '今晚 02:00' : '周六 02:00',
      capacitySummary: `已用 ${35 + (index % 40)}% · ${(1.5 + index * 0.2).toFixed(1)} TB 可用`,
      freeSpaceSummary: `${(1.5 + index * 0.2).toFixed(1)} TB 可用`,
      capacityPercent: 35 + (index % 40),
      riskTags: index % 5 === 0 ? ['扫描失败'] : [],
      badges: [index % 3 === 0 ? '本地' : index % 3 === 1 ? 'SMB' : '115', index % 4 === 0 ? '只读' : '可写'],
      authStatus: index % 3 === 0 ? '无需鉴权' : index % 5 === 0 ? '鉴权异常' : '鉴权正常',
      authTone: index % 3 === 0 ? 'info' : index % 5 === 0 ? 'warning' : 'success',
      notes: `扩展挂载记录 ${index + 1}`,
    }));
  const mounts: MountRecord[] = baseMountFolders.concat(extraMountFolders);

  return {
    localNodes,
    mounts,
    nasNodes,
    cloudNodes,
    histories: {
      'mount-cloud-archive': [
        {
          id: 'history-cloud-1',
          startedAt: '2026-03-31 02:00',
          finishedAt: '2026-03-31 02:18',
          status: '成功',
          summary: '新增 218 项，变更 12 项，未发现异常。',
          trigger: '计划扫描',
        },
        {
          id: 'history-cloud-2',
          startedAt: '2026-03-30 02:00',
          finishedAt: '2026-03-30 02:06',
          status: '失败',
          summary: '远端目录读取超时，已写入异常中心。',
          trigger: '计划扫描',
        },
      ],
    },
  };
}
