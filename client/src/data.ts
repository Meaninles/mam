export type MainView =
  | 'file-center'
  | 'task-center'
  | 'issues'
  | 'storage-nodes'
  | 'settings'
  | 'import-center';

export type ThemeMode = 'light' | 'dark';
export type Severity = 'success' | 'warning' | 'critical' | 'info';
export type SettingsTab =
  | 'general'
  | 'workspace'
  | 'file-overview'
  | 'tag-management'
  | 'import-archive'
  | 'notifications'
  | 'issue-governance'
  | 'verification'
  | 'background-tasks'
  | 'appearance';
export type TaskTab = 'transfer' | 'other';
export type TransferBusinessType = 'IMPORT' | 'SYNC';
export type TransferSyncLinkType = 'COPY' | 'UPLOAD' | 'DOWNLOAD';
export type OtherTaskType = 'SCAN' | 'METADATA_EXTRACT' | 'VERIFY' | 'DELETE_CLEANUP';
export type TaskPriority = '高优先级' | '普通优先级' | '低优先级';
export type FileTypeFilter = '全部' | '文件夹' | '图片' | '视频' | '音频' | '文档';
export type StorageTypeFilter = '全部' | '本机磁盘' | '移动硬盘' | 'NAS/SMB' | '115网盘';
export type SettingControlType = 'toggle' | 'select' | 'input' | 'segmented';
export type AssetLifecycleState = 'ACTIVE' | 'PENDING_DELETE';
export type IssueCategory = '冲突' | '传输' | '校验' | '节点与权限' | '容量与资源' | '扫描与解析' | '清理与治理';
export type IssueNature = 'BLOCKING' | 'RISK';
export type IssueSourceDomain = '传输任务' | '其他任务' | '文件中心' | '存储节点' | '系统治理';
export type IssueStatus = '待处理' | '待确认' | '处理中' | '已延后' | '已忽略' | '已解决' | '已归档';
export type HeaderSignalType = 'DEVICE_INSERTED';
export type NoticeKind = 'ACTION_REQUIRED' | 'REMINDER';
export type NoticeSourceType = 'ISSUE' | 'JOB';
export type NoticeStatus = 'UNREAD' | 'READ' | 'JUMPED' | 'STALE';
export type NoticeSourceDomain = '异常中心' | '任务中心' | '导入中心' | '存储节点' | '系统提醒';
export type NoticeJumpTargetKind = 'issues' | 'task-center' | 'file-center' | 'storage-nodes' | 'import-center';

export interface NavigationItem {
  id: Exclude<MainView, 'import-center'>;
  label: string;
  badge?: string;
}

export interface Library {
  id: string;
  name: string;
  rootLabel: string;
  itemCount: string;
  health: string;
  storagePolicy: string;
}

export interface EndpointState {
  name: string;
  state: string;
  tone: Severity;
}

export interface FileNode {
  id: string;
  libraryId: string;
  parentId: string | null;
  type: 'folder' | 'file';
  lifecycleState: AssetLifecycleState;
  name: string;
  fileKind: FileTypeFilter;
  displayType: string;
  modifiedAt: string;
  size: string;
  path: string;
  endpoints: EndpointState[];
  metadata: Array<{ label: string; value: string }>;
}

export interface ImportSourceFile {
  id: string;
  batchId: string;
  name: string;
  type: FileTypeFilter;
  size: string;
  relativePath: string;
  selectedTargets: string[];
  status: string;
}

export interface ImportBatch {
  id: string;
  name: string;
  source: string;
  sourceType: string;
  fileCount: string;
  targetCandidates: string[];
  status: string;
  lastScannedAt: string;
}

export type ImportDeviceType = '读卡器' | '存储卡' | '移动硬盘' | 'U 盘';
export type ImportDeviceScanStatus = '待识别' | '扫描中' | '已完成' | '扫描失败';
export type ImportDeviceSessionStatus =
  | '待识别'
  | '扫描中'
  | '可导入'
  | '导入中'
  | '部分完成'
  | '异常待处理'
  | '已拔出';
export type ImportSessionStatus =
  | '待扫描'
  | '可导入'
  | '预检失败'
  | '待提交'
  | '导入中'
  | '部分成功'
  | '已完成'
  | '异常待处理'
  | '已拔出';
export type ImportDraftStatus = '草稿中' | '待提交' | '导入中' | '已提交';
export type ImportFileStatus = '待导入' | '已排队' | '传输中' | '校验中' | '已完成' | '失败' | '冲突' | '已跳过';
export type ImportCheckStatus = 'passed' | 'risk' | 'blocking';
export type ImportTargetEndpointType = '本机磁盘' | 'NAS/SMB' | '115网盘';

export interface ImportCapacitySummary {
  total: string;
  available: string;
  usedPercent: number;
}

export interface ImportTargetEndpointRecord {
  id: string;
  endpointId: string;
  label: string;
  type: ImportTargetEndpointType;
  writable: boolean;
  availableSpace: string;
  statusLabel: string;
  tone: Severity;
}

export interface ImportDeviceSessionRecord {
  id: string;
  deviceKey: string;
  deviceLabel: string;
  deviceType: ImportDeviceType;
  libraryId: string;
  mountPath: string;
  connectedAt: string;
  connectedAtSortKey: number;
  lastSeenAt: string;
  capacitySummary: ImportCapacitySummary;
  scanStatus: ImportDeviceScanStatus;
  sessionStatus: ImportDeviceSessionStatus;
  activeDraftId?: string;
  latestReportId?: string;
  issueIds: string[];
  fileCount: number;
  folderCount: number;
  duplicateCount: number;
  exceptionCount: number;
  description: string;
  availableTargetEndpointIds: string[];
}

export interface ImportPrecheckItem {
  id: string;
  label: string;
  status: ImportCheckStatus;
  detail: string;
}

export interface ImportPrecheckSummary {
  blockingCount: number;
  riskCount: number;
  passedCount: number;
  updatedAt: string;
  checks: {
    sourceReadable: ImportCheckStatus;
    targetWritable: ImportCheckStatus;
    capacityReady: ImportCheckStatus;
    pathConflict: ImportCheckStatus;
    deviceOnline: ImportCheckStatus;
    executorReady: ImportCheckStatus;
  };
  items: ImportPrecheckItem[];
}

export interface ImportDraftRecord {
  id: string;
  deviceSessionId: string;
  libraryId: string;
  selectedFileIds: string[];
  targetEndpointIds: string[];
  targetStrategy: string;
  precheckSummary: ImportPrecheckSummary;
  lastEditedAt: string;
  hasBlockingIssues: boolean;
  status: ImportDraftStatus;
}

export interface ImportSourceNodeRecord {
  id: string;
  deviceSessionId: string;
  name: string;
  relativePath: string;
  fileKind: FileTypeFilter;
  size: string;
  status: ImportFileStatus;
  targetEndpointIds: string[];
  issueIds: string[];
  note?: string;
}

export interface ImportReportTargetSummary {
  endpointId: string;
  label: string;
  status: string;
  successCount: number;
  failedCount: number;
  transferredSize: string;
}

export interface ImportReportSnapshot {
  id: string;
  deviceSessionId: string;
  taskId: string;
  title: string;
  status: '已排队' | '运行中' | '部分成功' | '失败' | '已完成';
  submittedAt: string;
  finishedAt?: string;
  successCount: number;
  failedCount: number;
  partialCount: number;
  verifySummary: string;
  targetSummaries: ImportReportTargetSummary[];
  issueIds: string[];
  latestUpdatedAt: string;
  fileCount: number;
  note?: string;
}

export interface TaskRecord {
  id: string;
  kind: TaskTab;
  title: string;
  type: string;
  otherTaskType?: OtherTaskType;
  backendTypes?: string[];
  businessType?: TransferBusinessType;
  syncLinkType?: TransferSyncLinkType;
  status: string;
  statusTone: Severity;
  libraryId: string;
  source?: string;
  target?: string;
  fileNodeIds?: string[];
  sourcePath?: string;
  targetPath?: string;
  progress: number;
  speed: string;
  eta: string;
  fileCount: number;
  folderCount?: number;
  totalSize?: string;
  totalSizeBytes?: number;
  pendingTotalSizeBytes?: number;
  multiFile: boolean;
  updatedAt?: string;
  priority?: TaskPriority;
  issueIds?: string[];
  creator?: string;
  createdAt?: string;
  startedAt?: string;
  finishedAt?: string;
  phaseLabel?: string;
  scopeLabel?: string;
  endpointLabel?: string;
  resultSummary?: string;
  waitingReason?: string;
  assetIds?: string[];
}

export interface TaskItemRecord {
  id: string;
  taskId: string;
  parentId?: string | null;
  fileNodeId?: string;
  name?: string;
  kind?: 'group' | 'folder' | 'file' | 'asset' | 'step';
  depth?: number;
  phase?: string;
  status: string;
  statusTone: Severity;
  priority?: TaskPriority;
  progress: number;
  size?: string;
  speed: string;
  sourcePath?: string;
  targetPath?: string;
  pathLabel?: string;
  resultLabel?: string;
  issueIds?: string[];
  invalidated?: boolean;
}

export interface IssueSource {
  sourceDomain: IssueSourceDomain;
  taskId?: string;
  taskItemId?: string;
  taskTitle?: string;
  taskItemTitle?: string;
  assetId?: string;
  fileNodeId?: string;
  endpointId?: string;
  endpointLabel?: string;
  path?: string;
  sourceLabel?: string;
  routeLabel?: string;
}

export interface IssueCapabilities {
  canRetry?: boolean;
  canConfirm?: boolean;
  canPostpone?: boolean;
  canIgnore?: boolean;
  canRefreshCheck?: boolean;
  canArchive?: boolean;
  canClearHistory?: boolean;
  canOpenTaskCenter?: boolean;
  canOpenFileCenter?: boolean;
  canOpenStorageNodes?: boolean;
}

export interface IssueHistoryRecord {
  id: string;
  issueId: string;
  action: string;
  operatorLabel: string;
  result: string;
  createdAt: string;
}

export interface IssueImpactSummary {
  assetCount: number;
  replicaCount: number;
  folderCount: number;
  endpointCount: number;
  blocksStatusCommit: boolean;
  blocksTaskExecution: boolean;
}

export interface IssueRetentionPolicy {
  maxHistoryCount: number;
  maxHistoryAgeDays: number;
  autoArchiveResolved: boolean;
  autoPurgeArchived: boolean;
}

export interface IssueRecord {
  id: string;
  libraryId: string;
  taskId?: string;
  taskItemId?: string;
  category: IssueCategory;
  type: string;
  nature: IssueNature;
  sourceDomain: IssueSourceDomain;
  severity: Severity;
  title: string;
  summary: string;
  asset: string;
  objectLabel: string;
  action: string;
  actionLabel: string;
  suggestion: string;
  detail: string;
  occurrenceCount?: number;
  status: IssueStatus;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
  archivedAt?: string;
  source: IssueSource;
  impact: IssueImpactSummary;
  capabilities: IssueCapabilities;
  histories: IssueHistoryRecord[];
}

export interface NoticeJumpParams {
  kind: NoticeJumpTargetKind;
  issueId?: string;
  taskId?: string;
  taskItemId?: string;
  libraryId?: string;
  endpointId?: string;
  fileNodeId?: string;
  path?: string;
  sourceDomain?: IssueSourceDomain;
  label?: string;
}

export interface HeaderSignal {
  id: string;
  signalType: HeaderSignalType;
  tone: Severity;
  title: string;
  summary: string;
  createdAt: string;
  expiresAt?: string;
  deviceId?: string;
  deviceName?: string;
  jumpParams: NoticeJumpParams;
}

export interface NoticeSource {
  sourceDomain: NoticeSourceDomain;
  issueCategory?: IssueCategory;
  issueNature?: IssueNature;
  issueSourceDomain?: IssueSourceDomain;
  taskId?: string;
  taskItemId?: string;
  fileNodeId?: string;
  endpointId?: string;
  path?: string;
  sourceLabel?: string;
  routeLabel?: string;
}

export interface NoticeCapabilities {
  canMarkRead?: boolean;
  canOpenIssueCenter?: boolean;
  canOpenTaskCenter?: boolean;
  canOpenFileCenter?: boolean;
  canOpenStorageNodes?: boolean;
  canOpenImportCenter?: boolean;
}

export interface NoticeRecord {
  id: string;
  kind: NoticeKind;
  sourceType: NoticeSourceType;
  sourceId: string;
  issueId?: string;
  title: string;
  summary: string;
  severity: Severity;
  libraryId?: string;
  objectLabel: string;
  status: NoticeStatus;
  createdAt: string;
  updatedAt: string;
  readAt?: string;
  jumpedAt?: string;
  sortKey: number;
  source: NoticeSource;
  capabilities: NoticeCapabilities;
  jumpParams: NoticeJumpParams;
}

export interface StorageNode {
  id: string;
  name: string;
  nodeType: StorageTypeFilter;
  address: string;
  mountMode: string;
  status: string;
  freeSpace: string;
  lastCheck: string;
  capacityPercent: number;
}

export interface SettingRow {
  id: string;
  label: string;
  value: string;
  control: SettingControlType;
  options?: string[];
  description?: string;
}

export interface SettingSection {
  id: string;
  title: string;
  rows: SettingRow[];
}

export const navigationItems: NavigationItem[] = [
  { id: 'file-center', label: '文件中心' },
  { id: 'task-center', label: '任务中心', badge: '14' },
  { id: 'issues', label: '异常中心', badge: '5' },
  { id: 'storage-nodes', label: '存储节点', badge: '4' },
  { id: 'settings', label: '设置' },
];

