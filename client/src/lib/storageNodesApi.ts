export type StorageTone = 'success' | 'warning' | 'critical' | 'info';
export type StorageHeartbeatPolicy = '从不' | '每周（深夜）' | '每日（深夜）' | '每小时';
export type StorageMountMode = '只读' | '可写';
export type MountFolderType = '本地' | 'NAS' | '网盘';
export type StorageCloudAccessMethod = '填入 Token' | '扫码登录获取 Token';
export type StorageCloudQrChannel = '微信小程序' | '支付宝小程序' | '电视端';

export type MountFolderRecord = {
  id: string;
  name: string;
  libraryId: string;
  libraryName: string;
  folderType: MountFolderType;
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

export type NasRecord = {
  id: string;
  name: string;
  address: string;
  username: string;
  passwordHint: string;
  lastTestAt?: string;
  status: string;
  tone: StorageTone;
  notes: string;
};

export type CloudRecord = {
  id: string;
  name: string;
  vendor: '115';
  accessMethod: StorageCloudAccessMethod;
  qrChannel?: StorageCloudQrChannel;
  accountAlias: string;
  mountDirectory: string;
  tokenStatus: string;
  lastTestAt?: string;
  status: string;
  tone: StorageTone;
  notes: string;
};

export type StorageNodesDashboard = {
  mountFolders: MountFolderRecord[];
  nasNodes: NasRecord[];
  cloudNodes: CloudRecord[];
};

export type MountFolderDraft = {
  id?: string;
  name: string;
  libraryId: string;
  folderType: MountFolderType;
  mountMode: StorageMountMode;
  heartbeatPolicy: StorageHeartbeatPolicy;
  localPath: string;
  nasId: string;
  cloudId: string;
  targetFolder: string;
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
  accountAlias: string;
  mountDirectory: string;
  token: string;
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
  mountFolders: MountFolderRecord[];
  nasNodes: NasRecord[];
};

const BROWSER_STORAGE_KEY = 'mare-storage-nodes-browser-v3';

export const storageNodesApi = {
  async loadDashboard(): Promise<StorageNodesDashboard> {
    return invokeStorageCommand<StorageNodesDashboard>('storage_nodes_load_dashboard', {});
  },

  async saveMountFolder(draft: MountFolderDraft): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_save_mount_folder', { draft });
  },

  async saveNasNode(draft: NasDraft): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_save_nas_node', { draft });
  },

  async saveCloudNode(draft: CloudDraft): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_save_cloud_node', { draft });
  },

  async runMountScan(ids: string[]): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_run_mount_scan', { ids });
  },

  async runMountConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return invokeStorageCommand<{ message: string; results: StorageConnectionTestResult[] }>(
      'storage_nodes_run_mount_connection_test',
      { ids },
    );
  },

  async runNasConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return invokeStorageCommand<{ message: string; results: StorageConnectionTestResult[] }>(
      'storage_nodes_run_nas_connection_test',
      { ids },
    );
  },

  async runCloudConnectionTest(ids: string[]): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return invokeStorageCommand<{ message: string; results: StorageConnectionTestResult[] }>(
      'storage_nodes_run_cloud_connection_test',
      { ids },
    );
  },

  async updateMountHeartbeat(ids: string[], heartbeatPolicy: StorageHeartbeatPolicy): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_update_mount_heartbeat', { heartbeatPolicy, ids });
  },

  async loadMountScanHistory(id: string): Promise<{ id: string; items: StorageScanHistoryItem[] }> {
    return invokeStorageCommand<{ id: string; items: StorageScanHistoryItem[] }>('storage_nodes_load_mount_scan_history', { id });
  },

  async deleteMountFolder(id: string): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_delete_mount_folder', { id });
  },

  async deleteNasNode(id: string): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_delete_nas_node', { id });
  },

  async deleteCloudNode(id: string): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_delete_cloud_node', { id });
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

function isTauriRuntime() {
  return typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window;
}

