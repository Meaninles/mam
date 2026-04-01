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
  | 'file-overview'
  | 'tag-management'
  | 'verification'
  | 'performance'
  | 'appearance';
export type TaskTab = 'transfer' | 'other';
export type FileTypeFilter = '全部' | '文件夹' | '图片' | '视频' | '音频' | '文档';
export type StorageTypeFilter = '全部' | '本机磁盘' | '移动硬盘' | 'NAS/SMB' | '115网盘';
export type SettingControlType = 'toggle' | 'select' | 'input' | 'segmented';
export type AssetLifecycleState = 'ACTIVE' | 'PENDING_DELETE';

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

export interface TaskRecord {
  id: string;
  kind: TaskTab;
  title: string;
  type: string;
  status: string;
  statusTone: Severity;
  libraryId: string;
  source?: string;
  target?: string;
  progress: number;
  speed: string;
  eta: string;
  fileCount: number;
  multiFile: boolean;
  updatedAt: string;
}

export interface TaskItemRecord {
  id: string;
  taskId: string;
  name: string;
  status: string;
  statusTone: Severity;
  progress: number;
  size: string;
  speed: string;
}

export interface IssueRecord {
  id: string;
  libraryId: string;
  type: string;
  severity: Severity;
  asset: string;
  action: string;
  detail: string;
  status: string;
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

export const libraries: Library[] = [
  {
    id: 'photo',
    name: '商业摄影资产库',
    rootLabel: '2026 / Shanghai Launch',
    itemCount: '46,820',
    health: '98.2%',
    storagePolicy: '本地 + NAS + 115',
  },
  {
    id: 'video',
    name: '视频工作流资产库',
    rootLabel: '2026 / Interview',
    itemCount: '28,406',
    health: '91.6%',
    storagePolicy: '本地 + NAS',
  },
  {
    id: 'family',
    name: '家庭照片资产库',
    rootLabel: 'Archive / Family',
    itemCount: '49,222',
    health: '99.4%',
    storagePolicy: '本地 + NAS + 云归档',
  },
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

export const taskRecords: TaskRecord[] = [
  {
    id: 'task-transfer-1',
    kind: 'transfer',
    title: '发布会原片补齐到 NAS',
    type: 'SYNC',
    status: '运行中',
    statusTone: 'warning',
    libraryId: 'photo',
    source: '本地NVMe',
    target: '影像NAS',
    progress: 72,
    speed: '182 MB/s',
    eta: '12 分钟',
    fileCount: 124,
    multiFile: true,
    updatedAt: '刚刚',
  },
  {
    id: 'task-transfer-2',
    kind: 'transfer',
    title: '现场移动硬盘入库',
    type: 'IMPORT',
    status: '等待确认',
    statusTone: 'info',
    libraryId: 'photo',
    source: '现场移动硬盘 T7',
    target: '本地NVMe、影像NAS、115',
    progress: 38,
    speed: '96 MB/s',
    eta: '24 分钟',
    fileCount: 1824,
    multiFile: true,
    updatedAt: '3 分钟前',
  },
  {
    id: 'task-transfer-3',
    kind: 'transfer',
    title: '客户访谈精编版同步',
    type: 'COPY',
    status: '同步中',
    statusTone: 'warning',
    libraryId: 'video',
    source: '本地NVMe',
    target: '影像NAS',
    progress: 56,
    speed: '128 MB/s',
    eta: '7 分钟',
    fileCount: 1,
    multiFile: false,
    updatedAt: '1 分钟前',
  },
  {
    id: 'task-other-1',
    kind: 'other',
    title: '家庭照片夜间强校验',
    type: 'VERIFY',
    status: '暂停中',
    statusTone: 'info',
    libraryId: 'family',
    progress: 54,
    speed: '40 MB/s',
    eta: '今晚 22:00 继续',
    fileCount: 6400,
    multiFile: true,
    updatedAt: '12 分钟前',
  },
  {
    id: 'task-other-2',
    kind: 'other',
    title: '片头配乐校验',
    type: 'VERIFY',
    status: '失败',
    statusTone: 'critical',
    libraryId: 'video',
    progress: 100,
    speed: '—',
    eta: '已结束',
    fileCount: 1,
    multiFile: false,
    updatedAt: '28 分钟前',
  },
];

export const taskItemRecords: TaskItemRecord[] = [
  {
    id: 'task-item-1',
    taskId: 'task-transfer-1',
    name: 'A-cam_001.RAW',
    status: '已完成',
    statusTone: 'success',
    progress: 100,
    size: '48.2 MB',
    speed: '—',
  },
  {
    id: 'task-item-2',
    taskId: 'task-transfer-1',
    name: 'B-cam_018.RAW',
    status: '同步中',
    statusTone: 'warning',
    progress: 72,
    size: '47.8 MB',
    speed: '182 MB/s',
  },
  {
    id: 'task-item-3',
    taskId: 'task-transfer-1',
    name: '精选封面.jpg',
    status: '等待执行',
    statusTone: 'info',
    progress: 0,
    size: '12.4 MB',
    speed: '—',
  },
  {
    id: 'task-item-4',
    taskId: 'task-transfer-2',
    name: 'A001_C001_0329.mov',
    status: '导入中',
    statusTone: 'warning',
    progress: 43,
    size: '16.2 GB',
    speed: '96 MB/s',
  },
  {
    id: 'task-item-5',
    taskId: 'task-transfer-2',
    name: 'RAW_0001.ARW',
    status: '等待导入',
    statusTone: 'info',
    progress: 0,
    size: '48.2 MB',
    speed: '—',
  },
  {
    id: 'task-item-6',
    taskId: 'task-other-1',
    name: 'IMG_3288.HEIC',
    status: '已校验',
    statusTone: 'success',
    progress: 100,
    size: '3.8 MB',
    speed: '—',
  },
  {
    id: 'task-item-7',
    taskId: 'task-other-1',
    name: 'IMG_3289.HEIC',
    status: '等待下个窗口',
    statusTone: 'info',
    progress: 54,
    size: '4.0 MB',
    speed: '40 MB/s',
  },
];

export const issueRecords: IssueRecord[] = [
  {
    id: 'issue-1',
    libraryId: 'video',
    type: '路径冲突',
    severity: 'critical',
    asset: '客户访谈_第一机位_精编版.mov',
    action: '重命名旧版母带或执行删除资产',
    detail: '目标目录已有同名母带，继续同步会覆盖已交付版本。',
    status: '待处理',
  },
  {
    id: 'issue-2',
    libraryId: 'video',
    type: '校验失败',
    severity: 'critical',
    asset: '片头配乐_v4_master.wav',
    action: '从本地副本重新同步到 NAS',
    detail: '影像 NAS 返回的校验值与本地源文件不一致。',
    status: '待处理',
  },
  {
    id: 'issue-3',
    libraryId: 'photo',
    type: '鉴权提醒',
    severity: 'warning',
    asset: '115 云归档',
    action: '刷新令牌并重新检测容量',
    detail: '距离令牌过期不足 48 小时，继续归档存在中断风险。',
    status: '待确认',
  },
  {
    id: 'issue-4',
    libraryId: 'photo',
    type: '历史临时文件',
    severity: 'info',
    asset: 'cache_export.tmp',
    action: '忽略或执行删除资产',
    detail: '缓存导出临时文件已 14 天未访问。',
    status: '待处理',
  },
  {
    id: 'issue-5',
    libraryId: 'photo',
    type: '空间不足',
    severity: 'warning',
    asset: '影像NAS',
    action: '清理旧副本或切换目标端',
    detail: '预计 3 天内达到容量阈值，导入任务可能被阻塞。',
    status: '待处理',
  },
];

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
  { id: 'file-overview', label: '文件总览' },
  { id: 'tag-management', label: '标签管理' },
  { id: 'verification', label: '校验恢复' },
  { id: 'performance', label: '性能' },
  { id: 'appearance', label: '外观' },
];

export const settingsContent: Record<SettingsTab, SettingSection[]> = {
  general: [
    {
      id: 'library',
      title: '资产库',
      rows: [
        {
          id: 'default-library',
          label: '默认打开资产库',
          value: '上次使用',
          control: 'select',
          options: ['上次使用', '商业摄影资产库', '视频工作流资产库', '家庭照片资产库'],
        },
        {
          id: 'folder-pattern',
          label: '新建目录命名规则',
          value: 'yyyy-MM-dd 项目名',
          control: 'input',
        },
      ],
    },
  ],
  'file-overview': [
    {
      id: 'overview',
      title: '文件总览',
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
  ],
  'tag-management': [],
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
  ],
  performance: [
    {
      id: 'perf',
      title: '性能',
      rows: [
        {
          id: 'list-page-size',
          label: '列表分页',
          value: '每页 200 项',
          control: 'select',
          options: ['每页 100 项', '每页 200 项', '每页 500 项'],
        },
        {
          id: 'parallel-jobs',
          label: '并行任务',
          value: '4 个任务 / 12 个子项',
          control: 'select',
          options: ['2 个任务 / 6 个子项', '4 个任务 / 12 个子项', '6 个任务 / 20 个子项'],
        },
        {
          id: 'metadata-parse',
          label: '元数据解析',
          value: '开启',
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
          value: '深色主题',
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
    'file-overview': settingsContent['file-overview'].map(cloneSection),
    'tag-management': settingsContent['tag-management'].map(cloneSection),
    verification: settingsContent.verification.map(cloneSection),
    performance: settingsContent.performance.map(cloneSection),
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