export const fileNodes: FileNode[] = [
  {
    id: 'photo-root-raw',
    libraryId: 'photo',
    parentId: null,
    type: 'folder',
    lifecycleState: 'ACTIVE',
    name: '拍摄原片',
    fileKind: '文件夹',
    displayType: '文件夹',
    modifiedAt: '今天 09:18',
    size: '2,184 项',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片',
    endpoints: [
      { name: '本地NVMe', state: '已索引', tone: 'success' },
      { name: '影像NAS', state: '已同步', tone: 'success' },
    ],
    metadata: [
      { label: '路径', value: '/2026/ShanghaiLaunch/raw' },
      { label: '子项目', value: '2,184' },
      { label: '同步策略', value: '双副本必达' },
      { label: '最近更新', value: '今天 09:18' },
    ],
  },
  {
    id: 'photo-root-delivery',
    libraryId: 'photo',
    parentId: null,
    type: 'folder',
    lifecycleState: 'ACTIVE',
    name: '精选交付',
    fileKind: '文件夹',
    displayType: '文件夹',
    modifiedAt: '昨天 21:10',
    size: '136 项',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 精选交付',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '已存在', tone: 'success' },
      { name: '115', state: '已归档', tone: 'success' },
    ],
    metadata: [
      { label: '路径', value: '/2026/ShanghaiLaunch/delivery' },
      { label: '子项目', value: '136' },
      { label: '同步策略', value: '交付后归档' },
      { label: '最近更新', value: '昨天 21:10' },
    ],
  },
  {
    id: 'photo-file-raw-001',
    libraryId: 'photo',
    parentId: 'photo-root-raw',
    type: 'file',
    lifecycleState: 'ACTIVE',
    name: '2026-03-29_上海发布会_A-cam_001.RAW',
    fileKind: '图片',
    displayType: 'RAW 图像',
    modifiedAt: '2026-03-29 18:42',
    size: '48.2 MB',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '已存在', tone: 'success' },
      { name: '115', state: '待同步', tone: 'warning' },
    ],
    metadata: [
      { label: '设备', value: 'Sony A7R V' },
      { label: '镜头', value: '24-70mm F2.8 GM II' },
      { label: '分辨率', value: '9504 × 6336' },
      { label: '曝光', value: '1/250 · f/2.8 · ISO 640' },
    ],
  },
  {
    id: 'photo-file-raw-002',
    libraryId: 'photo',
    parentId: 'photo-root-raw',
    type: 'file',
    lifecycleState: 'ACTIVE',
    name: '2026-03-29_上海发布会_B-cam_018.RAW',
    fileKind: '图片',
    displayType: 'RAW 图像',
    modifiedAt: '2026-03-29 18:49',
    size: '47.8 MB',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '同步中', tone: 'warning' },
      { name: '115', state: '未开始', tone: 'info' },
    ],
    metadata: [
      { label: '设备', value: 'Sony A7 IV' },
      { label: '镜头', value: '70-200mm F2.8 GM II' },
      { label: '分辨率', value: '7008 × 4672' },
      { label: '曝光', value: '1/200 · f/2.8 · ISO 800' },
    ],
  },
  {
    id: 'photo-file-cover',
    libraryId: 'photo',
    parentId: 'photo-root-delivery',
    type: 'file',
    lifecycleState: 'ACTIVE',
    name: '上海发布会_精选封面.jpg',
    fileKind: '图片',
    displayType: 'JPEG 图像',
    modifiedAt: '2026-03-30 00:24',
    size: '12.4 MB',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 精选交付',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '已存在', tone: 'success' },
      { name: '115', state: '已归档', tone: 'success' },
    ],
    metadata: [
      { label: '设备', value: 'Sony A7R V' },
      { label: '分辨率', value: '4096 × 2731' },
      { label: '评级', value: '五星' },
      { label: '颜色标记', value: '黄色' },
    ],
  },
  {
    id: 'photo-folder-delivery-pack',
    libraryId: 'photo',
    parentId: 'photo-root-delivery',
    type: 'folder',
    lifecycleState: 'ACTIVE',
    name: '发布会资料包',
    fileKind: '文件夹',
    displayType: '文件夹',
    modifiedAt: '昨天 20:40',
    size: '2 项',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 精选交付 / 发布会资料包',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '已存在', tone: 'success' },
      { name: '115', state: '部分同步', tone: 'warning' },
    ],
    metadata: [
      { label: '路径', value: '/2026/ShanghaiLaunch/delivery/package' },
      { label: '子项目', value: '2' },
      { label: '同步策略', value: '交付后归档' },
      { label: '最近更新', value: '昨天 20:40' },
    ],
  },
  {
    id: 'photo-file-gallery-zip',
    libraryId: 'photo',
    parentId: 'photo-folder-delivery-pack',
    type: 'file',
    lifecycleState: 'ACTIVE',
    name: '上海发布会_精选组图.zip',
    fileKind: '文档',
    displayType: 'ZIP 压缩包',
    modifiedAt: '2026-03-30 00:31',
    size: '1.8 GB',
    path: '商业摄影资产库 / 2026 / Shanghai Launch / 精选交付 / 发布会资料包',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '已存在', tone: 'success' },
      { name: '115', state: '未同步', tone: 'info' },
    ],
    metadata: [
      { label: '来源', value: '精选交付' },
      { label: '归档包', value: '发布会资料包' },
      { label: '校验摘要', value: '待生成' },
      { label: '备注', value: '供客户整包下载' },
    ],
  },
  {
    id: 'video-root-final',
    libraryId: 'video',
    parentId: null,
    type: 'folder',
    lifecycleState: 'ACTIVE',
    name: '成片交付',
    fileKind: '文件夹',
    displayType: '文件夹',
    modifiedAt: '今天 11:40',
    size: '46 项',
    path: '视频工作流资产库 / 2026 / Interview / final',
    endpoints: [
      { name: '本地NVMe', state: '已索引', tone: 'success' },
      { name: '影像NAS', state: '同步中', tone: 'warning' },
    ],
    metadata: [
      { label: '路径', value: '/2026/Interview/final' },
      { label: '子项目', value: '46' },
      { label: '同步策略', value: '交付前必须校验' },
      { label: '最近更新', value: '今天 11:40' },
    ],
  },
  {
    id: 'video-file-final',
    libraryId: 'video',
    parentId: 'video-root-final',
    type: 'file',
    lifecycleState: 'ACTIVE',
    name: '客户访谈_第一机位_精编版.mov',
    fileKind: '视频',
    displayType: 'ProRes 视频',
    modifiedAt: '2026-03-30 22:14',
    size: '12.8 GB',
    path: '视频工作流资产库 / 2026 / Interview / final',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '同步中', tone: 'warning' },
      { name: '115', state: '未开始', tone: 'info' },
    ],
    metadata: [
      { label: '分辨率', value: '3840 × 2160' },
      { label: '帧率', value: '25 fps' },
      { label: '时长', value: '23 分 14 秒' },
      { label: '编码', value: 'Apple ProRes 422 HQ' },
    ],
  },
  {
    id: 'video-file-audio',
    libraryId: 'video',
    parentId: null,
    type: 'file',
    lifecycleState: 'ACTIVE',
    name: '片头配乐_v4_master.wav',
    fileKind: '音频',
    displayType: 'WAV 音频',
    modifiedAt: '2026-03-28 10:05',
    size: '148 MB',
    path: '视频工作流资产库 / 2026 / Interview / audio',
    endpoints: [
      { name: '本地NVMe', state: '已存在', tone: 'success' },
      { name: '影像NAS', state: '校验失败', tone: 'critical' },
      { name: '115', state: '已存在', tone: 'success' },
    ],
    metadata: [
      { label: '采样率', value: '48 kHz' },
      { label: '位深', value: '24 bit' },
      { label: '时长', value: '3 分 42 秒' },
      { label: '声道', value: '立体声' },
    ],
  },
];

export const importBatches: ImportBatch[] = [
  {
    id: 'import-photo',
    name: '现场移动硬盘 T7 / 上海发布会',
    source: 'E:\\DCIM',
    sourceType: '移动硬盘',
    fileCount: '1,824 项 / 2.1 TB',
    targetCandidates: ['本地NVMe', '影像NAS', '115'],
    status: '待分发',
    lastScannedAt: '今天 10:08',
  },
  {
    id: 'import-audio',
    name: '录音卡 / 访谈音频',
    source: 'F:\\Audio',
    sourceType: '移动硬盘',
    fileCount: '94 项 / 18.4 GB',
    targetCandidates: ['本地NVMe', '影像NAS'],
    status: '待分发',
    lastScannedAt: '今天 08:34',
  },
];

export const importSourceFiles: ImportSourceFile[] = [
  {
    id: 'import-file-1',
    batchId: 'import-photo',
    name: 'A001_C001_0329.mov',
    type: '视频',
    size: '16.2 GB',
    relativePath: '/DCIM/CARD_01/A001_C001_0329.mov',
    selectedTargets: ['本地NVMe', '影像NAS'],
    status: '待提交',
  },
  {
    id: 'import-file-2',
    batchId: 'import-photo',
    name: 'A001_C001_0330_proxy.mp4',
    type: '视频',
    size: '2.1 GB',
    relativePath: '/DCIM/CARD_01/A001_C001_0330_proxy.mp4',
    selectedTargets: ['本地NVMe'],
    status: '待提交',
  },
  {
    id: 'import-file-3',
    batchId: 'import-photo',
    name: 'RAW_0001.ARW',
    type: '图片',
    size: '48.2 MB',
    relativePath: '/PHOTO/DAY1/RAW_0001.ARW',
    selectedTargets: ['本地NVMe', '115'],
    status: '待提交',
  },
  {
    id: 'import-file-4',
    batchId: 'import-audio',
    name: '访谈_环境底噪.wav',
    type: '音频',
    size: '218 MB',
    relativePath: '/Audio/roomtone.wav',
    selectedTargets: ['本地NVMe', '影像NAS'],
    status: '待提交',
  },
];

export const importTargetEndpoints: ImportTargetEndpointRecord[] = [
  {
    id: 'import-target-local',
    endpointId: 'node-1',
    label: '本地 NVMe 主盘',
    type: '本机磁盘',
    writable: true,
    availableSpace: '3.4 TB 可用',
    statusLabel: '可写',
    tone: 'success',
  },
  {
    id: 'import-target-nas',
    endpointId: 'node-2',
    label: '影像 NAS 01',
    type: 'NAS/SMB',
    writable: true,
    availableSpace: '18.9 TB 可用',
    statusLabel: '可写',
    tone: 'success',
  },
  {
    id: 'import-target-cloud',
    endpointId: 'node-4',
    label: '115 云归档',
    type: '115网盘',
    writable: false,
    availableSpace: '鉴权异常',
    statusLabel: '需修复鉴权',
    tone: 'warning',
  },
  {
    id: 'import-target-delivery',
    endpointId: 'node-2',
    label: '交付热目录',
    type: 'NAS/SMB',
    writable: true,
    availableSpace: '6.8 TB 可用',
    statusLabel: '可写',
    tone: 'info',
  },
];

export const importDeviceSessions: ImportDeviceSessionRecord[] = [
  {
    id: 'import-device-cfexpress-a',
    deviceKey: 'cfexpress-a',
    deviceLabel: 'CFexpress A 卡（A 机位）',
    deviceType: '存储卡',
    libraryId: 'photo',
    mountPath: 'E:\\DCIM',
    connectedAt: '今天 10:08',
    connectedAtSortKey: 1008,
    lastSeenAt: '刚刚',
    capacitySummary: {
      total: '960 GB',
      available: '118 GB',
      usedPercent: 88,
    },
    scanStatus: '已完成',
    sessionStatus: '可导入',
    activeDraftId: 'import-draft-cfexpress-a',
    issueIds: [],
    fileCount: 428,
    folderCount: 36,
    duplicateCount: 18,
    exceptionCount: 2,
    description: '现场主机位素材，目录扫描已完成，待确认最终分发目标。',
    availableTargetEndpointIds: ['import-target-local', 'import-target-nas', 'import-target-cloud'],
  },
  {
    id: 'import-device-t7',
    deviceKey: 't7-shanghai',
    deviceLabel: '现场移动硬盘 T7',
    deviceType: '移动硬盘',
    libraryId: 'photo',
    mountPath: 'G:\\ShanghaiLaunch',
    connectedAt: '今天 09:22',
    connectedAtSortKey: 922,
    lastSeenAt: '1 分钟前',
    capacitySummary: {
      total: '4 TB',
      available: '1.2 TB',
      usedPercent: 70,
    },
    scanStatus: '已完成',
    sessionStatus: '导入中',
    activeDraftId: 'import-draft-t7',
    latestReportId: 'import-report-t7-running',
    issueIds: ['issue-12'],
    fileCount: 1824,
    folderCount: 128,
    duplicateCount: 64,
    exceptionCount: 1,
    description: '大批量现场素材导入中，建议转到任务中心持续观察。',
    availableTargetEndpointIds: ['import-target-local', 'import-target-nas', 'import-target-cloud'],
  },
  {
    id: 'import-device-audio',
    deviceKey: 'audio-usb',
    deviceLabel: '录音 U 盘（访谈音频）',
    deviceType: 'U 盘',
    libraryId: 'video',
    mountPath: 'F:\\Audio',
    connectedAt: '今天 08:34',
    connectedAtSortKey: 834,
    lastSeenAt: '2 分钟前',
    capacitySummary: {
      total: '256 GB',
      available: '41 GB',
      usedPercent: 84,
    },
    scanStatus: '已完成',
    sessionStatus: '异常待处理',
    activeDraftId: 'import-draft-audio',
    issueIds: ['issue-6'],
    fileCount: 94,
    folderCount: 8,
    duplicateCount: 4,
    exceptionCount: 3,
    description: '音频素材已扫描，但存在目标冲突和校验风险，提交前需要先处理。',
    availableTargetEndpointIds: ['import-target-local', 'import-target-nas', 'import-target-delivery'],
  },
  {
    id: 'import-device-microsd-b',
    deviceKey: 'microsd-b',
    deviceLabel: 'microSD 卡（B 机位）',
    deviceType: '存储卡',
    libraryId: 'photo',
    mountPath: 'H:\\CardB',
    connectedAt: '今天 07:42',
    connectedAtSortKey: 742,
    lastSeenAt: '5 分钟前',
    capacitySummary: {
      total: '512 GB',
      available: '76 GB',
      usedPercent: 85,
    },
    scanStatus: '已完成',
    sessionStatus: '部分完成',
    activeDraftId: 'import-draft-microsd-b',
    latestReportId: 'import-report-microsd-partial',
    issueIds: ['issue-1'],
    fileCount: 312,
    folderCount: 24,
    duplicateCount: 12,
    exceptionCount: 5,
    description: '部分素材已完成入库，仍有失败与冲突待回看。',
    availableTargetEndpointIds: ['import-target-local', 'import-target-nas'],
  },
  {
    id: 'import-device-reader',
    deviceKey: 'dual-reader',
    deviceLabel: '双卡读卡器（新接入）',
    deviceType: '读卡器',
    libraryId: 'photo',
    mountPath: 'I:\\',
    connectedAt: '今天 11:06',
    connectedAtSortKey: 1106,
    lastSeenAt: '刚刚',
    capacitySummary: {
      total: '未知',
      available: '检测中',
      usedPercent: 0,
    },
    scanStatus: '扫描中',
    sessionStatus: '扫描中',
    activeDraftId: 'import-draft-reader',
    issueIds: [],
    fileCount: 0,
    folderCount: 0,
    duplicateCount: 0,
    exceptionCount: 0,
    description: '设备已识别，正在建立来源摘要和文件清单。',
    availableTargetEndpointIds: ['import-target-local', 'import-target-nas'],
  },
  {
    id: 'import-device-cfast-removed',
    deviceKey: 'cfast-backup',
    deviceLabel: 'CFast 卡（备机）',
    deviceType: '存储卡',
    libraryId: 'photo',
    mountPath: 'J:\\DCIM',
    connectedAt: '今天 06:58',
    connectedAtSortKey: 658,
    lastSeenAt: '22 分钟前',
    capacitySummary: {
      total: '256 GB',
      available: '已拔出',
      usedPercent: 0,
    },
    scanStatus: '已完成',
    sessionStatus: '已拔出',
    activeDraftId: 'import-draft-cfast-removed',
    latestReportId: 'import-report-cfast-complete',
    issueIds: [],
    fileCount: 206,
    folderCount: 14,
    duplicateCount: 0,
    exceptionCount: 0,
    description: '设备已拔出，但保留最近一次导入结果和草稿快照。',
    availableTargetEndpointIds: ['import-target-local', 'import-target-nas'],
  },
];

