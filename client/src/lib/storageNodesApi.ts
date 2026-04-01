export type StorageNodeType = '本机磁盘' | 'NAS/SMB' | '网盘';
export type StorageMountMode = '只读' | '可写';
export type StorageHeartbeatPolicy = '从不' | '每周（深夜）' | '每日（深夜）' | '每小时';
export type StorageTone = 'success' | 'warning' | 'critical' | 'info';
export type StorageCloudAccessMethod = '填入 Token' | '扫码登录获取 Token';
export type StorageCloudQrChannel = '微信小程序' | '支付宝小程序' | '电视端';

export type StorageNodeRecord = {
  id: string;
  name: string;
  nodeType: StorageNodeType;
  address: string;
  mountMode: StorageMountMode;
  enabled: boolean;
  scanStatus: string;
  scanTone: StorageTone;
  lastScanAt: string;
  heartbeatPolicy: StorageHeartbeatPolicy;
  nextHeartbeatAt: string;
  lastHeartbeatResult: string;
  heartbeatFailures: number;
  capacitySummary: string;
  freeSpaceSummary: string;
  capacityPercent: number;
  libraryBindings: string[];
  badges: string[];
  riskTags: string[];
  authStatus: string;
  authTone: StorageTone;
  notes: string;
  detail:
    | {
        kind: 'local';
        rootPath: string;
      }
    | {
        kind: 'nas';
        protocol: 'SMB';
        host: string;
        shareName: string;
        username: string;
        passwordHint: string;
      }
      | {
          kind: 'cloud';
          vendor: '115';
          accountAlias: string;
          mountDirectory: string;
          accessMethod: StorageCloudAccessMethod;
          qrChannel?: StorageCloudQrChannel;
          tokenStatus: string;
        };
};

export type StorageNodeDraft = {
  id?: string;
  name: string;
  nodeType: StorageNodeType;
  notes: string;
  mountMode: StorageMountMode;
  heartbeatPolicy: StorageHeartbeatPolicy;
  detail:
    | {
        kind: 'local';
        rootPath: string;
      }
    | {
        kind: 'nas';
        protocol: 'SMB';
        host: string;
        shareName: string;
        username: string;
        password: string;
      }
    | {
        kind: 'cloud';
        vendor: '115';
        accountAlias: string;
        mountDirectory: string;
        accessMethod: StorageCloudAccessMethod;
        qrChannel: StorageCloudQrChannel;
        token: string;
      };
};

export type StorageCredentialDraft = {
  id: string;
  nodeName: string;
  authMode: '账号密码' | StorageCloudAccessMethod;
  username: string;
  password: string;
  token: string;
  qrChannel: StorageCloudQrChannel;
};

export type StorageConnectionTestResult = {
  nodeId: string;
  nodeName: string;
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

export type StorageNodesDashboard = {
  nodes: StorageNodeRecord[];
};

type StorageBrowserDb = {
  histories: Record<string, StorageScanHistoryItem[]>;
  nodes: StorageNodeRecord[];
};

const BROWSER_STORAGE_KEY = 'mare-storage-nodes-browser-v2';

export const storageNodesApi = {
  async loadDashboard(): Promise<StorageNodesDashboard> {
    return invokeStorageCommand<StorageNodesDashboard>('storage_nodes_load_dashboard', {});
  },

  async saveNode({ draft }: { draft: StorageNodeDraft }): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_save_node', { draft });
  },

  async runScan({ ids }: { ids: string[] }): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_run_scan', { payload: { ids } });
  },

  async runConnectionTest({ ids }: { ids: string[] }): Promise<{ message: string; results: StorageConnectionTestResult[] }> {
    return invokeStorageCommand<{ message: string; results: StorageConnectionTestResult[] }>('storage_nodes_run_connection_test', { payload: { ids } });
  },

  async updateHeartbeat({
    heartbeatPolicy,
    ids,
  }: {
    heartbeatPolicy: StorageHeartbeatPolicy;
    ids: string[];
  }): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_update_heartbeat', { payload: { heartbeatPolicy, ids } });
  },

  async saveCredentials(draft: StorageCredentialDraft): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_save_credentials', { draft });
  },

  async updateEnabled({ enabled, ids }: { enabled: boolean; ids: string[] }): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_update_enabled', { payload: { enabled, ids } });
  },

  async deleteNode({ id }: { id: string }): Promise<{ message: string }> {
    return invokeStorageCommand<{ message: string }>('storage_nodes_delete_node', { payload: { id } });
  },

  async loadScanHistory({ id }: { id: string }): Promise<{ items: StorageScanHistoryItem[]; nodeId: string }> {
    return invokeStorageCommand<{ items: StorageScanHistoryItem[]; nodeId: string }>('storage_nodes_load_scan_history', { payload: { id } });
  },
};