function runBrowserFallback<T>(command: string, payload: Record<string, unknown>): T {
  const db = loadBrowserDb();

  switch (command) {
    case 'storage_nodes_load_dashboard':
      return structuredClone({
        mountFolders: db.mountFolders,
        nasNodes: db.nasNodes,
        cloudNodes: db.cloudNodes,
      }) as T;
    case 'storage_nodes_save_mount_folder':
      saveMountFolderInBrowser(db, payload.draft as MountFolderDraft);
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
      db.mountFolders = db.mountFolders.filter((item) => item.id !== payload.id);
      delete db.histories[payload.id as string];
      persistBrowserDb(db);
      return { message: '挂载文件夹已删除' } as T;
    case 'storage_nodes_delete_nas_node':
      db.nasNodes = db.nasNodes.filter((item) => item.id !== payload.id);
      db.mountFolders = db.mountFolders.filter((item) => item.sourceRefId !== payload.id);
      persistBrowserDb(db);
      return { message: 'NAS 已删除' } as T;
    case 'storage_nodes_delete_cloud_node':
      db.cloudNodes = db.cloudNodes.filter((item) => item.id !== payload.id);
      db.mountFolders = db.mountFolders.filter((item) => item.sourceRefId !== payload.id);
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
    if (!Array.isArray(parsed.mountFolders) || !Array.isArray(parsed.nasNodes) || !Array.isArray(parsed.cloudNodes)) {
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

function saveMountFolderInBrowser(db: StorageBrowserDb, draft: MountFolderDraft) {
  const record = draftToMountFolderRecord(db, draft);
  db.mountFolders = draft.id
    ? db.mountFolders.map((item) => (item.id === draft.id ? record : item))
    : [record, ...db.mountFolders];

  if (!db.histories[record.id]) {
    db.histories[record.id] = [];
  }
}

function saveNasNodeInBrowser(db: StorageBrowserDb, draft: NasDraft) {
  const record: NasRecord = {
    id: draft.id ?? `nas-${Math.random().toString(36).slice(2, 8)}`,
    name: draft.name,
    address: draft.address,
    username: draft.username,
    passwordHint: draft.password ? '刚刚更新' : '未更新',
    lastTestAt: draft.id ? db.nasNodes.find((item) => item.id === draft.id)?.lastTestAt : undefined,
    status: '鉴权正常',
    tone: 'success',
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
    accountAlias: draft.accountAlias,
    mountDirectory: draft.mountDirectory,
    tokenStatus: draft.token ? '已配置' : '未配置',
    lastTestAt: draft.id ? db.cloudNodes.find((item) => item.id === draft.id)?.lastTestAt : undefined,
    status: draft.token ? '鉴权正常' : '待鉴权',
    tone: draft.token ? 'success' : 'warning',
    notes: draft.notes,
  };

  db.cloudNodes = draft.id
    ? db.cloudNodes.map((item) => (item.id === draft.id ? record : item))
    : [record, ...db.cloudNodes];
}

function runMountScanInBrowser(db: StorageBrowserDb, ids: string[]) {
  db.mountFolders = db.mountFolders.map((item) =>
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
    const item = db.mountFolders.find((mount) => mount.id === id);
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
  db.mountFolders = db.mountFolders.map((item) =>
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
    .map((id) => db.mountFolders.find((item) => item.id === id))
    .filter((item): item is MountFolderRecord => Boolean(item))
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

function draftToMountFolderRecord(db: StorageBrowserDb, draft: MountFolderDraft): MountFolderRecord {
  const libraryMap = new Map(
    [
      ['photo', '商业摄影资产库'],
      ['video', '视频工作流资产库'],
      ['family', '家庭照片资产库'],
    ] as Array<[string, string]>,
  );

  if (draft.folderType === '本地') {
    return {
      id: draft.id ?? `mount-${Math.random().toString(36).slice(2, 8)}`,
      name: draft.name,
      libraryId: draft.libraryId,
      libraryName: libraryMap.get(draft.libraryId) ?? draft.libraryId,
      folderType: draft.folderType,
      address: draft.localPath,
      mountMode: draft.mountMode,
      enabled: true,
      scanStatus: '未扫描',
      scanTone: 'info',
      lastScanAt: '未扫描',
      heartbeatPolicy: draft.heartbeatPolicy,
      nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
      capacitySummary: '待首次检测',
      freeSpaceSummary: '待首次检测',
      capacityPercent: 0,
      riskTags: [],
      badges: ['本地', draft.mountMode].filter(Boolean) as string[],
      authStatus: '无需鉴权',
      authTone: 'info',
      notes: draft.notes,
    };
  }

  if (draft.folderType === 'NAS') {
    const nas = db.nasNodes.find((item) => item.id === draft.nasId);
    const folder = draft.targetFolder.trim().replace(/^[/\\]+/, '');
    return {
      id: draft.id ?? `mount-${Math.random().toString(36).slice(2, 8)}`,
      name: draft.name,
      libraryId: draft.libraryId,
      libraryName: libraryMap.get(draft.libraryId) ?? draft.libraryId,
      folderType: draft.folderType,
      sourceRefId: nas?.id,
      sourceName: nas?.name,
      address: folder ? `${nas?.address ?? ''}\\${folder}` : nas?.address ?? '',
      mountMode: draft.mountMode,
      enabled: true,
      scanStatus: '未扫描',
      scanTone: 'info',
      lastScanAt: '未扫描',
      heartbeatPolicy: draft.heartbeatPolicy,
      nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
      capacitySummary: '待首次检测',
      freeSpaceSummary: '待首次检测',
      capacityPercent: 0,
      riskTags: [],
      badges: [draft.mountMode, 'SMB'],
      authStatus: nas?.status ?? '待鉴权',
      authTone: nas?.tone ?? 'warning',
      notes: draft.notes,
    };
  }

  const cloud = db.cloudNodes.find((item) => item.id === draft.cloudId);
  const folder = draft.targetFolder.trim().replace(/^[/\\]+/, '');

  return {
    id: draft.id ?? `mount-${Math.random().toString(36).slice(2, 8)}`,
    name: draft.name,
    libraryId: draft.libraryId,
    libraryName: libraryMap.get(draft.libraryId) ?? draft.libraryId,
    folderType: draft.folderType,
    sourceRefId: cloud?.id,
    sourceName: cloud?.name,
    address: folder ? `${cloud?.mountDirectory ?? ''}/${folder}`.replace(/\/+/g, '/') : cloud?.mountDirectory ?? '',
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
    badges: [draft.mountMode, cloud?.vendor ?? '网盘'],
    authStatus: cloud?.status ?? '待鉴权',
    authTone: cloud?.tone ?? 'warning',
    notes: draft.notes,
  };
}

function createSeedBrowserDb(): StorageBrowserDb {
  const baseNasNodes: NasRecord[] = [
    {
      id: 'nas-main',
      name: '影像 NAS 01',
      address: '\\\\192.168.10.20\\media',
      username: 'mare-sync',
      passwordHint: '已保存，最近更新于 3 天前',
      lastTestAt: '今天 10:12',
      status: '鉴权正常',
      tone: 'success',
      notes: '主 NAS',
    },
    {
      id: 'nas-backup',
      name: '影像 NAS 备份柜',
      address: '\\\\192.168.10.36\\backup_media',
      username: 'mare-archive',
      passwordHint: '已保存，最近更新于 7 天前',
      lastTestAt: '昨天 22:18',
      status: '鉴权正常',
      tone: 'success',
      notes: '备份 NAS',
    },
  ];
  const extraNasNodes: NasRecord[] = Array.from({ length: 22 }, (_, index): NasRecord => ({
      id: `nas-extra-${index + 1}`,
      name: `项目 NAS ${index + 1}`,
      address: `\\\\192.168.20.${100 + index}\\share_${index + 1}`,
      username: `project-user-${index + 1}`,
      passwordHint: `已保存，最近更新于 ${index + 1} 天前`,
      lastTestAt: `昨天 ${String((index % 9) + 10).padStart(2, '0')}:00`,
      status: index % 4 === 0 ? '鉴权异常' : '鉴权正常',
      tone: index % 4 === 0 ? 'warning' : 'success',
      notes: `项目 NAS ${index + 1}`,
    }));
  const nasNodes: NasRecord[] = baseNasNodes.concat(extraNasNodes);

  const baseCloudNodes: CloudRecord[] = [
    {
      id: 'cloud-archive',
      name: '115 云归档',
      vendor: '115',
      accessMethod: '填入 Token',
      accountAlias: 'mare-archive',
      mountDirectory: '/MareArchive',
      tokenStatus: '48 小时内过期',
      lastTestAt: '今天 08:40',
      status: '鉴权异常',
      tone: 'warning',
      notes: '云归档空间',
    },
    {
      id: 'cloud-exchange',
      name: '115 项目交换区',
      vendor: '115',
      accessMethod: '扫码登录获取 Token',
      qrChannel: '微信小程序',
      accountAlias: 'mare-exchange',
      mountDirectory: '/ProjectExchange',
      tokenStatus: '已配置',
      lastTestAt: '今天 09:05',
      status: '鉴权正常',
      tone: 'success',
      notes: '项目交换空间',
    },
  ];
  const extraCloudNodes: CloudRecord[] = Array.from({ length: 22 }, (_, index): CloudRecord => ({
      id: `cloud-extra-${index + 1}`,
      name: `115 分区 ${index + 1}`,
      vendor: '115' as const,
      accessMethod: index % 2 === 0 ? '填入 Token' : '扫码登录获取 Token',
      qrChannel: index % 2 === 0 ? undefined : (index % 3 === 0 ? '支付宝小程序' : '微信小程序'),
      accountAlias: `cloud-alias-${index + 1}`,
      mountDirectory: `/ProjectSpace/${index + 1}`,
      tokenStatus: index % 5 === 0 ? '即将过期' : '已配置',
      lastTestAt: `今天 ${String((index % 10) + 8).padStart(2, '0')}:30`,
      status: index % 5 === 0 ? '鉴权异常' : '鉴权正常',
      tone: index % 5 === 0 ? 'warning' : 'success',
      notes: `网盘空间 ${index + 1}`,
    }));
  const cloudNodes: CloudRecord[] = baseCloudNodes.concat(extraCloudNodes);

  const baseMountFolders: MountFolderRecord[] = [
    {
      id: 'mount-local-main',
      name: '商业摄影原片库',
      libraryId: 'photo',
      libraryName: '商业摄影资产库',
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
    {
      id: 'mount-nas-main',
      name: '视频工作流 NAS 挂载',
      libraryId: 'video',
      libraryName: '视频工作流资产库',
      folderType: 'NAS',
      sourceRefId: 'nas-main',
      sourceName: '影像 NAS 01',
      address: '\\\\192.168.10.20\\media\\video_workflow',
      mountMode: '可写',
      enabled: true,
      scanStatus: '等待队列',
      scanTone: 'info',
      lastScanAt: '2 分钟前',
      heartbeatPolicy: '每日（深夜）',
      nextHeartbeatAt: '今晚 02:00',
      capacitySummary: '已用 48% · 18.9 TB 可用',
      freeSpaceSummary: '18.9 TB 可用',
      capacityPercent: 48,
      riskTags: [],
      badges: ['可写', 'SMB'],
      authStatus: '鉴权正常',
      authTone: 'success',
      notes: '视频资产库主挂载目录',
    },
    {
      id: 'mount-cloud-archive',
      name: '家庭照片网盘归档',
      libraryId: 'family',
      libraryName: '家庭照片资产库',
      folderType: '网盘',
      sourceRefId: 'cloud-archive',
      sourceName: '115 云归档',
      address: '/MareArchive/family_album',
      mountMode: '可写',
      enabled: true,
      scanStatus: '最近扫描失败',
      scanTone: 'critical',
      lastScanAt: '今天 07:40',
      heartbeatPolicy: '每周（深夜）',
      nextHeartbeatAt: '周六 02:00',
      capacitySummary: '远端容量正常 · 约 37% 已使用',
      freeSpaceSummary: '远端容量正常',
      capacityPercent: 37,
      riskTags: ['扫描失败', '鉴权异常'],
      badges: ['115', '可写'],
      authStatus: 'Token 48 小时内过期',
      authTone: 'warning',
      notes: '家庭照片网盘归档目录',
    },
  ];
  const extraMountFolders: MountFolderRecord[] = Array.from({ length: 24 }, (_, index): MountFolderRecord => ({
      id: `mount-extra-${index + 1}`,
      name: `扩展挂载 ${index + 1}`,
      libraryId: index % 3 === 0 ? 'photo' : index % 3 === 1 ? 'video' : 'family',
      libraryName: index % 3 === 0 ? '商业摄影资产库' : index % 3 === 1 ? '视频工作流资产库' : '家庭照片资产库',
      folderType: index % 3 === 0 ? '本地' as const : index % 3 === 1 ? 'NAS' as const : '网盘' as const,
      sourceRefId: index % 3 === 1 ? `nas-extra-${(index % 22) + 1}` : index % 3 === 2 ? `cloud-extra-${(index % 22) + 1}` : undefined,
      sourceName: index % 3 === 1 ? `项目 NAS ${(index % 22) + 1}` : index % 3 === 2 ? `115 分区 ${(index % 22) + 1}` : undefined,
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
  const mountFolders: MountFolderRecord[] = baseMountFolders.concat(extraMountFolders);

  return {
    mountFolders,
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