export const importDrafts: ImportDraftRecord[] = [
  {
    id: 'import-draft-cfexpress-a',
    deviceSessionId: 'import-device-cfexpress-a',
    libraryId: 'photo',
    selectedFileIds: ['import-node-cf-a-1', 'import-node-cf-a-2', 'import-node-cf-a-3', 'import-node-cf-a-4'],
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    targetStrategy: '原片入本地与 NAS，交付代理文件视情况补传云归档',
    lastEditedAt: '今天 10:12',
    hasBlockingIssues: false,
    status: '待提交',
    precheckSummary: {
      blockingCount: 0,
      riskCount: 1,
      passedCount: 5,
      updatedAt: '刚刚',
      checks: {
        sourceReadable: 'passed',
        targetWritable: 'passed',
        capacityReady: 'passed',
        pathConflict: 'risk',
        deviceOnline: 'passed',
        executorReady: 'passed',
      },
      items: [
        { id: 'cf-a-check-1', label: '来源可读', status: 'passed', detail: '设备在线，目录扫描与抽样读取正常。' },
        { id: 'cf-a-check-2', label: '目标可写', status: 'passed', detail: '本地 NVMe 与影像 NAS 均可写。' },
        { id: 'cf-a-check-3', label: '目标容量', status: 'passed', detail: '当前目标端容量足够本次导入。' },
        { id: 'cf-a-check-4', label: '路径冲突', status: 'risk', detail: '发现 2 个同名代理文件，建议提交前确认保留策略。' },
        { id: 'cf-a-check-5', label: '设备在线', status: 'passed', detail: '最近心跳为刚刚。' },
        { id: 'cf-a-check-6', label: '执行器可用', status: 'passed', detail: '本地执行器与默认校验器均可用。' },
      ],
    },
  },
  {
    id: 'import-draft-t7',
    deviceSessionId: 'import-device-t7',
    libraryId: 'photo',
    selectedFileIds: ['import-node-t7-1', 'import-node-t7-2', 'import-node-t7-3', 'import-node-t7-4'],
    targetEndpointIds: ['import-target-local', 'import-target-nas', 'import-target-cloud'],
    targetStrategy: '现场硬盘全量入库，多目标同时分发并保留默认校验。',
    lastEditedAt: '今天 09:23',
    hasBlockingIssues: false,
    status: '导入中',
    precheckSummary: {
      blockingCount: 0,
      riskCount: 0,
      passedCount: 6,
      updatedAt: '今天 09:22',
      checks: {
        sourceReadable: 'passed',
        targetWritable: 'passed',
        capacityReady: 'passed',
        pathConflict: 'passed',
        deviceOnline: 'passed',
        executorReady: 'passed',
      },
      items: [
        { id: 't7-check-1', label: '来源可读', status: 'passed', detail: '现场移动硬盘随机抽样通过。' },
        { id: 't7-check-2', label: '目标可写', status: 'passed', detail: '本地、NAS 与云归档均已完成写入测试。' },
        { id: 't7-check-3', label: '目标容量', status: 'passed', detail: '三处目标端容量均满足。' },
        { id: 't7-check-4', label: '路径冲突', status: 'passed', detail: '未检测到正式冲突。' },
        { id: 't7-check-5', label: '设备在线', status: 'passed', detail: '设备仍保持在线。' },
        { id: 't7-check-6', label: '执行器可用', status: 'passed', detail: '已分配原生传输执行器。' },
      ],
    },
  },
  {
    id: 'import-draft-audio',
    deviceSessionId: 'import-device-audio',
    libraryId: 'video',
    selectedFileIds: ['import-node-audio-1', 'import-node-audio-2', 'import-node-audio-3'],
    targetEndpointIds: ['import-target-local', 'import-target-delivery'],
    targetStrategy: '先入本地 NVMe，再按访谈目录策略同步到交付热目录。',
    lastEditedAt: '今天 08:41',
    hasBlockingIssues: true,
    status: '草稿中',
    precheckSummary: {
      blockingCount: 2,
      riskCount: 1,
      passedCount: 3,
      updatedAt: '2 分钟前',
      checks: {
        sourceReadable: 'passed',
        targetWritable: 'passed',
        capacityReady: 'passed',
        pathConflict: 'blocking',
        deviceOnline: 'passed',
        executorReady: 'risk',
      },
      items: [
        { id: 'audio-check-1', label: '来源可读', status: 'passed', detail: '音频目录读取正常。' },
        { id: 'audio-check-2', label: '目标可写', status: 'passed', detail: '本地 NVMe 与交付热目录可写。' },
        { id: 'audio-check-3', label: '目标容量', status: 'passed', detail: '容量满足当前 18.4 GB 导入。' },
        { id: 'audio-check-4', label: '路径冲突', status: 'blocking', detail: '交付热目录已存在 3 个同名访谈音频文件。' },
        { id: 'audio-check-5', label: '校验风险', status: 'risk', detail: '录音底噪文件最近一次归档后校验失败，建议强校验。' },
        { id: 'audio-check-6', label: '执行器可用', status: 'blocking', detail: '目标目录目前被另一个整理任务占用。' },
      ],
    },
  },
  {
    id: 'import-draft-microsd-b',
    deviceSessionId: 'import-device-microsd-b',
    libraryId: 'photo',
    selectedFileIds: ['import-node-microsd-1', 'import-node-microsd-2', 'import-node-microsd-3', 'import-node-microsd-4'],
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    targetStrategy: '素材先入本地，NAS 作为正式备份端。',
    lastEditedAt: '今天 07:50',
    hasBlockingIssues: false,
    status: '已提交',
    precheckSummary: {
      blockingCount: 0,
      riskCount: 2,
      passedCount: 4,
      updatedAt: '今天 07:44',
      checks: {
        sourceReadable: 'passed',
        targetWritable: 'passed',
        capacityReady: 'passed',
        pathConflict: 'risk',
        deviceOnline: 'passed',
        executorReady: 'risk',
      },
      items: [
        { id: 'microsd-check-1', label: '来源可读', status: 'passed', detail: '设备读取正常。' },
        { id: 'microsd-check-2', label: '目标可写', status: 'passed', detail: '本地 NVMe 与 NAS 均可写。' },
        { id: 'microsd-check-3', label: '目标容量', status: 'passed', detail: '容量满足本次导入。' },
        { id: 'microsd-check-4', label: '路径冲突', status: 'risk', detail: '存在旧版代理文件，已按跳过策略处理。' },
        { id: 'microsd-check-5', label: '设备在线', status: 'passed', detail: '设备保持在线。' },
        { id: 'microsd-check-6', label: '执行器可用', status: 'risk', detail: 'NAS 写入抖动，导致部分文件重试。' },
      ],
    },
  },
  {
    id: 'import-draft-reader',
    deviceSessionId: 'import-device-reader',
    libraryId: 'photo',
    selectedFileIds: [],
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    targetStrategy: '等待扫描完成后再选择导入目标。',
    lastEditedAt: '刚刚',
    hasBlockingIssues: false,
    status: '草稿中',
    precheckSummary: {
      blockingCount: 0,
      riskCount: 0,
      passedCount: 1,
      updatedAt: '刚刚',
      checks: {
        sourceReadable: 'passed',
        targetWritable: 'passed',
        capacityReady: 'passed',
        pathConflict: 'passed',
        deviceOnline: 'passed',
        executorReady: 'passed',
      },
      items: [
        { id: 'reader-check-1', label: '等待扫描完成', status: 'passed', detail: '设备已识别，正在生成来源摘要。' },
      ],
    },
  },
  {
    id: 'import-draft-cfast-removed',
    deviceSessionId: 'import-device-cfast-removed',
    libraryId: 'photo',
    selectedFileIds: ['import-node-cfast-1', 'import-node-cfast-2'],
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    targetStrategy: '最近一次导入已完成，保留只读草稿快照。',
    lastEditedAt: '今天 07:06',
    hasBlockingIssues: false,
    status: '已提交',
    precheckSummary: {
      blockingCount: 0,
      riskCount: 0,
      passedCount: 6,
      updatedAt: '今天 07:02',
      checks: {
        sourceReadable: 'passed',
        targetWritable: 'passed',
        capacityReady: 'passed',
        pathConflict: 'passed',
        deviceOnline: 'passed',
        executorReady: 'passed',
      },
      items: [
        { id: 'cfast-check-1', label: '预检快照', status: 'passed', detail: '最后一次导入提交前所有检查均通过。' },
      ],
    },
  },
];

export const importSourceNodes: ImportSourceNodeRecord[] = [
  {
    id: 'import-node-cf-a-1',
    deviceSessionId: 'import-device-cfexpress-a',
    name: 'A001_C001_0329.mov',
    relativePath: '/DCIM/CARD_A/CLIPS/A001_C001_0329.mov',
    fileKind: '视频',
    size: '16.2 GB',
    status: '待导入',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
  },
  {
    id: 'import-node-cf-a-2',
    deviceSessionId: 'import-device-cfexpress-a',
    name: 'A001_C001_0329_proxy.mp4',
    relativePath: '/DCIM/CARD_A/PROXY/A001_C001_0329_proxy.mp4',
    fileKind: '视频',
    size: '2.1 GB',
    status: '待导入',
    targetEndpointIds: ['import-target-local'],
    issueIds: [],
    note: '代理文件只需要先落到本地 NVMe。',
  },
  {
    id: 'import-node-cf-a-3',
    deviceSessionId: 'import-device-cfexpress-a',
    name: 'RAW_0001.ARW',
    relativePath: '/PHOTO/DAY1/RAW_0001.ARW',
    fileKind: '图片',
    size: '48.2 MB',
    status: '冲突',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
    note: '目标目录存在同名代理产物，需要确认是否覆盖。',
  },
  {
    id: 'import-node-cf-a-4',
    deviceSessionId: 'import-device-cfexpress-a',
    name: 'Thumbs.db',
    relativePath: '/PHOTO/DAY1/Thumbs.db',
    fileKind: '文档',
    size: '64 KB',
    status: '已跳过',
    targetEndpointIds: [],
    issueIds: [],
    note: '系统临时文件默认跳过。',
  },
  {
    id: 'import-node-t7-1',
    deviceSessionId: 'import-device-t7',
    name: '上海发布会_A-cam_001.RAW',
    relativePath: '/ShanghaiLaunch/RAW/A-cam/001.RAW',
    fileKind: '图片',
    size: '48.2 MB',
    status: '已完成',
    targetEndpointIds: ['import-target-local', 'import-target-nas', 'import-target-cloud'],
    issueIds: [],
  },
  {
    id: 'import-node-t7-2',
    deviceSessionId: 'import-device-t7',
    name: '上海发布会_A-cam_002.RAW',
    relativePath: '/ShanghaiLaunch/RAW/A-cam/002.RAW',
    fileKind: '图片',
    size: '48.6 MB',
    status: '传输中',
    targetEndpointIds: ['import-target-local', 'import-target-nas', 'import-target-cloud'],
    issueIds: [],
  },
  {
    id: 'import-node-t7-3',
    deviceSessionId: 'import-device-t7',
    name: '上海发布会_BTS.mp4',
    relativePath: '/ShanghaiLaunch/BTS/上海发布会_BTS.mp4',
    fileKind: '视频',
    size: '14.8 GB',
    status: '校验中',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
  },
  {
    id: 'import-node-t7-4',
    deviceSessionId: 'import-device-t7',
    name: '上海发布会_精选封面.jpg',
    relativePath: '/ShanghaiLaunch/DELIVERY/上海发布会_精选封面.jpg',
    fileKind: '图片',
    size: '12.4 MB',
    status: '已排队',
    targetEndpointIds: ['import-target-local', 'import-target-cloud'],
    issueIds: ['issue-12'],
    note: '云归档需要等待鉴权修复后继续。',
  },
  {
    id: 'import-node-audio-1',
    deviceSessionId: 'import-device-audio',
    name: '访谈_环境底噪.wav',
    relativePath: '/Audio/roomtone.wav',
    fileKind: '音频',
    size: '218 MB',
    status: '冲突',
    targetEndpointIds: ['import-target-local', 'import-target-delivery'],
    issueIds: ['issue-6'],
    note: '交付热目录中已存在同名版本。',
  },
  {
    id: 'import-node-audio-2',
    deviceSessionId: 'import-device-audio',
    name: '访谈_A 机位_主录.wav',
    relativePath: '/Audio/interview-main.wav',
    fileKind: '音频',
    size: '3.8 GB',
    status: '待导入',
    targetEndpointIds: ['import-target-local', 'import-target-delivery'],
    issueIds: [],
  },
  {
    id: 'import-node-audio-3',
    deviceSessionId: 'import-device-audio',
    name: '访谈_A 机位_备份.wav',
    relativePath: '/Audio/interview-backup.wav',
    fileKind: '音频',
    size: '3.8 GB',
    status: '失败',
    targetEndpointIds: ['import-target-local'],
    issueIds: ['issue-6'],
    note: '最近一次校验失败，建议重新强校验。',
  },
  {
    id: 'import-node-microsd-1',
    deviceSessionId: 'import-device-microsd-b',
    name: 'B002_C003_1201.mov',
    relativePath: '/DCIM/B_CAM/B002_C003_1201.mov',
    fileKind: '视频',
    size: '18.4 GB',
    status: '已完成',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
  },
  {
    id: 'import-node-microsd-2',
    deviceSessionId: 'import-device-microsd-b',
    name: 'B002_C003_1202.mov',
    relativePath: '/DCIM/B_CAM/B002_C003_1202.mov',
    fileKind: '视频',
    size: '17.9 GB',
    status: '失败',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: ['issue-1'],
    note: 'NAS 路径冲突导致写入失败。',
  },
  {
    id: 'import-node-microsd-3',
    deviceSessionId: 'import-device-microsd-b',
    name: 'B002_C003_1202_proxy.mp4',
    relativePath: '/DCIM/B_CAM/PROXY/B002_C003_1202_proxy.mp4',
    fileKind: '视频',
    size: '2.6 GB',
    status: '已跳过',
    targetEndpointIds: [],
    issueIds: [],
    note: '代理文件按策略跳过。',
  },
  {
    id: 'import-node-microsd-4',
    deviceSessionId: 'import-device-microsd-b',
    name: 'B002_C003_1203.mov',
    relativePath: '/DCIM/B_CAM/B002_C003_1203.mov',
    fileKind: '视频',
    size: '18.1 GB',
    status: '已完成',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
  },
  {
    id: 'import-node-cfast-1',
    deviceSessionId: 'import-device-cfast-removed',
    name: '备机_主素材_001.RAW',
    relativePath: '/DCIM/BACKUP/001.RAW',
    fileKind: '图片',
    size: '47.1 MB',
    status: '已完成',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
  },
  {
    id: 'import-node-cfast-2',
    deviceSessionId: 'import-device-cfast-removed',
    name: '备机_主素材_002.RAW',
    relativePath: '/DCIM/BACKUP/002.RAW',
    fileKind: '图片',
    size: '47.4 MB',
    status: '已完成',
    targetEndpointIds: ['import-target-local', 'import-target-nas'],
    issueIds: [],
  },
];