async function invokeStorageCommand<T>(command: string, payload: Record<string, unknown>): Promise<T> {
  if (isTauriRuntime()) {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke<T>(command, payload);
  }

  return runBrowserFallback<T>(command, payload);
}

function isTauriRuntime() {
  return typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window;
}

function runBrowserFallback<T>(command: string, payload: Record<string, unknown>): T {
  const db = loadBrowserDb();
  const innerPayload = (payload.payload as Record<string, unknown> | undefined) ?? payload;

  switch (command) {
    case 'storage_nodes_load_dashboard':
      return structuredClone({ nodes: db.nodes }) as T;
    case 'storage_nodes_save_node':
      saveNodeInBrowser(db, payload.draft as StorageNodeDraft);
      persistBrowserDb(db);
      return { message: '存储节点已保存' } as T;
    case 'storage_nodes_run_scan':
      runScanInBrowser(db, innerPayload.ids as string[]);
      persistBrowserDb(db);
      return { message: `已为 ${(innerPayload.ids as string[]).length} 个节点创建扫描任务` } as T;
    case 'storage_nodes_run_connection_test':
      return {
        message: (innerPayload.ids as string[]).length > 1 ? `已完成 ${(innerPayload.ids as string[]).length} 个节点的连接测试` : '连接测试已完成',
        results: buildConnectionResults(db.nodes, innerPayload.ids as string[]),
      } as T;
    case 'storage_nodes_update_heartbeat':
      updateHeartbeatInBrowser(db, innerPayload.ids as string[], innerPayload.heartbeatPolicy as StorageHeartbeatPolicy);
      persistBrowserDb(db);
      return {
        message: (innerPayload.ids as string[]).length > 1 ? `已更新 ${(innerPayload.ids as string[]).length} 个节点的心跳策略` : '心跳策略已更新',
      } as T;
    case 'storage_nodes_save_credentials':
      saveCredentialsInBrowser(db, payload.draft as StorageCredentialDraft);
      persistBrowserDb(db);
      return { message: '鉴权信息已保存' } as T;
    case 'storage_nodes_update_enabled':
      updateEnabledInBrowser(db, innerPayload.ids as string[], innerPayload.enabled as boolean);
      persistBrowserDb(db);
      return { message: (innerPayload.enabled as boolean) ? '已启用所选节点' : '已停用所选节点' } as T;
    case 'storage_nodes_delete_node':
      deleteNodeInBrowser(db, innerPayload.id as string);
      persistBrowserDb(db);
      return { message: '节点已删除' } as T;
    case 'storage_nodes_load_scan_history':
      return {
        nodeId: innerPayload.id as string,
        items: structuredClone(db.histories[innerPayload.id as string] ?? []),
      } as T;
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
    if (!Array.isArray(parsed.nodes) || parsed.nodes.length === 0) {
      throw new Error('empty storage nodes');
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

function saveNodeInBrowser(db: StorageBrowserDb, draft: StorageNodeDraft) {
  const record = draftToRecord(draft);
  const nextNodes = draft.id
    ? db.nodes.map((node) => (node.id === draft.id ? { ...record, id: draft.id } : node))
    : [record, ...db.nodes];

  db.nodes = nextNodes;

  if (!draft.id && !db.histories[record.id]) {
    db.histories[record.id] = [];
  }
}

function runScanInBrowser(db: StorageBrowserDb, ids: string[]) {
  db.nodes = db.nodes.map((node) =>
    ids.includes(node.id)
      ? {
          ...node,
          scanStatus: '扫描中',
          scanTone: 'warning',
          lastScanAt: '正在执行',
        }
      : node,
  );

  ids.forEach((id) => {
    const history = db.histories[id] ?? [];
    db.histories[id] = [
      {
        id: `history-${Math.random().toString(36).slice(2, 8)}`,
        startedAt: '刚刚',
        finishedAt: '进行中',
        status: '进行中',
        summary: '扫描任务已创建，可继续浏览其他节点。',
        trigger: ids.length > 1 ? '批量扫描' : '手动扫描',
      },
      ...history,
    ];
  });
}

function updateHeartbeatInBrowser(db: StorageBrowserDb, ids: string[], heartbeatPolicy: StorageHeartbeatPolicy) {
  db.nodes = db.nodes.map((node) =>
    ids.includes(node.id)
      ? {
          ...node,
          heartbeatPolicy,
          nextHeartbeatAt: heartbeatPolicy === '从不' ? '—' : heartbeatPolicy === '每小时' ? '1 小时后' : heartbeatPolicy === '每日（深夜）' ? '今晚 02:00' : '周六 02:00',
          lastHeartbeatResult: heartbeatPolicy === '从不' ? '无需心跳' : node.lastHeartbeatResult,
        }
      : node,
  );
}

function saveCredentialsInBrowser(db: StorageBrowserDb, draft: StorageCredentialDraft) {
  db.nodes = db.nodes.map((node) =>
    node.id === draft.id
      ? {
          ...node,
          authStatus: '鉴权正常',
          authTone: 'success',
          riskTags: node.riskTags.filter((tag) => tag !== '鉴权异常'),
          detail:
            node.detail.kind === 'cloud'
              ? {
                  ...node.detail,
                  accessMethod: draft.authMode === '账号密码' ? node.detail.accessMethod : draft.authMode,
                  qrChannel: draft.authMode === '扫码登录获取 Token' ? draft.qrChannel : node.detail.qrChannel,
                  tokenStatus: draft.token ? '已更新' : node.detail.tokenStatus,
                }
              : node.detail,
        }
      : node,
  );
}

function updateEnabledInBrowser(db: StorageBrowserDb, ids: string[], enabled: boolean) {
  db.nodes = db.nodes.map((node) =>
    ids.includes(node.id)
      ? {
          ...node,
          enabled,
        }
      : node,
  );
}

function deleteNodeInBrowser(db: StorageBrowserDb, id: string) {
  db.nodes = db.nodes.filter((node) => node.id !== id);
  delete db.histories[id];
}

function buildConnectionResults(nodes: StorageNodeRecord[], ids: string[]): StorageConnectionTestResult[] {
  return ids
    .map((id) => nodes.find((node) => node.id === id))
    .filter((node): node is StorageNodeRecord => Boolean(node))
    .map((node) => ({
      nodeId: node.id,
      nodeName: node.name,
      overallTone: node.authTone === 'critical' ? 'critical' : node.authTone === 'warning' || node.riskTags.length > 0 ? 'warning' : 'success',
      summary: node.authTone === 'warning' || node.riskTags.length > 0 ? '当前连接可达，但仍建议先处理风险提示。' : '节点可达且当前配置可继续使用。',
      checks: [
        { label: '可达性', status: 'success', detail: `${node.address} 可达。` },
        { label: '鉴权状态', status: node.authTone, detail: node.authStatus },
        { label: '读权限', status: 'success', detail: '可读取目标目录。' },
        { label: '写权限', status: node.mountMode === '只读' ? 'warning' : 'success', detail: node.mountMode === '只读' ? '当前为只读挂载。' : '可写入目标目录。' },
        { label: '目标目录可访问', status: 'success', detail: '目录检查通过。' },
      ],
      suggestion: node.authTone === 'warning' || node.authTone === 'critical' ? '重新鉴权' : '可立即执行扫描',
      testedAt: '刚刚',
    }));
}

function draftToRecord(draft: StorageNodeDraft): StorageNodeRecord {
  if (draft.detail.kind === 'local') {
    return {
      id: draft.id ?? `node-${Math.random().toString(36).slice(2, 8)}`,
      name: draft.name,
      nodeType: draft.nodeType,
      address: draft.detail.rootPath,
      mountMode: draft.mountMode,
      enabled: true,
      scanStatus: '未扫描',
      scanTone: 'info',
      lastScanAt: '未扫描',
      heartbeatPolicy: draft.heartbeatPolicy,
      nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
      lastHeartbeatResult: draft.heartbeatPolicy === '从不' ? '无需心跳' : '尚未执行',
      heartbeatFailures: 0,
      capacitySummary: '待首次检测',
      freeSpaceSummary: '待首次检测',
      capacityPercent: 0,
      libraryBindings: [],
      badges: [draft.mountMode],
      riskTags: [],
      authStatus: '无需鉴权',
      authTone: 'info',
      notes: draft.notes,
      detail: {
        kind: 'local',
        rootPath: draft.detail.rootPath,
      },
    };
  }

  if (draft.detail.kind === 'nas') {
    return {
      id: draft.id ?? `node-${Math.random().toString(36).slice(2, 8)}`,
      name: draft.name,
      nodeType: draft.nodeType,
      address: `\\\\${draft.detail.host}\\${draft.detail.shareName}`,
      mountMode: draft.mountMode,
      enabled: true,
      scanStatus: '未扫描',
      scanTone: 'info',
      lastScanAt: '未扫描',
      heartbeatPolicy: draft.heartbeatPolicy,
      nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
      lastHeartbeatResult: '尚未执行',
      heartbeatFailures: 0,
      capacitySummary: '待首次检测',
      freeSpaceSummary: '待首次检测',
      capacityPercent: 0,
      libraryBindings: [],
      badges: [draft.mountMode, 'SMB'],
      riskTags: [],
      authStatus: draft.detail.username ? '待连接测试' : '待鉴权',
      authTone: draft.detail.username ? 'info' : 'warning',
      notes: draft.notes,
      detail: {
        kind: 'nas',
        protocol: 'SMB',
        host: draft.detail.host,
        shareName: draft.detail.shareName,
        username: draft.detail.username,
        passwordHint: draft.detail.password ? '刚刚更新' : '未保存',
      },
    };
  }

  return {
    id: draft.id ?? `node-${Math.random().toString(36).slice(2, 8)}`,
    name: draft.name,
    nodeType: draft.nodeType,
    address: draft.detail.mountDirectory,
    mountMode: draft.mountMode,
    enabled: true,
    scanStatus: '未扫描',
    scanTone: 'info',
    lastScanAt: '未扫描',
    heartbeatPolicy: draft.heartbeatPolicy,
    nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
    lastHeartbeatResult: '尚未执行',
    heartbeatFailures: 0,
    capacitySummary: '远端容量待检测',
    freeSpaceSummary: '待首次检测',
    capacityPercent: 0,
    libraryBindings: [],
    badges: ['115', draft.mountMode],
    riskTags: draft.detail.token ? [] : ['鉴权异常'],
    authStatus:
      draft.detail.accessMethod === '扫码登录获取 Token'
        ? draft.detail.token
          ? `已通过${draft.detail.qrChannel}获取 Token`
          : `待完成${draft.detail.qrChannel}扫码登录`
        : draft.detail.token
          ? '待连接测试'
          : 'Token 缺失',
    authTone: draft.detail.token ? 'info' : 'warning',
    notes: draft.notes,
    detail: {
      kind: 'cloud',
      vendor: '115',
      accountAlias: draft.detail.accountAlias,
      mountDirectory: draft.detail.mountDirectory,
      accessMethod: draft.detail.accessMethod,
      qrChannel: draft.detail.accessMethod === '扫码登录获取 Token' ? draft.detail.qrChannel : undefined,
      tokenStatus: draft.detail.token ? '已配置' : '未配置',
    },
  };
}

function createSeedBrowserDb(): StorageBrowserDb {
  return {
    nodes: [
      {
        id: 'node-local-main',
        name: '本地 NVMe 主盘',
        nodeType: '本机磁盘',
        address: 'D:\\Mare\\Assets',
        mountMode: '可写',
        enabled: true,
        scanStatus: '最近扫描成功',
        scanTone: 'success',
        lastScanAt: '今天 09:12',
        heartbeatPolicy: '从不',
        nextHeartbeatAt: '—',
        lastHeartbeatResult: '无需心跳',
        heartbeatFailures: 0,
        capacitySummary: '已用 64% · 3.4 TB 可用',
        freeSpaceSummary: '3.4 TB 可用',
        capacityPercent: 64,
        libraryBindings: ['商业摄影资产库', '视频工作流资产库'],
        badges: ['可写', '已绑定 2 个资产库'],
        riskTags: [],
        authStatus: '无需鉴权',
        authTone: 'info',
        notes: '本地生产主盘',
        detail: {
          kind: 'local',
          rootPath: 'D:\\Mare\\Assets',
        },
      },
      {
        id: 'node-removable',
        name: '现场移动硬盘 T7',
        nodeType: '本机磁盘',
        address: 'E:\\shooting-card',
        mountMode: '只读',
        enabled: true,
        scanStatus: '未扫描',
        scanTone: 'info',
        lastScanAt: '未扫描',
        heartbeatPolicy: '从不',
        nextHeartbeatAt: '—',
        lastHeartbeatResult: '无需心跳',
        heartbeatFailures: 0,
        capacitySummary: '已用 82% · 1.2 TB 可用',
        freeSpaceSummary: '1.2 TB 可用',
        capacityPercent: 82,
        libraryBindings: ['商业摄影资产库'],
        badges: ['只读', '移动设备'],
        riskTags: ['空间风险'],
        authStatus: '无需鉴权',
        authTone: 'info',
        notes: '现场素材源盘',
        detail: {
          kind: 'local',
          rootPath: 'E:\\shooting-card',
        },
      },
      {
        id: 'node-nas-main',
        name: '影像 NAS 01',
        nodeType: 'NAS/SMB',
        address: '\\\\192.168.10.20\\media',
        mountMode: '可写',
        enabled: true,
        scanStatus: '等待队列',
        scanTone: 'info',
        lastScanAt: '2 分钟前',
        heartbeatPolicy: '每日（深夜）',
        nextHeartbeatAt: '今晚 02:00',
        lastHeartbeatResult: '上次成功',
        heartbeatFailures: 0,
        capacitySummary: '已用 48% · 18.9 TB 可用',
        freeSpaceSummary: '18.9 TB 可用',
        capacityPercent: 48,
        libraryBindings: ['商业摄影资产库', '视频工作流资产库'],
        badges: ['可写', 'SMB'],
        riskTags: [],
        authStatus: '鉴权正常',
        authTone: 'success',
        notes: '影像主 NAS',
        detail: {
          kind: 'nas',
          protocol: 'SMB',
          host: '192.168.10.20',
          shareName: 'media',
          username: 'mare-sync',
          passwordHint: '已保存，最近更新于 3 天前',
        },
      },
      {
        id: 'node-nas-backup',
        name: '影像 NAS 备份柜',
        nodeType: 'NAS/SMB',
        address: '\\\\192.168.10.36\\backup_media',
        mountMode: '只读',
        enabled: true,
        scanStatus: '最近扫描成功',
        scanTone: 'success',
        lastScanAt: '昨天 23:18',
        heartbeatPolicy: '每周（深夜）',
        nextHeartbeatAt: '周六 02:00',
        lastHeartbeatResult: '上次成功',
        heartbeatFailures: 0,
        capacitySummary: '已用 71% · 9.6 TB 可用',
        freeSpaceSummary: '9.6 TB 可用',
        capacityPercent: 71,
        libraryBindings: ['家庭照片资产库'],
        badges: ['只读', 'SMB'],
        riskTags: [],
        authStatus: '鉴权正常',
        authTone: 'success',
        notes: '历史归档备份节点',
        detail: {
          kind: 'nas',
          protocol: 'SMB',
          host: '192.168.10.36',
          shareName: 'backup_media',
          username: 'mare-archive',
          passwordHint: '已保存，最近更新于 7 天前',
        },
      },
      {
        id: 'node-cloud-115',
        name: '115 云归档',
        nodeType: '网盘',
        address: '/MareArchive',
        mountMode: '可写',
        enabled: true,
        scanStatus: '最近扫描失败',
        scanTone: 'critical',
        lastScanAt: '今天 07:40',
        heartbeatPolicy: '每周（深夜）',
        nextHeartbeatAt: '周六 02:00',
        lastHeartbeatResult: '连续 2 次失败',
        heartbeatFailures: 2,
        capacitySummary: '远端容量正常 · 约 37% 已使用',
        freeSpaceSummary: '远端容量正常',
        capacityPercent: 37,
        libraryBindings: ['商业摄影资产库'],
        badges: ['115', '可写'],
        riskTags: ['扫描失败', '鉴权异常'],
        authStatus: '令牌 48 小时内过期',
        authTone: 'warning',
        notes: '云归档节点',
        detail: {
          kind: 'cloud',
          vendor: '115',
          accountAlias: 'mare-archive',
          mountDirectory: '/MareArchive',
          accessMethod: '填入 Token',
          tokenStatus: '48 小时内过期',
        },
      },
      {
        id: 'node-cloud-project',
        name: '115 项目交换区',
        nodeType: '网盘',
        address: '/ProjectExchange',
        mountMode: '可写',
        enabled: true,
        scanStatus: '最近扫描成功',
        scanTone: 'success',
        lastScanAt: '今天 08:20',
        heartbeatPolicy: '每日（深夜）',
        nextHeartbeatAt: '今晚 02:00',
        lastHeartbeatResult: '上次成功',
        heartbeatFailures: 0,
        capacitySummary: '远端容量正常 · 约 22% 已使用',
        freeSpaceSummary: '远端容量正常',
        capacityPercent: 22,
        libraryBindings: ['视频工作流资产库'],
        badges: ['115', '可写'],
        riskTags: [],
        authStatus: '鉴权正常',
        authTone: 'success',
        notes: '项目交换临时空间',
        detail: {
          kind: 'cloud',
          vendor: '115',
          accountAlias: 'mare-exchange',
          mountDirectory: '/ProjectExchange',
          accessMethod: '扫码登录获取 Token',
          qrChannel: '微信小程序',
          tokenStatus: '已配置',
        },
      },
    ],
    histories: {
      'node-local-main': [
        {
          id: 'history-local-1',
          startedAt: '2026-03-31 01:50',
          finishedAt: '2026-03-31 02:12',
          status: '成功',
          summary: '新增 86 项，变更 4 项。',
          trigger: '手动扫描',
        },
      ],
      'node-removable': [],
      'node-nas-main': [
        {
          id: 'history-nas-1',
          startedAt: '2026-03-31 02:00',
          finishedAt: '2026-03-31 02:18',
          status: '成功',
          summary: '新增 218 项，变更 12 项，未发现异常。',
          trigger: '计划扫描',
        },
      ],
      'node-cloud-115': [
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