export const importReports: ImportReportSnapshot[] = [
  {
    id: 'import-report-t7-running',
    deviceSessionId: 'import-device-t7',
    taskId: 'task-import-running-t7',
    title: '现场移动硬盘 T7 / 正在导入',
    status: '运行中',
    submittedAt: '今天 09:23',
    successCount: 612,
    failedCount: 1,
    partialCount: 17,
    verifySummary: '默认校验进行中，已有 612 个文件完成写账。',
    targetSummaries: [
      {
        endpointId: 'import-target-local',
        label: '本地 NVMe 主盘',
        status: '已完成 58%',
        successCount: 612,
        failedCount: 0,
        transferredSize: '812 GB',
      },
      {
        endpointId: 'import-target-nas',
        label: '影像 NAS 01',
        status: '已完成 42%',
        successCount: 448,
        failedCount: 1,
        transferredSize: '584 GB',
      },
      {
        endpointId: 'import-target-cloud',
        label: '115 云归档',
        status: '等待恢复',
        successCount: 116,
        failedCount: 0,
        transferredSize: '94 GB',
      },
    ],
    issueIds: ['issue-12'],
    latestUpdatedAt: '今天 11:06',
    fileCount: 1824,
    note: '云归档端因鉴权异常暂时停在等待恢复状态。',
  },
  {
    id: 'import-report-microsd-partial',
    deviceSessionId: 'import-device-microsd-b',
    taskId: 'task-import-partial-microsd',
    title: 'microSD 卡（B 机位） / 第 1 次导入',
    status: '部分成功',
    submittedAt: '今天 07:44',
    finishedAt: '今天 08:16',
    successCount: 286,
    failedCount: 18,
    partialCount: 8,
    verifySummary: '已有 18 个文件待补处理，主要集中在 NAS 路径冲突。',
    targetSummaries: [
      {
        endpointId: 'import-target-local',
        label: '本地 NVMe 主盘',
        status: '已完成',
        successCount: 304,
        failedCount: 0,
        transferredSize: '1.4 TB',
      },
      {
        endpointId: 'import-target-nas',
        label: '影像 NAS 01',
        status: '部分成功',
        successCount: 286,
        failedCount: 18,
        transferredSize: '1.2 TB',
      },
    ],
    issueIds: ['issue-1'],
    latestUpdatedAt: '今天 08:16',
    fileCount: 312,
  },
  {
    id: 'import-report-cfast-complete',
    deviceSessionId: 'import-device-cfast-removed',
    taskId: 'task-import-complete-cfast',
    title: 'CFast 卡（备机） / 最近一次导入',
    status: '已完成',
    submittedAt: '今天 07:02',
    finishedAt: '今天 07:18',
    successCount: 206,
    failedCount: 0,
    partialCount: 0,
    verifySummary: '默认校验全部通过，可回到文件中心继续整理。',
    targetSummaries: [
      {
        endpointId: 'import-target-local',
        label: '本地 NVMe 主盘',
        status: '已完成',
        successCount: 206,
        failedCount: 0,
        transferredSize: '418 GB',
      },
      {
        endpointId: 'import-target-nas',
        label: '影像 NAS 01',
        status: '已完成',
        successCount: 206,
        failedCount: 0,
        transferredSize: '418 GB',
      },
    ],
    issueIds: [],
    latestUpdatedAt: '今天 07:18',
    fileCount: 206,
  },
];

export const taskRecords: TaskRecord[] = [
  {
    id: 'task-transfer-1',
    kind: 'transfer',
    title: '拍摄原片同步',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'COPY',
    status: '运行中',
    statusTone: 'warning',
    libraryId: 'photo',
    source: '本地NVMe',
    target: '影像NAS',
    fileNodeIds: ['photo-file-raw-001', 'photo-file-raw-002'],
    sourcePath: '/2026/ShanghaiLaunch/raw',
    targetPath: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\raw',
    progress: 72,
    speed: '182 MB/s',
    eta: '12 分钟',
    fileCount: 124,
    folderCount: 0,
    totalSize: '96 MB',
    totalSizeBytes: 100663296,
    multiFile: true,
    updatedAt: '刚刚',
    priority: '普通优先级',
    issueIds: [],
    creator: '文件中心',
    createdAt: '今天 09:18',
  },
  {
    id: 'task-transfer-2',
    kind: 'transfer',
    title: '精选交付归档',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'UPLOAD',
    status: '运行中',
    statusTone: 'warning',
    libraryId: 'photo',
    source: '本地NVMe',
    target: '115 云归档',
    fileNodeIds: ['photo-root-delivery'],
    sourcePath: '/2026/ShanghaiLaunch/delivery',
    targetPath: '/MareArchive/2026/ShanghaiLaunch/delivery',
    progress: 48,
    speed: '68 MB/s',
    eta: '18 分钟',
    fileCount: 136,
    folderCount: 1,
    totalSize: '136 项',
    totalSizeBytes: 0,
    multiFile: false,
    updatedAt: '1 分钟前',
    priority: '高优先级',
    issueIds: ['issue-3', 'issue-5'],
    creator: '文件中心',
    createdAt: '今天 09:42',
  },
  {
    id: 'task-transfer-3',
    kind: 'transfer',
    title: '成片交付备份',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'COPY',
    status: '已暂停',
    statusTone: 'info',
    libraryId: 'video',
    source: '本地NVMe',
    target: '影像NAS',
    fileNodeIds: ['video-root-final'],
    sourcePath: '/2026/Interview/final',
    targetPath: '\\\\192.168.10.20\\media\\video\\Interview\\final',
    progress: 41,
    speed: '—',
    eta: '等待继续',
    fileCount: 46,
    folderCount: 1,
    totalSize: '46 项',
    totalSizeBytes: 0,
    multiFile: true,
    updatedAt: '11 分钟前',
    priority: '普通优先级',
    issueIds: [],
    creator: '文件中心',
    createdAt: '今天 09:50',
  },
  {
    id: 'task-transfer-4',
    kind: 'transfer',
    title: '客户成片下载',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'DOWNLOAD',
    status: '已完成',
    statusTone: 'success',
    libraryId: 'video',
    source: '115 云归档',
    target: '本地NVMe',
    fileNodeIds: ['video-file-final'],
    sourcePath: '/MareArchive/Video/Interview/final',
    targetPath: 'D:\\Mare\\Assets\\Video\\Interview\\final',
    progress: 100,
    speed: '—',
    eta: '已完成',
    fileCount: 1,
    folderCount: 0,
    totalSize: '12.8 GB',
    totalSizeBytes: 13743895347,
    multiFile: false,
    updatedAt: '8 分钟前',
    priority: '低优先级',
    issueIds: ['issue-1'],
    creator: '文件中心',
    createdAt: '今天 08:56',
  },
  {
    id: 'task-transfer-5',
    kind: 'transfer',
    title: '音频归档上传',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'UPLOAD',
    status: '失败',
    statusTone: 'critical',
    libraryId: 'video',
    source: '本地NVMe',
    target: '115 云归档',
    fileNodeIds: ['video-file-audio'],
    sourcePath: '/2026/Interview/audio',
    targetPath: '/MareArchive/Interview/audio',
    progress: 84,
    speed: '—',
    eta: '等待重试',
    fileCount: 1,
    folderCount: 0,
    totalSize: '148 MB',
    totalSizeBytes: 155189248,
    multiFile: false,
    updatedAt: '26 分钟前',
    priority: '高优先级',
    issueIds: ['issue-6'],
    creator: '文件中心',
    createdAt: '今天 10:35',
  },
  {
    id: 'task-transfer-6',
    kind: 'transfer',
    title: '客户返签确认单_2026Q2.pdf',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'UPLOAD',
    status: '等待确认',
    statusTone: 'info',
    libraryId: 'video',
    source: '本地NVMe',
    target: '115 云归档',
    sourcePath: 'C:\\Mare\\Exports\\客户返签确认单_2026Q2.pdf',
    targetPath: '/MareArchive/Interview/Exports/客户返签确认单_2026Q2.pdf',
    progress: 0,
    speed: '—',
    eta: '等待执行器接管',
    fileCount: 1,
    folderCount: 0,
    totalSize: '6.2 MB',
    totalSizeBytes: 6501171,
    multiFile: false,
    updatedAt: '刚刚',
    priority: '普通优先级',
    issueIds: ['issue-6'],
    creator: '文件中心',
    createdAt: '今天 11:52',
  },
  {
    id: 'task-transfer-7',
    kind: 'transfer',
    title: '跨目录素材归档',
    type: 'SYNC',
    businessType: 'SYNC',
    syncLinkType: 'UPLOAD',
    status: '等待确认',
    statusTone: 'info',
    libraryId: 'photo',
    source: '本地NVMe',
    target: '115 云归档',
    fileNodeIds: ['photo-file-raw-001', 'photo-file-cover'],
    sourcePath: '/2026/ShanghaiLaunch',
    targetPath: '/MareArchive/2026/ShanghaiLaunch/mixed',
    progress: 0,
    speed: '—',
    eta: '等待执行器接管',
    fileCount: 2,
    folderCount: 0,
    totalSize: '60.6 MB',
    totalSizeBytes: 63543705,
    multiFile: true,
    updatedAt: '刚刚',
    priority: '普通优先级',
    issueIds: [],
    creator: '文件中心',
    createdAt: '今天 12:06',
  },
  {
    id: 'task-other-scan-1',
    kind: 'other',
    title: '影像 NAS 挂载目录全量扫描',
    type: 'SCAN',
    otherTaskType: 'SCAN',
    status: '运行中',
    statusTone: 'warning',
    libraryId: 'photo',
    source: '存储节点',
    target: '影像 NAS 01',
    fileNodeIds: ['photo-root-raw', 'photo-root-delivery'],
    sourcePath: '\\\\192.168.10.20\\media\\photo',
    progress: 63,
    speed: '128 项/秒',
    eta: '6 分钟',
    fileCount: 3218,
    folderCount: 184,
    multiFile: true,
    priority: '普通优先级',
    issueIds: ['issue-7'],
    creator: '存储节点',
    createdAt: '今天 12:18',
    startedAt: '今天 12:19',
    updatedAt: '刚刚',
    phaseLabel: '归并提交',
    scopeLabel: '影像 NAS 01 / 2026 / Shanghai Launch',
    endpointLabel: '影像 NAS 01',
    resultSummary: '已枚举 3,218 项，正在写入正式状态',
  },
  {
    id: 'task-other-scan-2',
    kind: 'other',
    title: '家庭照片增量扫描',
    type: 'SCAN',
    otherTaskType: 'SCAN',
    status: '已暂停',
    statusTone: 'info',
    libraryId: 'family',
    source: '存储节点',
    target: '家庭照片 NAS',
    sourcePath: '\\\\192.168.10.30\\family',
    progress: 35,
    speed: '—',
    eta: '等待继续',
    fileCount: 842,
    folderCount: 37,
    multiFile: true,
    priority: '高优先级',
    creator: '存储节点',
    createdAt: '今天 10:06',
    startedAt: '今天 10:07',
    updatedAt: '9 分钟前',
    phaseLabel: '枚举目录',
    scopeLabel: '家庭照片 NAS / Archive / Family',
    endpointLabel: '家庭照片 NAS',
    resultSummary: '已扫描 295 项，等待继续',
  },
  {
    id: 'task-other-meta-1',
    kind: 'other',
    title: '上海发布会素材元数据解析',
    type: 'METADATA_EXTRACT',
    otherTaskType: 'METADATA_EXTRACT',
    status: '运行中',
    statusTone: 'warning',
    libraryId: 'photo',
    source: '系统解析队列',
    fileNodeIds: ['photo-file-raw-001', 'photo-file-cover', 'photo-file-gallery-zip'],
    sourcePath: '/2026/ShanghaiLaunch',
    progress: 47,
    speed: '42 文件/分',
    eta: '14 分钟',
    fileCount: 186,
    folderCount: 0,
    multiFile: true,
    priority: '普通优先级',
    issueIds: ['issue-8'],
    creator: '系统',
    createdAt: '今天 11:42',
    startedAt: '今天 11:43',
    updatedAt: '2 分钟前',
    phaseLabel: '写入媒体表',
    scopeLabel: '商业摄影资产库 / 2026 / Shanghai Launch',
    resultSummary: '照片、视频、音频字段正在结构化写入',
  },
  {
    id: 'task-other-meta-2',
    kind: 'other',
    title: '家庭视频历史元数据补齐',
    type: 'METADATA_EXTRACT',
    otherTaskType: 'METADATA_EXTRACT',
    status: '部分成功',
    statusTone: 'warning',
    libraryId: 'family',
    source: '系统解析队列',
    progress: 100,
    speed: '—',
    eta: '已结束',
    fileCount: 92,
    folderCount: 0,
    multiFile: true,
    priority: '低优先级',
    issueIds: ['issue-9'],
    creator: '系统',
    createdAt: '今天 08:10',
    startedAt: '今天 08:11',
    finishedAt: '今天 08:44',
    updatedAt: '今天 08:44',
    phaseLabel: '更新解析状态',
    scopeLabel: '家庭照片资产库 / 历史视频目录',
    resultSummary: '84 项完成，8 项因原文件损坏未解析',
  },
  {
    id: 'task-other-verify-1',
    kind: 'other',
    title: '客户访谈成片夜间强校验',
    type: 'VERIFY',
    otherTaskType: 'VERIFY',
    status: '等待确认',
    statusTone: 'info',
    libraryId: 'video',
    source: '校验恢复',
    target: '影像 NAS 01',
    fileNodeIds: ['video-root-final', 'video-file-final'],
    sourcePath: '/2026/Interview/final',
    targetPath: '\\\\192.168.10.20\\media\\video\\Interview\\final',
    progress: 46,
    speed: '—',
    eta: '今晚 22:00 自动继续',
    fileCount: 12,
    folderCount: 1,
    multiFile: true,
    priority: '普通优先级',
    creator: '校验恢复',
    createdAt: '今天 15:30',
    startedAt: '今天 15:35',
    updatedAt: '5 分钟前',
    phaseLabel: '等待时间窗',
    scopeLabel: '客户访谈成片目录 / 强校验',
    endpointLabel: '影像 NAS 01',
    resultSummary: '已完成 46%，将在夜间时间窗继续',
    waitingReason: '等待强校验时间窗',
  },
  {
    id: 'task-other-verify-2',
    kind: 'other',
    title: '上海发布会精选包校验',
    type: 'VERIFY',
    otherTaskType: 'VERIFY',
    status: '失败',
    statusTone: 'critical',
    libraryId: 'photo',
    source: '校验恢复',
    target: '115 云归档',
    fileNodeIds: ['photo-file-gallery-zip'],
    sourcePath: '/2026/ShanghaiLaunch/delivery/package',
    targetPath: '/MareArchive/2026/ShanghaiLaunch/delivery/package',
    progress: 100,
    speed: '—',
    eta: '已结束',
    fileCount: 24,
    folderCount: 0,
    multiFile: true,
    priority: '高优先级',
    issueIds: ['issue-10'],
    creator: '校验恢复',
    createdAt: '今天 09:40',
    startedAt: '今天 09:41',
    finishedAt: '今天 10:02',
    updatedAt: '今天 10:02',
    phaseLabel: '写入校验结果',
    scopeLabel: '上海发布会精选包 / 115 云归档',
    endpointLabel: '115 云归档',
    resultSummary: '发现 3 项校验失败',
  },
  {
    id: 'task-other-delete-1',
    kind: 'other',
    title: '删除资产：2026-03-29_上海发布会_A-cam_001.RAW',
    type: 'DELETE',
    otherTaskType: 'DELETE_CLEANUP',
    backendTypes: ['DELETE', 'ASSET_CLEANUP'],
    status: '等待清理',
    statusTone: 'info',
    libraryId: 'photo',
    source: '文件中心',
    fileNodeIds: ['photo-file-raw-001'],
    sourcePath: '/2026/ShanghaiLaunch/raw/A-cam_001.RAW',
    progress: 72,
    speed: '—',
    eta: '等待系统空闲',
    fileCount: 1,
    folderCount: 0,
    multiFile: false,
    priority: '普通优先级',
    creator: '文件中心',
    createdAt: '今天 16:12',
    startedAt: '今天 16:12',
    updatedAt: '刚刚',
    phaseLabel: '等待清理',
    scopeLabel: '删除资产并等待后台清理',
    resultSummary: '副本已移除，等待清理扩展数据',
    assetIds: ['photo-file-raw-001'],
  },
  {
    id: 'task-other-delete-2',
    kind: 'other',
    title: '历史交付包后台清理',
    type: 'ASSET_CLEANUP',
    otherTaskType: 'DELETE_CLEANUP',
    backendTypes: ['ASSET_CLEANUP'],
    status: '运行中',
    statusTone: 'warning',
    libraryId: 'photo',
    source: '系统清理队列',
    progress: 28,
    speed: '36 项/秒',
    eta: '9 分钟',
    fileCount: 34,
    folderCount: 0,
    multiFile: true,
    priority: '低优先级',
    issueIds: ['issue-11'],
    creator: '系统',
    createdAt: '今天 16:05',
    startedAt: '今天 16:06',
    updatedAt: '刚刚',
    phaseLabel: '清理关联数据',
    scopeLabel: '历史交付包 / PENDING_DELETE 清理队列',
    resultSummary: '已清理 9/34 项扩展数据',
  },
  {
    id: 'task-other-delete-3',
    kind: 'other',
    title: '旧缓存副本清理',
    type: 'ASSET_CLEANUP',
    otherTaskType: 'DELETE_CLEANUP',
    backendTypes: ['ASSET_CLEANUP'],
    status: '已完成',
    statusTone: 'success',
    libraryId: 'video',
    source: '系统清理队列',
    progress: 100,
    speed: '—',
    eta: '已结束',
    fileCount: 18,
    folderCount: 0,
    multiFile: true,
    priority: '低优先级',
    creator: '系统',
    createdAt: '今天 07:30',
    startedAt: '今天 07:31',
    finishedAt: '今天 07:36',
    updatedAt: '今天 07:36',
    phaseLabel: '完成',
    scopeLabel: '旧缓存副本 / 清理完成',
    resultSummary: '18 项关联记录已清理完成',
  },
];

export const taskItemRecords: TaskItemRecord[] = [
  {
    id: 'task-item-1',
    taskId: 'task-transfer-1',
    fileNodeId: 'photo-file-raw-001',
    kind: 'file',
    depth: 1,
    phase: '已完成',
    status: '已完成',
    statusTone: 'success',
    priority: '普通优先级',
    progress: 100,
    speed: '—',
    targetPath: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\raw\\A-cam_001.RAW',
  },
  {
    id: 'task-item-2',
    taskId: 'task-transfer-1',
    fileNodeId: 'photo-file-raw-002',
    kind: 'file',
    depth: 1,
    phase: '传输中',
    status: '传输中',
    statusTone: 'warning',
    priority: '普通优先级',
    progress: 74,
    speed: '176 MB/s',
    targetPath: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\raw\\B-cam_018.RAW',
  },
  {
    id: 'task-item-3',
    taskId: 'task-transfer-2',
    fileNodeId: 'photo-file-cover',
    kind: 'file',
    depth: 1,
    phase: '传输中',
    status: '传输中',
    statusTone: 'warning',
    priority: '高优先级',
    progress: 48,
    speed: '68 MB/s',
    issueIds: ['issue-3', 'issue-5'],
    targetPath: '/MareArchive/2026/ShanghaiLaunch/delivery/上海发布会_精选封面.jpg',
  },
  {
    id: 'task-item-4',
    taskId: 'task-transfer-2',
    fileNodeId: 'photo-file-gallery-zip',
    kind: 'file',
    depth: 1,
    phase: '校验中',
    status: '校验中',
    statusTone: 'warning',
    priority: '高优先级',
    progress: 52,
    speed: '68 MB/s',
    targetPath: '/MareArchive/2026/ShanghaiLaunch/delivery/上海发布会_精选组图.zip',
  },
  {
    id: 'task-item-5',
    taskId: 'task-transfer-4',
    fileNodeId: 'video-file-final',
    kind: 'file',
    depth: 1,
    phase: '已完成',
    status: '已完成',
    statusTone: 'success',
    priority: '低优先级',
    progress: 100,
    speed: '—',
    targetPath: 'D:\\Mare\\Assets\\Video\\Interview\\final\\客户访谈_第一机位_精编版.mov',
  },
  {
    id: 'task-item-6',
    taskId: 'task-transfer-5',
    fileNodeId: 'video-file-audio',
    kind: 'file',
    depth: 1,
    phase: '失败',
    status: '失败',
    statusTone: 'critical',
    priority: '高优先级',
    progress: 84,
    speed: '—',
    issueIds: ['issue-6'],
    targetPath: '/MareArchive/Interview/audio/片头配乐_v4_master.wav',
  },
  {
    id: 'task-item-7',
    taskId: 'task-transfer-6',
    name: '客户返签确认单_2026Q2.pdf',
    kind: 'file',
    depth: 1,
    phase: '待执行',
    status: '待执行',
    statusTone: 'info',
    priority: '普通优先级',
    progress: 0,
    size: '6.2 MB',
    speed: '—',
    sourcePath: 'C:\\Mare\\Exports\\客户返签确认单_2026Q2.pdf',
    targetPath: '/MareArchive/Interview/Exports/客户返签确认单_2026Q2.pdf',
  },
  {
    id: 'task-item-12',
    taskId: 'task-transfer-3',
    fileNodeId: 'video-file-final',
    kind: 'file',
    depth: 1,
    phase: '已暂停',
    status: '已暂停',
    statusTone: 'info',
    priority: '普通优先级',
    progress: 41,
    speed: '—',
    targetPath: '\\\\192.168.10.20\\media\\video\\Interview\\final\\客户访谈_第一机位_精编版.mov',
  },
  {
    id: 'task-item-8',
    taskId: 'task-transfer-7',
    fileNodeId: 'photo-file-raw-001',
    kind: 'file',
    depth: 1,
    phase: '待执行',
    status: '待执行',
    statusTone: 'info',
    priority: '普通优先级',
    progress: 0,
    speed: '—',
    targetPath: '/MareArchive/2026/ShanghaiLaunch/mixed/2026-03-29_上海发布会_A-cam_001.RAW',
  },
  {
    id: 'task-item-9',
    taskId: 'task-transfer-7',
    fileNodeId: 'photo-file-gallery-zip',
    kind: 'file',
    depth: 1,
    phase: '待执行',
    status: '待执行',
    statusTone: 'info',
    priority: '普通优先级',
    progress: 0,
    speed: '—',
    targetPath: '/MareArchive/2026/ShanghaiLaunch/mixed/上海发布会_精选组图.zip',
  },
  {
    id: 'task-item-other-scan-1',
    taskId: 'task-other-scan-1',
    fileNodeId: 'photo-root-raw',
    kind: 'folder',
    depth: 1,
    phase: '已完成',
    status: '已完成',
    statusTone: 'success',
    priority: '普通优先级',
    progress: 100,
    size: '2,184 项',
    speed: '—',
    pathLabel: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\raw',
    resultLabel: '目录枚举完成',
  },
  {
    id: 'task-item-other-scan-2',
    taskId: 'task-other-scan-1',
    fileNodeId: 'photo-root-delivery',
    kind: 'folder',
    depth: 1,
    phase: '归并提交',
    status: '运行中',
    statusTone: 'warning',
    priority: '普通优先级',
    progress: 63,
    size: '136 项',
    speed: '128 项/秒',
    pathLabel: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\delivery',
    issueIds: ['issue-7'],
    resultLabel: '准备写入正式状态',
  },
  {
    id: 'task-item-other-scan-3',
    taskId: 'task-other-scan-2',
    name: 'Archive / Family / 2024',
    kind: 'folder',
    depth: 1,
    phase: '枚举目录',
    status: '已暂停',
    statusTone: 'info',
    priority: '高优先级',
    progress: 35,
    size: '842 项',
    speed: '—',
    pathLabel: '\\\\192.168.10.30\\family\\2024',
    resultLabel: '等待继续',
  },
  {
    id: 'task-item-other-meta-1',
    taskId: 'task-other-meta-1',
    fileNodeId: 'photo-file-raw-001',
    kind: 'file',
    depth: 1,
    phase: '读取原始元数据',
    status: '运行中',
    statusTone: 'warning',
    priority: '普通优先级',
    progress: 58,
    speed: '42 文件/分',
    pathLabel: '/2026/ShanghaiLaunch/raw/2026-03-29_上海发布会_A-cam_001.RAW',
    resultLabel: 'EXIF 已读取',
  },
  {
    id: 'task-item-other-meta-2',
    taskId: 'task-other-meta-1',
    fileNodeId: 'photo-file-cover',
    kind: 'file',
    depth: 1,
    phase: '写入媒体表',
    status: '运行中',
    statusTone: 'warning',
    priority: '普通优先级',
    progress: 47,
    speed: '42 文件/分',
    pathLabel: '/2026/ShanghaiLaunch/delivery/上海发布会_精选封面.jpg',
    issueIds: ['issue-8'],
    resultLabel: '缩略图生成完成，结构化字段写入中',
  },
  {
    id: 'task-item-other-meta-3',
    taskId: 'task-other-meta-2',
    name: '家庭视频 / 2019 / 毕业旅行.mp4',
    kind: 'file',
    depth: 1,
    phase: '更新解析状态',
    status: '已完成',
    statusTone: 'success',
    priority: '低优先级',
    progress: 100,
    size: '2.4 GB',
    speed: '—',
    resultLabel: '解析完成',
  },
  {
    id: 'task-item-other-meta-4',
    taskId: 'task-other-meta-2',
    name: '家庭视频 / 2019 / 机内代理.mov',
    kind: 'file',
    depth: 1,
    phase: '写入媒体表',
    status: '失败',
    statusTone: 'critical',
    priority: '低优先级',
    progress: 100,
    size: '1.8 GB',
    speed: '—',
    issueIds: ['issue-9'],
    resultLabel: '原文件损坏，无法完成解析',
  },
  {
    id: 'task-item-other-verify-1',
    taskId: 'task-other-verify-1',
    fileNodeId: 'video-file-final',
    kind: 'file',
    depth: 1,
    phase: '等待时间窗',
    status: '等待确认',
    statusTone: 'info',
    priority: '普通优先级',
    progress: 46,
    speed: '—',
    pathLabel: '/2026/Interview/final/客户访谈_第一机位_精编版.mov',
    resultLabel: '已完成快速校验，等待强校验时间窗',
  },
  {
    id: 'task-item-other-verify-2',
    taskId: 'task-other-verify-2',
    fileNodeId: 'photo-file-gallery-zip',
    kind: 'file',
    depth: 1,
    phase: '写入校验结果',
    status: '失败',
    statusTone: 'critical',
    priority: '高优先级',
    progress: 100,
    speed: '—',
    pathLabel: '/2026/ShanghaiLaunch/delivery/package/上海发布会_精选组图.zip',
    issueIds: ['issue-10'],
    resultLabel: '目标端哈希不一致',
  },
  {
    id: 'task-item-other-delete-1',
    taskId: 'task-other-delete-1',
    fileNodeId: 'photo-file-raw-001',
    kind: 'asset',
    depth: 1,
    phase: '等待清理',
    status: '等待清理',
    statusTone: 'info',
    priority: '普通优先级',
    progress: 72,
    speed: '—',
    pathLabel: '/2026/ShanghaiLaunch/raw/2026-03-29_上海发布会_A-cam_001.RAW',
    resultLabel: '副本已删除，等待清理扩展数据',
  },
  {
    id: 'task-item-other-delete-2',
    taskId: 'task-other-delete-2',
    name: '清理标签关联',
    kind: 'step',
    depth: 1,
    phase: '清理关联数据',
    status: '运行中',
    statusTone: 'warning',
    priority: '低优先级',
    progress: 28,
    speed: '36 项/秒',
    resultLabel: '已清理 9/34 项',
    issueIds: ['issue-11'],
  },
  {
    id: 'task-item-other-delete-3',
    taskId: 'task-other-delete-3',
    name: '清理 media_asset 关联记录',
    kind: 'step',
    depth: 1,
    phase: '完成',
    status: '已完成',
    statusTone: 'success',
    priority: '低优先级',
    progress: 100,
    speed: '—',
    resultLabel: '已完成',
  },
];

export const issueRecords: IssueRecord[] = [
  {
    id: 'issue-1',
    libraryId: 'video',
    taskId: 'task-transfer-4',
    category: '冲突',
    type: '路径冲突',
    nature: 'BLOCKING',
    sourceDomain: '传输任务',
    severity: 'critical',
    title: '客户访谈_第一机位_精编版.mov 路径冲突',
    summary: '下载完成前检测到目标目录存在同名已交付母带，继续覆盖会破坏正式交付版本。',
    asset: '客户访谈_第一机位_精编版.mov',
    objectLabel: '客户访谈_第一机位_精编版.mov / D:\\Mare\\Assets\\Video\\Interview\\final',
    action: '重命名旧版母带或执行删除资产',
    actionLabel: '打开文件中心',
    suggestion: '当前更适合回到文件中心确认是否保留旧版本或修改落地路径。',
    detail: '目标目录已有同名母带，继续同步会覆盖已交付版本。',
    status: '待处理',
    createdAt: '今天 10:21',
    updatedAt: '刚刚',
    source: {
      sourceDomain: '传输任务',
      taskId: 'task-transfer-4',
      taskTitle: '客户成片下载',
      fileNodeId: 'video-file-final',
      sourceLabel: '115 云归档 → 本地 NVMe',
      path: 'D:\\Mare\\Assets\\Video\\Interview\\final\\客户访谈_第一机位_精编版.mov',
      routeLabel: '115 云归档 → 本地 NVMe',
    },
    impact: {
      assetCount: 1,
      replicaCount: 1,
      folderCount: 1,
      endpointCount: 2,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canIgnore: true,
      canPostpone: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-1-history-1',
        issueId: 'issue-1',
        action: '自动发现',
        operatorLabel: '传输执行器',
        result: '下载前校验发现目标路径已有受管文件。',
        createdAt: '今天 10:21',
      },
    ],
  },
  {
    id: 'issue-2',
    libraryId: 'video',
    category: '校验',
    type: '校验失败',
    nature: 'BLOCKING',
    sourceDomain: '文件中心',
    severity: 'critical',
    title: '片头配乐_v4_master.wav 默认校验失败',
    summary: '文件中心从 NAS 拉取状态时发现远端校验值与本地副本不一致，当前副本可信度不足。',
    asset: '片头配乐_v4_master.wav',
    objectLabel: '片头配乐_v4_master.wav / 影像 NAS / /2026/Interview/audio',
    action: '从本地副本重新同步到 NAS',
    actionLabel: '重试',
    suggestion: '建议先从本地可信副本重新推送，再在夜间窗口执行强校验。',
    detail: '影像 NAS 返回的校验值与本地源文件不一致。',
    status: '待处理',
    createdAt: '今天 09:58',
    updatedAt: '今天 10:03',
    source: {
      sourceDomain: '文件中心',
      fileNodeId: 'video-file-audio',
      endpointId: 'node-2',
      endpointLabel: '影像 NAS 01',
      sourceLabel: '文件中心同步状态检测',
      path: '视频工作流资产库 / 2026 / Interview / audio / 片头配乐_v4_master.wav',
    },
    impact: {
      assetCount: 1,
      replicaCount: 1,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: false,
    },
    capabilities: {
      canRetry: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-2-history-1',
        issueId: 'issue-2',
        action: '自动发现',
        operatorLabel: '文件中心',
        result: '端点状态轮询返回校验值不一致。',
        createdAt: '今天 09:58',
      },
    ],
  },
  {
    id: 'issue-3',
    libraryId: 'photo',
    taskId: 'task-transfer-2',
    taskItemId: 'task-item-3',
    category: '节点与权限',
    type: '鉴权提醒',
    nature: 'RISK',
    sourceDomain: '传输任务',
    severity: 'warning',
    title: '115 云归档鉴权即将过期',
    summary: '当前归档链路仍可继续执行，但令牌将在 12 小时内过期，晚间批量上传存在中断风险。',
    asset: '115 云归档',
    objectLabel: '115 云归档 / 上海发布会归档目录',
    action: '刷新令牌并重新检测目标目录',
    actionLabel: '标记确认',
    suggestion: '建议先标记已确认，再在存储节点页完成令牌刷新，避免夜间任务中断。',
    detail: '令牌将在 12 小时内过期',
    status: '待确认',
    createdAt: '今天 09:43',
    updatedAt: '今天 09:55',
    source: {
      sourceDomain: '传输任务',
      taskId: 'task-transfer-2',
      taskItemId: 'task-item-3',
      taskTitle: '精选交付归档',
      taskItemTitle: '上海发布会_精选封面.jpg',
      endpointId: 'node-4',
      endpointLabel: '115 云归档',
      sourceLabel: '本地 NVMe → 115 云归档',
      path: '/MareArchive/2026/ShanghaiLaunch/delivery',
      routeLabel: '本地 NVMe → 115 云归档',
    },
    impact: {
      assetCount: 136,
      replicaCount: 136,
      folderCount: 1,
      endpointCount: 1,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canConfirm: true,
      canPostpone: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
      canOpenStorageNodes: true,
    },
    histories: [
      {
        id: 'issue-3-history-1',
        issueId: 'issue-3',
        action: '自动发现',
        operatorLabel: '云端鉴权检查',
        result: '检测到 Token 即将过期。',
        createdAt: '今天 09:43',
      },
    ],
  },
  {
    id: 'issue-4',
    libraryId: 'photo',
    taskId: 'task-transfer-2',
    taskItemId: 'task-item-4',
    category: '冲突',
    type: '历史版本残留',
    nature: 'RISK',
    sourceDomain: '传输任务',
    severity: 'info',
    title: '精选交付目录存在历史版本残留',
    summary: '归档目录中发现上一轮交付目录，当前不会阻塞上传，但需要人工确认是否清理旧版本。',
    asset: '精选交付',
    objectLabel: '精选交付 / 上海发布会归档目录',
    action: '确认是否覆盖旧版交付目录',
    actionLabel: '延后处理',
    suggestion: '建议等本轮归档完成后统一在文件中心核对历史版本，再决定保留或清理。',
    detail: '远端已存在历史交付目录，归档完成后建议人工复核。',
    status: '待处理',
    createdAt: '今天 09:48',
    updatedAt: '今天 09:50',
    source: {
      sourceDomain: '传输任务',
      taskId: 'task-transfer-2',
      taskItemId: 'task-item-4',
      taskTitle: '精选交付归档',
      taskItemTitle: '上海发布会_精选组图.zip',
      endpointId: 'node-4',
      endpointLabel: '115 云归档',
      sourceLabel: '归档前目录预检查',
      path: '/MareArchive/2026/ShanghaiLaunch/delivery',
    },
    impact: {
      assetCount: 2,
      replicaCount: 2,
      folderCount: 1,
      endpointCount: 1,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canConfirm: true,
      canPostpone: true,
      canIgnore: true,
      canOpenTaskCenter: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-4-history-1',
        issueId: 'issue-4',
        action: '自动发现',
        operatorLabel: '归档预检查',
        result: '检测到同目录下存在历史交付残留。',
        createdAt: '今天 09:48',
      },
    ],
  },
  {
    id: 'issue-5',
    libraryId: 'photo',
    taskId: 'task-transfer-2',
    taskItemId: 'task-item-3',
    category: '容量与资源',
    type: '空间预警',
    nature: 'RISK',
    sourceDomain: '传输任务',
    severity: 'warning',
    title: '115 云归档空间进入预警区',
    summary: '归档目标剩余空间低于 50 GB，本轮可继续执行，但后续批量同步存在阻塞风险。',
    asset: '115 云归档',
    objectLabel: '115 云归档 / 剩余 46 GB',
    action: '清理旧副本或切换归档目录',
    actionLabel: '刷新检测',
    suggestion: '建议先刷新容量检测，再决定清理旧副本或切换归档目录。',
    detail: '归档目录剩余空间低于 50 GB，后续批量上传可能被阻塞。',
    status: '待处理',
    createdAt: '今天 09:44',
    updatedAt: '今天 09:57',
    source: {
      sourceDomain: '传输任务',
      taskId: 'task-transfer-2',
      taskItemId: 'task-item-3',
      taskTitle: '精选交付归档',
      taskItemTitle: '上海发布会_精选封面.jpg',
      endpointId: 'node-4',
      endpointLabel: '115 云归档',
      sourceLabel: '归档容量检测',
      path: '/MareArchive/2026/ShanghaiLaunch/delivery',
    },
    impact: {
      assetCount: 136,
      replicaCount: 136,
      folderCount: 1,
      endpointCount: 1,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canConfirm: true,
      canPostpone: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
      canOpenStorageNodes: true,
    },
    histories: [
      {
        id: 'issue-5-history-1',
        issueId: 'issue-5',
        action: '自动发现',
        operatorLabel: '容量巡检',
        result: '目标端容量低于预警阈值 50 GB。',
        createdAt: '今天 09:44',
      },
    ],
  },
  {
    id: 'issue-6',
    libraryId: 'video',
    taskId: 'task-transfer-5',
    taskItemId: 'task-item-6',
    category: '校验',
    type: '校验失败',
    nature: 'BLOCKING',
    sourceDomain: '传输任务',
    severity: 'critical',
    title: '片头配乐_v4_master.wav 上传后校验失败',
    summary: '上传任务已失败，当前远端副本不可使用，需要重新生成校验摘要并重试上传。',
    asset: '片头配乐_v4_master.wav',
    objectLabel: '片头配乐_v4_master.wav / 本地 NVMe → 115 云归档',
    action: '重试上传并重新生成校验摘要',
    actionLabel: '重试',
    suggestion: '优先重试上传；如果再次失败，再回到文件中心核对源文件和端点状态。',
    detail: '远端校验摘要与本地源文件不一致。',
    status: '待处理',
    createdAt: '今天 09:52',
    updatedAt: '今天 10:01',
    source: {
      sourceDomain: '传输任务',
      taskId: 'task-transfer-5',
      taskItemId: 'task-item-6',
      taskTitle: '音频归档上传',
      taskItemTitle: '片头配乐_v4_master.wav',
      endpointId: 'node-4',
      endpointLabel: '115 云归档',
      sourceLabel: '本地 NVMe → 115 云归档',
      path: '/MareArchive/Interview/audio/片头配乐_v4_master.wav',
      routeLabel: '本地 NVMe → 115 云归档',
    },
    impact: {
      assetCount: 1,
      replicaCount: 1,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canRetry: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
      canOpenFileCenter: true,
      canOpenStorageNodes: true,
    },
    histories: [
      {
        id: 'issue-6-history-1',
        issueId: 'issue-6',
        action: '自动发现',
        operatorLabel: '传输执行器',
        result: '上传完成后校验摘要不一致，任务转为失败。',
        createdAt: '今天 09:52',
      },
    ],
  },
  {
    id: 'issue-7',
    libraryId: 'photo',
    taskId: 'task-other-scan-1',
    taskItemId: 'task-item-other-scan-2',
    category: '扫描与解析',
    type: '扫描失败',
    nature: 'BLOCKING',
    sourceDomain: '其他任务',
    severity: 'warning',
    title: '影像 NAS 01 交付目录扫描失败',
    summary: '扫描任务在归并交付目录时有 12 项暂时不可读，当前目录状态尚未正式提交。',
    asset: '影像 NAS 01',
    objectLabel: '影像 NAS 01 / \\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\delivery',
    action: '稍后重试并检查共享目录权限',
    actionLabel: '重试',
    suggestion: '建议优先重试一次；若仍失败，再去存储节点页排查共享目录和挂载权限。',
    detail: '交付目录中有 12 项暂时无法读取，当前已先完成可访问项归并。',
    status: '待处理',
    createdAt: '今天 09:29',
    updatedAt: '今天 09:41',
    source: {
      sourceDomain: '其他任务',
      taskId: 'task-other-scan-1',
      taskItemId: 'task-item-other-scan-2',
      taskTitle: '影像 NAS 增量扫描',
      taskItemTitle: '交付目录归并',
      endpointId: 'node-2',
      endpointLabel: '影像 NAS 01',
      sourceLabel: '扫描任务 / 归并交付目录',
      path: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\delivery',
    },
    impact: {
      assetCount: 12,
      replicaCount: 12,
      folderCount: 1,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canRetry: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
      canOpenStorageNodes: true,
    },
    histories: [
      {
        id: 'issue-7-history-1',
        issueId: 'issue-7',
        action: '自动发现',
        operatorLabel: '扫描任务',
        result: '部分目录项返回读权限异常。',
        createdAt: '今天 09:29',
      },
    ],
  },
  {
    id: 'issue-8',
    libraryId: 'photo',
    taskId: 'task-other-meta-1',
    taskItemId: 'task-item-other-meta-2',
    category: '扫描与解析',
    type: '元数据字段缺失',
    nature: 'RISK',
    sourceDomain: '其他任务',
    severity: 'warning',
    title: '上海发布会_精选封面.jpg 缺少镜头字段',
    summary: '元数据写入可继续进行，但镜头信息缺失会影响后续检索与治理质量。',
    asset: '上海发布会_精选封面.jpg',
    objectLabel: '上海发布会_精选封面.jpg / /2026/ShanghaiLaunch/delivery',
    action: '允许继续写入基础字段，稍后补全扩展字段',
    actionLabel: '标记确认',
    suggestion: '如果当前以交付时效优先，可先标记确认，后续再补全镜头和拍摄参数字段。',
    detail: '原始图像缺失部分镜头信息，已降级写入基础元数据。',
    status: '待确认',
    createdAt: '今天 09:34',
    updatedAt: '今天 09:40',
    source: {
      sourceDomain: '其他任务',
      taskId: 'task-other-meta-1',
      taskItemId: 'task-item-other-meta-2',
      taskTitle: '上海发布会元数据回填',
      taskItemTitle: '上海发布会_精选封面.jpg',
      fileNodeId: 'photo-file-cover',
      sourceLabel: '元数据回填 / 写入媒体表',
      path: '/2026/ShanghaiLaunch/delivery/上海发布会_精选封面.jpg',
    },
    impact: {
      assetCount: 1,
      replicaCount: 0,
      folderCount: 0,
      endpointCount: 0,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canConfirm: true,
      canPostpone: true,
      canIgnore: true,
      canOpenTaskCenter: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-8-history-1',
        issueId: 'issue-8',
        action: '自动发现',
        operatorLabel: '元数据解析器',
        result: '镜头信息字段缺失，已降级写入基础字段。',
        createdAt: '今天 09:34',
      },
    ],
  },
  {
    id: 'issue-9',
    libraryId: 'family',
    taskId: 'task-other-meta-2',
    taskItemId: 'task-item-other-meta-4',
    category: '校验',
    type: '原文件损坏',
    nature: 'BLOCKING',
    sourceDomain: '其他任务',
    severity: 'critical',
    title: '家庭视频 / 2019 / 机内代理.mov 原文件损坏',
    summary: '媒体头读取失败，无法继续解析或生成结构化字段，需要先恢复可信副本。',
    asset: '家庭视频 / 2019 / 机内代理.mov',
    objectLabel: '家庭视频 / 2019 / 机内代理.mov',
    action: '从其他副本恢复后重新解析',
    actionLabel: '打开文件中心',
    suggestion: '异常中心不适合直接修复损坏文件，建议回到文件中心寻找其他副本后再重试解析。',
    detail: '读取视频封装头失败，无法生成媒体结构化字段。',
    status: '待处理',
    createdAt: '今天 08:52',
    updatedAt: '今天 09:02',
    source: {
      sourceDomain: '其他任务',
      taskId: 'task-other-meta-2',
      taskItemId: 'task-item-other-meta-4',
      taskTitle: '家庭历史视频重建元数据',
      taskItemTitle: '家庭视频 / 2019 / 机内代理.mov',
      sourceLabel: '元数据回填 / 写入媒体表',
      path: '\\\\192.168.10.30\\family\\2019\\机内代理.mov',
    },
    impact: {
      assetCount: 1,
      replicaCount: 2,
      folderCount: 0,
      endpointCount: 2,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canIgnore: true,
      canPostpone: true,
      canOpenTaskCenter: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-9-history-1',
        issueId: 'issue-9',
        action: '自动发现',
        operatorLabel: '元数据解析器',
        result: '读取封装头失败，解析中止。',
        createdAt: '今天 08:52',
      },
    ],
  },
  {
    id: 'issue-10',
    libraryId: 'photo',
    taskId: 'task-other-verify-2',
    taskItemId: 'task-item-other-verify-2',
    category: '校验',
    type: '强校验失败',
    nature: 'BLOCKING',
    sourceDomain: '其他任务',
    severity: 'critical',
    title: '上海发布会_精选组图.zip 强校验失败',
    summary: '强校验确认目标端哈希与源文件哈希不一致，当前归档结果不可可信。',
    asset: '上海发布会_精选组图.zip',
    objectLabel: '上海发布会_精选组图.zip / /MareArchive/2026/ShanghaiLaunch/delivery/package',
    action: '重新同步到目标端后再执行强校验',
    actionLabel: '重试',
    suggestion: '建议先重试归档，再等待夜间强校验重新确认结果。',
    detail: '目标端哈希与源文件哈希不一致，当前结果不可可信。',
    status: '待处理',
    createdAt: '今天 09:16',
    updatedAt: '今天 09:18',
    source: {
      sourceDomain: '其他任务',
      taskId: 'task-other-verify-2',
      taskItemId: 'task-item-other-verify-2',
      taskTitle: '上海发布会交付包强校验',
      taskItemTitle: '上海发布会_精选组图.zip',
      fileNodeId: 'photo-file-gallery-zip',
      endpointId: 'node-4',
      endpointLabel: '115 云归档',
      sourceLabel: '强校验任务 / 写入校验结果',
      path: '/MareArchive/2026/ShanghaiLaunch/delivery/package/上海发布会_精选组图.zip',
    },
    impact: {
      assetCount: 1,
      replicaCount: 1,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canRetry: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-10-history-1',
        issueId: 'issue-10',
        action: '自动发现',
        operatorLabel: '强校验任务',
        result: '目标端哈希与源端哈希不一致。',
        createdAt: '今天 09:16',
      },
    ],
  },
  {
    id: 'issue-11',
    libraryId: 'photo',
    taskId: 'task-other-delete-2',
    taskItemId: 'task-item-other-delete-2',
    category: '清理与治理',
    type: '清理阻塞',
    nature: 'BLOCKING',
    sourceDomain: '其他任务',
    severity: 'warning',
    title: '历史交付包后台清理被标签关联阻塞',
    summary: '治理队列已自动重试，但当前仍有 3 条标签关联未释放，清理流程尚未完成。',
    asset: '历史交付包后台清理',
    objectLabel: '清理标签关联 / 历史交付包',
    action: '等待标签关联释放后继续',
    actionLabel: '刷新检测',
    suggestion: '可先刷新检测确认是否已释放，如果仍阻塞，再跳到任务中心查看治理队列。',
    detail: '有 3 条标签关联暂未释放，当前清理队列已自动稍后重试。',
    status: '处理中',
    createdAt: '今天 08:43',
    updatedAt: '今天 09:12',
    source: {
      sourceDomain: '其他任务',
      taskId: 'task-other-delete-2',
      taskItemId: 'task-item-other-delete-2',
      taskTitle: '交付包扩展数据清理',
      taskItemTitle: '清理标签关联',
      sourceLabel: '治理任务 / 关联数据清理',
      path: 'photo:delivery-history/cleanup-tag-binding',
    },
    impact: {
      assetCount: 34,
      replicaCount: 0,
      folderCount: 1,
      endpointCount: 0,
      blocksStatusCommit: false,
      blocksTaskExecution: true,
    },
    capabilities: {
      canPostpone: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenTaskCenter: true,
    },
    histories: [
      {
        id: 'issue-11-history-1',
        issueId: 'issue-11',
        action: '自动发现',
        operatorLabel: '治理任务',
        result: '标签关联尚未释放，清理任务转入处理中。',
        createdAt: '今天 08:43',
      },
      {
        id: 'issue-11-history-2',
        issueId: 'issue-11',
        action: '自动重试',
        operatorLabel: '治理任务',
        result: '已自动重试 1 次，仍存在 3 条关联记录。',
        createdAt: '今天 09:12',
      },
    ],
  },
  {
    id: 'issue-12',
    libraryId: 'photo',
    category: '节点与权限',
    type: '节点读写权限异常',
    nature: 'BLOCKING',
    sourceDomain: '存储节点',
    severity: 'critical',
    title: '影像 NAS 01 共享目录写入权限异常',
    summary: '存储节点连接仍在线，但交付目录写入测试失败，后续归档任务会被阻塞。',
    asset: '影像 NAS 01',
    objectLabel: '影像 NAS 01 / \\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\delivery',
    action: '检查 NAS 共享权限并重新测试',
    actionLabel: '刷新检测',
    suggestion: '建议先在存储节点页执行连接测试；如权限仍异常，再修正挂载或共享策略。',
    detail: '写入测试返回 Access denied，客户端未获得目标目录写权限。',
    status: '待处理',
    createdAt: '今天 10:06',
    updatedAt: '今天 10:08',
    source: {
      sourceDomain: '存储节点',
      endpointId: 'node-2',
      endpointLabel: '影像 NAS 01',
      sourceLabel: '存储节点连接测试',
      path: '\\\\192.168.10.20\\media\\photo\\2026\\ShanghaiLaunch\\delivery',
    },
    impact: {
      assetCount: 136,
      replicaCount: 136,
      folderCount: 1,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canRefreshCheck: true,
      canPostpone: true,
      canIgnore: true,
      canOpenStorageNodes: true,
      canOpenTaskCenter: true,
    },
    histories: [
      {
        id: 'issue-12-history-1',
        issueId: 'issue-12',
        action: '自动发现',
        operatorLabel: '存储节点页',
        result: '连接测试失败，返回写权限异常。',
        createdAt: '今天 10:06',
      },
    ],
  },
  {
    id: 'issue-13',
    libraryId: 'family',
    category: '清理与治理',
    type: '临时文件待处理',
    nature: 'RISK',
    sourceDomain: '系统治理',
    severity: 'info',
    title: '家庭照片资产库存在 186 项临时文件待处理',
    summary: '系统治理扫描发现多个缓存与临时缩略图文件未清理，当前不阻塞业务，但长期会影响库整洁度。',
    asset: '家庭照片资产库',
    objectLabel: 'Archive / Family / 缓存清理待处理',
    action: '归档已解决项前统一清理',
    actionLabel: '延后处理',
    suggestion: '当前建议延后到低峰时段统一治理，避免影响正在进行的扫描与解析任务。',
    detail: '发现 186 项临时文件与旧缓存缩略图，均可在治理窗口统一清理。',
    status: '已延后',
    createdAt: '昨天 23:14',
    updatedAt: '今天 08:10',
    source: {
      sourceDomain: '系统治理',
      sourceLabel: '夜间治理巡检',
      path: 'Archive / Family / temp-cache',
    },
    impact: {
      assetCount: 186,
      replicaCount: 0,
      folderCount: 7,
      endpointCount: 0,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canConfirm: true,
      canIgnore: true,
      canArchive: true,
      canClearHistory: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-13-history-1',
        issueId: 'issue-13',
        action: '自动发现',
        operatorLabel: '夜间治理巡检',
        result: '发现临时文件与缓存缩略图累计 186 项。',
        createdAt: '昨天 23:14',
      },
      {
        id: 'issue-13-history-2',
        issueId: 'issue-13',
        action: '延后处理',
        operatorLabel: '当前客户端',
        result: '已延后到下一次治理窗口。',
        createdAt: '今天 08:10',
      },
    ],
  },
  {
    id: 'issue-14',
    libraryId: 'photo',
    category: '容量与资源',
    type: '空间不足',
    nature: 'BLOCKING',
    sourceDomain: '系统治理',
    severity: 'critical',
    title: '本地 NVMe 主盘剩余空间不足',
    summary: '后台容量治理判断本地主盘可用空间已经低于硬阈值，新的导入任务会被阻塞。',
    asset: '本地 NVMe 主盘',
    objectLabel: '本地 NVMe 主盘 / D:\\Mare\\Assets',
    action: '清理缓存或迁移热数据',
    actionLabel: '打开存储节点',
    suggestion: '此类问题需要在存储节点或文件中心执行强处理，异常中心只负责预警与统一追踪。',
    detail: '可用空间仅剩 18 GB，低于导入安全阈值 30 GB。',
    status: '待处理',
    createdAt: '今天 10:11',
    updatedAt: '今天 10:12',
    source: {
      sourceDomain: '系统治理',
      endpointId: 'node-1',
      endpointLabel: '本地 NVMe 主盘',
      sourceLabel: '容量治理巡检',
      path: 'D:\\Mare\\Assets',
    },
    impact: {
      assetCount: 0,
      replicaCount: 0,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: true,
      blocksTaskExecution: true,
    },
    capabilities: {
      canPostpone: true,
      canIgnore: true,
      canRefreshCheck: true,
      canOpenStorageNodes: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-14-history-1',
        issueId: 'issue-14',
        action: '自动发现',
        operatorLabel: '容量治理巡检',
        result: '可用空间低于安全阈值，新的导入任务将被阻塞。',
        createdAt: '今天 10:11',
      },
    ],
  },
  {
    id: 'issue-15',
    libraryId: 'photo',
    category: '清理与治理',
    type: '后台治理待确认项',
    nature: 'RISK',
    sourceDomain: '系统治理',
    severity: 'success',
    title: '上海发布会旧版缩略图清理已完成',
    summary: '夜间治理已清理旧版缩略图，记录保留用于回查，可归档或手动清理历史。',
    asset: '上海发布会旧版缩略图',
    objectLabel: '上海发布会 / 旧版缩略图清理',
    action: '归档历史记录',
    actionLabel: '归档',
    suggestion: '如果不再需要保留在主工作区，可直接归档或清理历史。',
    detail: '已清理 64 项旧版缩略图，原始资产与正式副本均未受影响。',
    status: '已解决',
    createdAt: '昨天 02:14',
    updatedAt: '昨天 02:32',
    resolvedAt: '昨天 02:32',
    source: {
      sourceDomain: '系统治理',
      sourceLabel: '夜间治理巡检',
      path: 'photo:delivery-thumbnail-history',
    },
    impact: {
      assetCount: 64,
      replicaCount: 0,
      folderCount: 2,
      endpointCount: 0,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canArchive: true,
      canClearHistory: true,
      canOpenFileCenter: true,
    },
    histories: [
      {
        id: 'issue-15-history-1',
        issueId: 'issue-15',
        action: '自动发现',
        operatorLabel: '夜间治理巡检',
        result: '发现旧版缩略图可安全清理。',
        createdAt: '昨天 02:14',
      },
      {
        id: 'issue-15-history-2',
        issueId: 'issue-15',
        action: '自动处理',
        operatorLabel: '系统治理',
        result: '已清理 64 项旧版缩略图。',
        createdAt: '昨天 02:32',
      },
    ],
  },
  {
    id: 'issue-16',
    libraryId: 'video',
    category: '节点与权限',
    type: '鉴权失效',
    nature: 'BLOCKING',
    sourceDomain: '存储节点',
    severity: 'info',
    title: '115 云归档旧鉴权异常已归档',
    summary: '上周令牌过期引发的异常已被处理完成，保留一条归档记录用于回查。',
    asset: '115 云归档',
    objectLabel: '115 云归档 / 历史异常',
    action: '查看归档记录',
    actionLabel: '查看详情',
    suggestion: '此记录仅用于回查，不需要再次处理。',
    detail: '令牌已更新，相关链路恢复正常。',
    status: '已归档',
    createdAt: '2026-04-02 21:10',
    updatedAt: '2026-04-03 09:08',
    resolvedAt: '2026-04-03 09:08',
    archivedAt: '2026-04-06 22:00',
    source: {
      sourceDomain: '存储节点',
      endpointId: 'node-4',
      endpointLabel: '115 云归档',
      sourceLabel: '存储节点鉴权巡检',
      path: '/MareArchive',
    },
    impact: {
      assetCount: 0,
      replicaCount: 0,
      folderCount: 0,
      endpointCount: 1,
      blocksStatusCommit: false,
      blocksTaskExecution: false,
    },
    capabilities: {
      canArchive: false,
      canClearHistory: true,
      canOpenStorageNodes: true,
    },
    histories: [
      {
        id: 'issue-16-history-1',
        issueId: 'issue-16',
        action: '自动发现',
        operatorLabel: '存储节点鉴权巡检',
        result: '检测到令牌过期。',
        createdAt: '2026-04-02 21:10',
      },
      {
        id: 'issue-16-history-2',
        issueId: 'issue-16',
        action: '刷新令牌',
        operatorLabel: '当前客户端',
        result: '令牌已更新，链路恢复正常。',
        createdAt: '2026-04-03 09:08',
      },
      {
        id: 'issue-16-history-3',
        issueId: 'issue-16',
        action: '自动归档',
        operatorLabel: '异常中心',
        result: '已按保留策略归档。',
        createdAt: '2026-04-06 22:00',
      },
    ],
  },
];

export const headerSignals: HeaderSignal[] = [
  {
    id: 'header-signal-device-t7',
    signalType: 'DEVICE_INSERTED',
    tone: 'success',
    title: '现场移动硬盘 T7 已插入',
    summary: '检测到 1,824 项待导入文件，可前往导入中心继续分批导入。',
    createdAt: '刚刚',
    deviceId: 'node-3',
    deviceName: '现场移动硬盘 T7',
    jumpParams: {
      kind: 'import-center',
      endpointId: 'node-3',
      label: '现场移动硬盘 T7 已插入',
    },
  },
];

taskRecords.splice(0, taskRecords.length);
taskItemRecords.splice(0, taskItemRecords.length);
issueRecords.splice(0, issueRecords.length);

export const storageNodes: StorageNode[] = [
  {
    id: 'node-1',
    name: '本地 NVMe 主盘',
    nodeType: '本机磁盘',
    address: 'D:\\Mare\\Assets',
    mountMode: '可写',
    status: '在线',
    freeSpace: '3.4 TB 可用',
    lastCheck: '刚刚',
    capacityPercent: 64,
  },
  {
    id: 'node-2',
    name: '影像 NAS 01',
    nodeType: 'NAS/SMB',
    address: '\\\\192.168.10.20\\media',
    mountMode: '可写',
    status: '在线',
    freeSpace: '18.9 TB 可用',
    lastCheck: '1 分钟前',
    capacityPercent: 48,
  },
  {
    id: 'node-3',
    name: '现场移动硬盘 T7',
    nodeType: '移动硬盘',
    address: 'E:\\shooting-card',
    mountMode: '只读',
    status: '已挂载',
    freeSpace: '1.2 TB 可用',
    lastCheck: '13 分钟前',
    capacityPercent: 82,
  },
  {
    id: 'node-4',
    name: '115 云归档',
    nodeType: '115网盘',
    address: '/MareArchive',
    mountMode: '可写',
    status: '鉴权正常',
    freeSpace: '远端容量正常',
    lastCheck: '6 分钟前',
    capacityPercent: 37,
  },
];

export const settingsTabs: Array<{ id: SettingsTab; label: string }> = [
  { id: 'general', label: '通用' },
  { id: 'workspace', label: '工作区' },
  { id: 'file-overview', label: '文件总览' },
  { id: 'tag-management', label: '标签管理' },
  { id: 'import-archive', label: '导入与归档' },
  { id: 'notifications', label: '通知与提醒' },
  { id: 'issue-governance', label: '异常治理' },
  { id: 'verification', label: '校验恢复' },
  { id: 'background-tasks', label: '后台任务与性能' },
  { id: 'appearance', label: '外观' },
];

export const settingsContent: Record<SettingsTab, SettingSection[]> = {
  general: [
    {
      id: 'launch',
      title: '启动与默认项',
      rows: [
        {
          id: 'default-library',
          label: '默认打开资产库',
          value: '上次使用',
          control: 'select',
          options: ['上次使用'],
        },
        {
          id: 'folder-pattern',
          label: '新建目录命名规则',
          value: 'yyyy-MM-dd 项目名',
          control: 'input',
          description: '影响文件中心与导入中心中的目录命名建议。',
        },
        {
          id: 'capacity-unit',
          label: '容量显示单位',
          value: '自动',
          control: 'segmented',
          options: ['自动', 'GB', 'TB'],
        },
        {
          id: 'danger-confirmation',
          label: '危险操作二次确认',
          value: '始终提醒',
          control: 'select',
          options: ['始终提醒', '仅高风险操作', '保持当前原型口径'],
        },
      ],
    },
  ],
  workspace: [
    {
      id: 'workspace-defaults',
      title: '工作区默认行为',
      rows: [
        {
          id: 'startup-page',
          label: '默认打开页面',
          value: '文件中心',
          control: 'select',
          options: ['文件中心', '任务中心', '异常中心', '存储节点', '设置', '导入中心'],
          description: '决定客户端启动后默认聚焦的一级页面。',
        },
        {
          id: 'close-fallback',
          label: '关闭当前标签后的回退页',
          value: '优先右侧标签',
          control: 'select',
          options: ['优先右侧标签', '优先左侧标签', '保持当前客户端口径'],
        },
        {
          id: 'preserve-context',
          label: '切换页面时保留筛选与草稿',
          value: '开启',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
      ],
    },
    {
      id: 'workspace-focus',
      title: '跳转与聚焦',
      rows: [
        {
          id: 'notice-open-detail',
          label: '通知跳转时自动打开详情',
          value: '开启',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
        {
          id: 'import-open-target',
          label: '导入入口默认聚焦',
          value: '最近活跃会话',
          control: 'select',
          options: ['最近活跃会话', '待导入设备池', '保持当前实现'],
        },
        {
          id: 'global-density',
          label: '全局列表密度',
          value: '紧凑',
          control: 'segmented',
          options: ['舒展', '紧凑', '高密'],
        },
      ],
    },
  ],
  'file-overview': [
    {
      id: 'overview',
      title: '默认浏览体验',
      rows: [
        {
          id: 'default-columns',
          label: '默认列集',
          value: '名称 / 修改日期 / 类型 / 大小 / 多端状态',
          control: 'select',
          options: ['名称 / 修改日期 / 类型 / 大小 / 多端状态', '名称 / 路径 / 类型 / 大小', '名称 / 评分 / 标签 / 同步状态'],
        },
        {
          id: 'default-sort',
          label: '默认排序',
          value: '修改日期降序',
          control: 'select',
          options: ['修改日期降序', '修改日期升序', '名称升序', '大小降序'],
        },
        {
          id: 'default-page-size',
          label: '每页默认数量',
          value: '20',
          control: 'segmented',
          options: ['10', '20', '50', '100'],
        },
      ],
    },
    {
      id: 'overview-detail',
      title: '搜索与详情偏好',
      rows: [
        {
          id: 'default-search-scope',
          label: '默认搜索范围',
          value: '名称 / 路径 / 来源端',
          control: 'select',
          options: ['名称 / 路径 / 来源端', '名称 / 标签 / 路径', '名称 / 路径 / 标签 / 来源端'],
        },
        {
          id: 'folder-open-mode',
          label: '目录默认打开方式',
          value: '双击进入',
          control: 'segmented',
          options: ['双击进入', '单击选中'],
        },
        {
          id: 'detail-default-block',
          label: '详情抽屉默认焦点区块',
          value: '基础信息',
          control: 'select',
          options: ['基础信息', '元数据', '最近任务', '标签与备注'],
        },
      ],
    },
  ],
  'tag-management': [],
  'import-archive': [
    {
      id: 'import-defaults',
      title: '导入默认策略',
      rows: [
        {
          id: 'default-import-targets',
          label: '默认导入目标策略',
          value: '按资产库推荐目标端',
          control: 'select',
          options: ['按资产库推荐目标端', '沿用最近一次选择', '仅应用主目标端'],
        },
        {
          id: 'default-import-folder-pattern',
          label: '导入目录命名规则',
          value: 'yyyy-MM-dd_项目名_机位',
          control: 'input',
        },
        {
          id: 'auto-precheck',
          label: '进入会话时自动预检',
          value: '开启',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
        {
          id: 'blocking-policy',
          label: '提交前阻塞规则',
          value: '存在阻塞项时禁止提交',
          control: 'select',
          options: ['存在阻塞项时禁止提交', '仅提醒但允许继续', '保持当前实现'],
        },
      ],
    },
    {
      id: 'archive-defaults',
      title: '归档与结果保留',
      rows: [
        {
          id: 'post-import-verify',
          label: '导入后默认校验',
          value: '大小 + 修改时间',
          control: 'segmented',
          options: ['大小 + 修改时间', '强校验', '不自动校验'],
        },
        {
          id: 'import-report-retention',
          label: '导入摘要保留时长',
          value: '30 天',
          control: 'select',
          options: ['7 天', '30 天', '90 天', '永久保留'],
        },
        {
          id: 'device-session-retention',
          label: '设备拔出后会话保留',
          value: '24 小时',
          control: 'select',
          options: ['立即关闭', '24 小时', '7 天', '直到手动清理'],
        },
      ],
    },
  ],
  notifications: [
    {
      id: 'notice-thresholds',
      title: '提醒阈值',
      rows: [
        {
          id: 'capacity-warning-threshold',
          label: '容量预警阈值',
          value: '剩余 15%',
          control: 'select',
          options: ['剩余 10%', '剩余 15%', '剩余 20%', '仅阻塞时提醒'],
        },
        {
          id: 'auth-expiry-lead',
          label: '鉴权到期提前提醒',
          value: '12 小时',
          control: 'select',
          options: ['6 小时', '12 小时', '24 小时', '48 小时'],
        },
        {
          id: 'metadata-reminder',
          label: '元数据缺失进入提醒',
          value: '开启',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
      ],
    },
    {
      id: 'notice-display',
      title: '展示与保留',
      rows: [
        {
          id: 'reminder-retention',
          label: '提醒类通知保留时长',
          value: '7 天',
          control: 'select',
          options: ['1 天', '7 天', '30 天', '直到手动清理'],
        },
        {
          id: 'quiet-hours',
          label: '安静时段',
          value: '23:00 - 08:00',
          control: 'input',
        },
        {
          id: 'signal-style',
          label: '系统提示方式',
          value: '页头角标 + 浮窗',
          control: 'select',
          options: ['页头角标 + 浮窗', '仅页头角标', '仅右侧浮窗'],
        },
      ],
    },
  ],
  'issue-governance': [
    {
      id: 'issue-retention',
      title: '历史保留策略',
      rows: [
        {
          id: 'issue-history-count',
          label: '最大保留条数',
          value: '500 条',
          control: 'select',
          options: ['100 条', '300 条', '500 条', '1000 条'],
          description: '仅作用于已解决、已忽略和已归档异常。',
        },
        {
          id: 'issue-history-age',
          label: '最长保留时间',
          value: '90 天',
          control: 'select',
          options: ['30 天', '60 天', '90 天', '180 天'],
          description: '超出保留时间的历史异常可在清理时被移除。',
        },
        {
          id: 'issue-auto-archive',
          label: '自动归档已解决异常',
          value: '开启',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
        {
          id: 'issue-auto-purge',
          label: '自动清理已归档历史',
          value: '关闭',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
      ],
    },
    {
      id: 'issue-rhythm',
      title: '治理节奏',
      rows: [
        {
          id: 'postpone-remind',
          label: '延后项再次提醒',
          value: '24 小时后',
          control: 'select',
          options: ['4 小时后', '24 小时后', '72 小时后', '仅手动回查'],
        },
        {
          id: 'history-visibility',
          label: '忽略项历史显示',
          value: '保留在历史视图',
          control: 'select',
          options: ['保留在历史视图', '仅统计不展示', '保持当前实现'],
        },
      ],
    },
  ],
  verification: [
    {
      id: 'verify',
      title: '校验与恢复',
      rows: [
        {
          id: 'verify-mode',
          label: '默认校验',
          value: '大小 + 修改时间',
          control: 'segmented',
          options: ['大小 + 修改时间', '强校验', '快速校验'],
        },
        {
          id: 'verify-window',
          label: '强校验窗口',
          value: '22:00 - 06:00',
          control: 'input',
        },
        {
          id: 'retry-count',
          label: '失败自动重试',
          value: '2 次',
          control: 'select',
          options: ['不重试', '1 次', '2 次', '3 次'],
        },
      ],
    },
    {
      id: 'verify-recovery',
      title: '恢复策略',
      rows: [
        {
          id: 'verify-resume-policy',
          label: '时间窗结束后的恢复方式',
          value: '下一窗口继续上次进度',
          control: 'select',
          options: ['下一窗口继续上次进度', '暂停并等待人工继续', '保持当前架构口径'],
        },
        {
          id: 'verify-result-retention',
          label: '强校验结果缓存保留',
          value: '180 天',
          control: 'select',
          options: ['30 天', '90 天', '180 天', '永久保留'],
        },
      ],
    },
  ],
  'background-tasks': [
    {
      id: 'background-concurrency',
      title: '后台并发',
      rows: [
        {
          id: 'parallel-jobs',
          label: '并行任务',
          value: '4 个任务 / 12 个子项',
          control: 'select',
          options: ['2 个任务 / 6 个子项', '4 个任务 / 12 个子项', '6 个任务 / 20 个子项'],
        },
        {
          id: 'scan-frequency',
          label: '后台扫描频率',
          value: '每 30 分钟',
          control: 'select',
          options: ['仅手动触发', '每 30 分钟', '每 2 小时', '每日一次'],
        },
        {
          id: 'cleanup-schedule',
          label: '删除清理调度',
          value: '系统空闲时优先执行',
          control: 'select',
          options: ['系统空闲时优先执行', '夜间时间窗执行', '立即进入队列'],
        },
      ],
    },
    {
      id: 'background-processing',
      title: '解析与性能',
      rows: [
        {
          id: 'metadata-parse',
          label: '元数据解析',
          value: '开启',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
        {
          id: 'verify-priority',
          label: '校验任务优先级',
          value: '普通优先级',
          control: 'segmented',
          options: ['低优先级', '普通优先级', '高优先级'],
        },
        {
          id: 'low-performance-mode',
          label: '低性能模式',
          value: '关闭',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
      ],
    },
  ],
  appearance: [
    {
      id: 'appearance',
      title: '外观',
      rows: [
        {
          id: 'theme',
          label: '主题',
          value: '浅色主题',
          control: 'segmented',
          options: ['跟随系统', '浅色主题', '深色主题'],
        },
        {
          id: 'compact-sidebar',
          label: '侧栏紧凑模式',
          value: '关闭',
          control: 'toggle',
          options: ['关闭', '开启'],
        },
        {
          id: 'font-size',
          label: '字号',
          value: '中',
          control: 'segmented',
          options: ['小', '中', '大'],
        },
      ],
    },
  ],
};

export function cloneSettingsContent(): Record<SettingsTab, SettingSection[]> {
  return {
    general: settingsContent.general.map(cloneSection),
    workspace: settingsContent.workspace.map(cloneSection),
    'file-overview': settingsContent['file-overview'].map(cloneSection),
    'tag-management': settingsContent['tag-management'].map(cloneSection),
    'import-archive': settingsContent['import-archive'].map(cloneSection),
    notifications: settingsContent.notifications.map(cloneSection),
    'issue-governance': settingsContent['issue-governance'].map(cloneSection),
    verification: settingsContent.verification.map(cloneSection),
    'background-tasks': settingsContent['background-tasks'].map(cloneSection),
    appearance: settingsContent.appearance.map(cloneSection),
  };
}

function cloneSection(section: SettingSection): SettingSection {
  return {
    ...section,
    rows: section.rows.map((row) => ({
      ...row,
      options: row.options ? [...row.options] : undefined,
    })),
  };
}
