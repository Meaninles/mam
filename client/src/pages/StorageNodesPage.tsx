import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import QRCode from 'qrcode';
import {
  ChevronLeft,
  ChevronRight,
  CircleDashed,
  CircleEllipsis,
  LoaderCircle,
  Pencil,
  Plus,
  Radar,
  RefreshCw,
} from 'lucide-react';
import type { Library } from '../data';
import {
  ActionButton,
  DenseRow,
  EmptyState,
  IconButton,
  ProgressBar,
  SelectPill,
  Sheet,
  TabSwitch,
  TonePill,
} from '../components/Shared';
import type {
  CloudDraft,
  CloudRecord,
  LocalNodeDraft,
  LocalNodeRecord,
  MountFolderDraft,
  MountFolderRecord,
  NasDraft,
  NasRecord,
  StorageCloudAccessMethod,
  StorageCloudQrChannel,
  StorageConnectionTestResult,
  StorageHeartbeatPolicy,
  StorageMountMode,
  StorageNodesDashboard,
  StorageScanHistoryItem,
} from '../lib/storageNodesApi';
import { storageNodesApi } from '../lib/storageNodesApi';

type StorageSubPage = 'mounts' | 'nas' | 'cloud';
type StoragePageSize = 10 | 20 | 50 | 100;
type FeedbackTone = 'success' | 'warning' | 'critical' | 'info';
type FeedbackState = { message: string; tone: FeedbackTone } | null;
type StatusFilter = '全部' | '可用' | '异常' | '已停用' | '扫描中';

type MountFormState = {
  draft: MountFolderDraft;
  errors: Partial<Record<string, string>>;
  saving: boolean;
};

type LocalNodeFormState = {
  draft: LocalNodeDraft;
  errors: Partial<Record<string, string>>;
  saving: boolean;
};

type NasFormState = {
  draft: NasDraft;
  errors: Partial<Record<string, string>>;
  saving: boolean;
};

type CloudFormState = {
  draft: CloudDraft;
  errors: Partial<Record<string, string>>;
  saving: boolean;
};

type HistoryState = {
  mountName: string;
  loading: boolean;
  items: StorageScanHistoryItem[];
};

type StorageMenuState = {
  type: 'mount' | 'nas' | 'cloud';
  id: string;
  top: number;
  right: number;
} | null;

type MountSourceOption = {
  id: string;
  label: string;
  sourceType: '本地' | 'NAS' | '网盘';
};

const SUB_PAGES: Array<{ id: StorageSubPage; label: string }> = [
  { id: 'mounts', label: '本地文件夹管理' },
  { id: 'nas', label: 'NAS 管理' },
  { id: 'cloud', label: '网盘管理' },
];

const STATUS_OPTIONS: StatusFilter[] = ['全部', '可用', '异常', '已停用', '扫描中'];
const HEARTBEAT_OPTIONS: StorageHeartbeatPolicy[] = ['从不', '每周（深夜）', '每日（深夜）', '每小时'];
const CLOUD_ACCESS_OPTIONS: StorageCloudAccessMethod[] = ['填入 Token', '扫码登录获取 Token'];
const CLOUD_QR_OPTIONS: StorageCloudQrChannel[] = ['微信小程序', '支付宝小程序', '电视端'];
const PAGE_SIZE_OPTIONS: StoragePageSize[] = [10, 20, 50, 100];
const ROW_MENU_ESTIMATED_HEIGHT = 220;

const EMPTY_LOCAL_NODE_DRAFT: LocalNodeDraft = {
  name: '',
  rootPath: '',
  notes: '',
};

const EMPTY_MOUNT_DRAFT: MountFolderDraft = {
  name: '',
  libraryId: '',
  nodeId: '',
  mountMode: '可写',
  heartbeatPolicy: '从不',
  relativePath: '',
  notes: '',
  folderType: '本地',
};

const EMPTY_NAS_DRAFT: NasDraft = {
  name: '',
  address: '',
  username: '',
  password: '',
  notes: '',
};

const EMPTY_CLOUD_DRAFT: CloudDraft = {
  name: '',
  vendor: '115',
  accessMethod: '扫码登录获取 Token',
  qrChannel: '微信小程序',
  mountDirectory: '',
  token: '',
  notes: '',
};

export function StorageNodesPage({
  libraries,
  focusRequest,
  onConsumeFocusRequest,
  onFeedback,
  onOpenIssueCenter,
  onOpenTaskCenter,
}: {
  libraries: Library[];
  focusRequest?: { id?: string; label: string; path?: string } | null;
  onConsumeFocusRequest?: () => void;
  onFeedback?: (value: FeedbackState) => void;
  onOpenIssueCenter?: (context: { id: string; label: string; path?: string }) => void;
  onOpenTaskCenter?: (id: string) => void;
}) {
  const [loading, setLoading] = useState(true);
  const [dashboard, setDashboard] = useState<StorageNodesDashboard>({
    localNodes: [],
    nasNodes: [],
    cloudNodes: [],
    mounts: [],
    mountFolders: [],
  });
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [subPage, setSubPage] = useState<StorageSubPage>('mounts');
  const [showMountManagement, setShowMountManagement] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('全部');
  const [mountLibraryFilter, setMountLibraryFilter] = useState('全部资产库');
  const [searchText, setSearchText] = useState('');
  const [selectedMountIds, setSelectedMountIds] = useState<string[]>([]);
  const [localNodeForm, setLocalNodeForm] = useState<LocalNodeFormState | null>(null);
  const [mountForm, setMountForm] = useState<MountFormState | null>(null);
  const [nasForm, setNasForm] = useState<NasFormState | null>(null);
  const [cloudForm, setCloudForm] = useState<CloudFormState | null>(null);
  const [historyState, setHistoryState] = useState<HistoryState | null>(null);
  const [connectionResults, setConnectionResults] = useState<StorageConnectionTestResult[] | null>(null);
  const [heartbeatEditor, setHeartbeatEditor] = useState<{ ids: string[]; value: StorageHeartbeatPolicy; saving: boolean } | null>(null);
  const [menuState, setMenuState] = useState<StorageMenuState>(null);
  const [scanningIds, setScanningIds] = useState<string[]>([]);
  const [testingIds, setTestingIds] = useState<string[]>([]);
  const [testingNasIds, setTestingNasIds] = useState<string[]>([]);
  const [testingCloudIds, setTestingCloudIds] = useState<string[]>([]);
  const [pageBySubPage, setPageBySubPage] = useState<Record<StorageSubPage, number>>({
    mounts: 1,
    nas: 1,
    cloud: 1,
  });
  const [pageSizeBySubPage, setPageSizeBySubPage] = useState<Record<StorageSubPage, StoragePageSize>>({
    mounts: 20,
    nas: 20,
    cloud: 20,
  });

  useEffect(() => {
    void refreshDashboard();
  }, []);

  useEffect(() => {
    onFeedback?.(feedback);
  }, [feedback, onFeedback]);

  useEffect(() => {
    setPageBySubPage((current) => ({ ...current, mounts: 1 }));
  }, [searchText, statusFilter, mountLibraryFilter, showMountManagement]);

  useEffect(() => {
    setPageBySubPage((current) => ({ ...current, nas: 1 }));
  }, [searchText, statusFilter, subPage]);

  useEffect(() => {
    setPageBySubPage((current) => ({ ...current, cloud: 1 }));
  }, [searchText, statusFilter, subPage]);

  useEffect(() => {
    if (!focusRequest) {
      return;
    }

    setSubPage('mounts');
    setShowMountManagement(false);
    setStatusFilter('全部');
    setMountLibraryFilter('全部资产库');
    setSearchText(focusRequest.path ?? focusRequest.label);
    onConsumeFocusRequest?.();
  }, [focusRequest, onConsumeFocusRequest]);

  useEffect(() => {
    if (!menuState) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      if (target.closest('.storage-menu-anchor')) return;
      if (target.closest('.context-menu')) return;
      setMenuState(null);
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, [menuState]);

  const visibleMounts = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return dashboard.mounts.filter((item) => {
      const matchesStatus = statusFilter === '全部' ? true : resolveMountStatus(item) === statusFilter;
      const matchesLibrary = mountLibraryFilter === '全部资产库' ? true : item.libraryName === mountLibraryFilter;
      const haystack = `${item.name} ${item.address} ${item.libraryName} ${item.nodeName}`.toLowerCase();
      return matchesStatus && matchesLibrary && (keyword ? haystack.includes(keyword) : true);
    });
  }, [dashboard.mounts, mountLibraryFilter, searchText, statusFilter]);

  const visibleLocalNodes = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return dashboard.localNodes.filter((item) => {
      const matchesStatus = statusFilter === '全部' ? true : resolveNodeStatus(item) === statusFilter;
      const haystack = `${item.name} ${item.rootPath}`.toLowerCase();
      return matchesStatus && (keyword ? haystack.includes(keyword) : true);
    });
  }, [dashboard.localNodes, searchText, statusFilter]);

  const visibleNas = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return dashboard.nasNodes.filter((item) =>
      keyword ? `${item.name} ${item.address} ${item.username}`.toLowerCase().includes(keyword) : true,
    );
  }, [dashboard.nasNodes, searchText]);

  const visibleCloud = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return dashboard.cloudNodes.filter((item) =>
      keyword ? `${item.name} ${item.mountDirectory} ${item.vendor}`.toLowerCase().includes(keyword) : true,
    );
  }, [dashboard.cloudNodes, searchText]);

  const mountPageCount = Math.max(1, Math.ceil((showMountManagement ? visibleMounts : visibleLocalNodes).length / pageSizeBySubPage.mounts));
  const nasPageCount = Math.max(1, Math.ceil(visibleNas.length / pageSizeBySubPage.nas));
  const cloudPageCount = Math.max(1, Math.ceil(visibleCloud.length / pageSizeBySubPage.cloud));
  const mountLibraryOptions = useMemo(() => ['全部资产库', ...libraries.map((library) => library.name)], [libraries]);
  const mountSourceOptions = useMemo<MountSourceOption[]>(
    () => [
      ...dashboard.localNodes.map((node) => ({
        id: node.id,
        label: `本地 · ${node.name}`,
        sourceType: '本地' as const,
      })),
      ...dashboard.nasNodes.map((node) => ({
        id: node.id,
        label: `NAS · ${node.name}`,
        sourceType: 'NAS' as const,
      })),
      ...dashboard.cloudNodes.map((node) => ({
        id: node.id,
        label: `网盘 · ${node.name}`,
        sourceType: '网盘' as const,
      })),
    ],
    [dashboard.cloudNodes, dashboard.localNodes, dashboard.nasNodes],
  );
  const mountSourceTypeOptions = useMemo<Array<MountSourceOption['sourceType']>>(() => {
    const types = new Set<MountSourceOption['sourceType']>(mountSourceOptions.map((item) => item.sourceType));
    return Array.from(types);
  }, [mountSourceOptions]);
  const visibleMountSourceOptions = useMemo(
    () => mountSourceOptions.filter((item) => item.sourceType === (mountForm?.draft.folderType ?? '本地')),
    [mountForm?.draft.folderType, mountSourceOptions],
  );

  const pagedMounts = useMemo(() => {
    const start = (pageBySubPage.mounts - 1) * pageSizeBySubPage.mounts;
    return visibleMounts.slice(start, start + pageSizeBySubPage.mounts);
  }, [pageBySubPage.mounts, pageSizeBySubPage.mounts, visibleMounts]);

  const pagedLocalNodes = useMemo(() => {
    const start = (pageBySubPage.mounts - 1) * pageSizeBySubPage.mounts;
    return visibleLocalNodes.slice(start, start + pageSizeBySubPage.mounts);
  }, [pageBySubPage.mounts, pageSizeBySubPage.mounts, visibleLocalNodes]);

  const pagedNas = useMemo(() => {
    const start = (pageBySubPage.nas - 1) * pageSizeBySubPage.nas;
    return visibleNas.slice(start, start + pageSizeBySubPage.nas);
  }, [pageBySubPage.nas, pageSizeBySubPage.nas, visibleNas]);

  const pagedCloud = useMemo(() => {
    const start = (pageBySubPage.cloud - 1) * pageSizeBySubPage.cloud;
    return visibleCloud.slice(start, start + pageSizeBySubPage.cloud);
  }, [pageBySubPage.cloud, pageSizeBySubPage.cloud, visibleCloud]);

  const allVisibleMountsSelected =
    (showMountManagement ? pagedMounts : pagedLocalNodes).length > 0 &&
    (showMountManagement ? pagedMounts : pagedLocalNodes).every((item) => selectedMountIds.includes(item.id));

  useEffect(() => {
    setPageBySubPage((current) => ({
      ...current,
      mounts: Math.min(current.mounts, mountPageCount),
      nas: Math.min(current.nas, nasPageCount),
      cloud: Math.min(current.cloud, cloudPageCount),
    }));
  }, [cloudPageCount, mountPageCount, nasPageCount]);

  async function refreshDashboard() {
    setLoading(true);
    try {
      const next = await storageNodesApi.loadDashboard();
      setDashboard(next);
      setSelectedMountIds((current) =>
        current.filter((id) => next.mounts.some((item) => item.id === id) || next.localNodes.some((item) => item.id === id)),
      );
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '加载存储节点失败，请稍后重试'), tone: 'critical' });
    } finally {
      setLoading(false);
    }
  }

  function openCreate() {
    setMenuState(null);
    if (showMountManagement) {
      const defaultSourceType = mountSourceTypeOptions[0] ?? '本地';
      const defaultNodeId = mountSourceOptions.find((item) => item.sourceType === defaultSourceType)?.id ?? '';
      setMountForm({
        draft: { ...EMPTY_MOUNT_DRAFT, libraryId: libraries[0]?.id ?? '', folderType: defaultSourceType, nodeId: defaultNodeId },
        errors: {},
        saving: false,
      });
      return;
    }
    if (subPage === 'mounts') {
      setLocalNodeForm({ draft: { ...EMPTY_LOCAL_NODE_DRAFT }, errors: {}, saving: false });
      return;
    }
    if (subPage === 'nas') {
      setNasForm({ draft: { ...EMPTY_NAS_DRAFT }, errors: {}, saving: false });
      return;
    }
    setCloudForm({ draft: { ...EMPTY_CLOUD_DRAFT }, errors: {}, saving: false });
  }

  async function saveLocalNode() {
    if (!localNodeForm) return;
    const errors = validateLocalNodeDraft(localNodeForm.draft);
    if (Object.keys(errors).length > 0) {
      setLocalNodeForm((current) => (current ? { ...current, errors } : current));
      return;
    }
    setLocalNodeForm((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await storageNodesApi.saveLocalNode(localNodeForm.draft);
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setLocalNodeForm(null);
    } catch (error) {
      setLocalNodeForm((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '保存本地文件夹节点失败'), tone: 'critical' });
    }
  }

  async function saveMountFolder() {
    if (!mountForm) return;
    const errors = validateMountDraft(mountForm.draft);
    if (Object.keys(errors).length > 0) {
      setMountForm((current) => (current ? { ...current, errors } : current));
      return;
    }
    setMountForm((current) => (current ? { ...current, saving: true } : current));
    try {
      const selectedLibrary = libraries.find((library) => library.id === mountForm.draft.libraryId);
      const result = await storageNodesApi.saveMount({
        ...mountForm.draft,
        libraryName: selectedLibrary?.name ?? mountForm.draft.libraryId,
      });
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setMountForm(null);
    } catch (error) {
      setMountForm((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '保存本地文件夹失败'), tone: 'critical' });
    }
  }

  async function saveNasNode() {
    if (!nasForm) return;
    const errors = validateNasDraft(nasForm.draft);
    if (Object.keys(errors).length > 0) {
      setNasForm((current) => (current ? { ...current, errors } : current));
      return;
    }
    setNasForm((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await storageNodesApi.saveNasNode(nasForm.draft);
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setNasForm(null);
    } catch (error) {
      setNasForm((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '保存 NAS 失败'), tone: 'critical' });
    }
  }

  async function saveCloudNode() {
    if (!cloudForm) return;
    const errors = validateCloudDraft(cloudForm.draft);
    if (Object.keys(errors).length > 0) {
      setCloudForm((current) => (current ? { ...current, errors } : current));
      return;
    }
    setCloudForm((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await storageNodesApi.saveCloudNode(cloudForm.draft);
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setCloudForm(null);
    } catch (error) {
      setCloudForm((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '保存网盘失败'), tone: 'critical' });
    }
  }

  async function browseLocalFolder() {
    if (!mountForm && !localNodeForm) return;
    try {
      const result = await storageNodesApi.browseLocalFolder();
      if (result.path) {
        setMountForm((current) => (current ? { ...current, draft: { ...current.draft, relativePath: result.path! }, errors: {} } : current));
        setLocalNodeForm((current) => (current ? { ...current, draft: { ...current.draft, rootPath: result.path! }, errors: {} } : current));
      }
    } catch {
      setFeedback({ message: '打开文件夹选择器失败', tone: 'warning' });
    }
  }

  async function runMountScan(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要扫描的本地文件夹', tone: 'info' });
      return;
    }
    setScanningIds(ids);
    try {
      const result = await storageNodesApi.runMountScan(ids);
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'info' });
      setSelectedMountIds((current) => current.filter((id) => !ids.includes(id)));
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '扫描发起失败'), tone: 'critical' });
    } finally {
      setScanningIds([]);
    }
  }

  async function runMountConnectionTest(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要测试的本地文件夹', tone: 'info' });
      return;
    }
    setTestingIds(ids);
    try {
      const result = await storageNodesApi.runMountConnectionTest(ids);
      setConnectionResults(result.results);
      setFeedback({ message: result.message, tone: 'success' });
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '连接测试失败'), tone: 'critical' });
    } finally {
      setTestingIds([]);
    }
  }

  async function runLocalNodeConnectionTest(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要测试的本地文件夹节点', tone: 'info' });
      return;
    }
    setTestingIds(ids);
    try {
      const result = await storageNodesApi.runLocalNodeConnectionTest(ids);
      await refreshDashboard();
      setConnectionResults(result.results);
      setFeedback({ message: result.message, tone: 'success' });
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '本地文件夹节点连接测试失败'), tone: 'critical' });
    } finally {
      setTestingIds([]);
    }
  }

  async function runNasConnectionTest(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要测试的 NAS', tone: 'info' });
      return;
    }
    setTestingNasIds(ids);
    setMenuState(null);
    try {
      const result = await storageNodesApi.runNasConnectionTest(ids);
      await refreshDashboard();
      setConnectionResults(result.results);
      setFeedback({ message: result.message, tone: 'success' });
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, 'NAS 连接测试失败'), tone: 'critical' });
    } finally {
      setTestingNasIds([]);
    }
  }

  async function runCloudConnectionTest(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要测试的网盘', tone: 'info' });
      return;
    }
    setTestingCloudIds(ids);
    setMenuState(null);
    try {
      const result = await storageNodesApi.runCloudConnectionTest(ids);
      await refreshDashboard();
      setConnectionResults(result.results);
      setFeedback({ message: result.message, tone: 'success' });
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '网盘连接测试失败'), tone: 'critical' });
    } finally {
      setTestingCloudIds([]);
    }
  }

  async function openHistory(item: MountFolderRecord) {
    setHistoryState({ mountName: item.name, loading: true, items: [] });
    try {
      const result = await storageNodesApi.loadMountScanHistory(item.id);
      setHistoryState({ mountName: item.name, loading: false, items: result.items });
    } catch (error) {
      setHistoryState({ mountName: item.name, loading: false, items: [] });
      setFeedback({ message: extractErrorMessage(error, '加载扫描历史失败'), tone: 'critical' });
    }
  }

  async function saveHeartbeat() {
    if (!heartbeatEditor) return;
    setHeartbeatEditor((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await storageNodesApi.updateMountHeartbeat(heartbeatEditor.ids, heartbeatEditor.value);
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setHeartbeatEditor(null);
    } catch (error) {
      setHeartbeatEditor((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '更新心跳失败'), tone: 'critical' });
    }
  }

  async function deleteMountFolder(id: string) {
    const result = await storageNodesApi.deleteMountFolder(id);
    await refreshDashboard();
    setFeedback({ message: result.message, tone: 'success' });
  }

  async function deleteLocalNode(id: string) {
    const result = await storageNodesApi.deleteLocalNode(id);
    await refreshDashboard();
    setFeedback({ message: result.message, tone: 'success' });
  }

  async function deleteNasNode(id: string) {
    const result = await storageNodesApi.deleteNasNode(id);
    await refreshDashboard();
    setFeedback({ message: result.message, tone: 'success' });
  }

  async function deleteCloudNode(id: string) {
    const result = await storageNodesApi.deleteCloudNode(id);
    await refreshDashboard();
    setFeedback({ message: result.message, tone: 'success' });
  }

  return (
    <section className="page-stack storage-page">
      <div className="toolbar-card storage-header-tabs">
        <TabSwitch
          items={SUB_PAGES}
          value={showMountManagement ? '__mount-management__' : subPage}
          onChange={(value) => {
            setShowMountManagement(false);
            setSubPage(value as StorageSubPage);
          }}
        />
        <div className="tab-switch storage-mode-switch" role="group" aria-label="挂载模式切换">
          <button
            className={showMountManagement ? 'active' : ''}
            type="button"
            onClick={() => {
              setSubPage('mounts');
              setShowMountManagement(true);
            }}
          >
            挂载管理
          </button>
        </div>
      </div>

      <div className="toolbar-card action-toolbar storage-topbar">
        <div className="toolbar-group wrap storage-toolbar-main">
          {showMountManagement ? (
            <SelectPill
              ariaLabel="资产库筛选"
              options={mountLibraryOptions}
              value={mountLibraryFilter}
              onChange={setMountLibraryFilter}
            />
          ) : null}
          <SelectPill
            ariaLabel="状态筛选"
            options={STATUS_OPTIONS}
            value={statusFilter}
            onChange={(value) => setStatusFilter(value as StatusFilter)}
          />
          <label className="search-field">
            <input
              aria-label="搜索存储项"
              placeholder={
                showMountManagement
                  ? '搜索挂载名称、挂载路径、节点、资产库'
                  : subPage === 'mounts'
                    ? '搜索名称、根目录'
                    : subPage === 'nas'
                      ? '搜索名称、地址、账号'
                      : '搜索名称、目录、账号别名'
              }
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
        </div>
        <div className="toolbar-group wrap">
          <ActionButton onClick={openCreate}>
            <Plus size={14} />
            {showMountManagement
              ? '新增挂载'
              : subPage === 'mounts'
                ? '新增本地文件夹'
                : subPage === 'nas'
                  ? '新增 NAS'
                  : '新增网盘'}
          </ActionButton>
        </div>
      </div>

      {selectedMountIds.length > 0 ? (
        <div className="toolbar-card selection-toolbar">
          <span className="selection-caption">已选择 {selectedMountIds.length} 个{showMountManagement ? '挂载' : subPage === 'mounts' ? '本地文件夹节点' : subPage === 'nas' ? 'NAS 节点' : '网盘节点'}</span>
          <div className="toolbar-group wrap">
            {showMountManagement ? (
              <ActionButton ariaLabel="批量扫描" onClick={() => void runMountScan(selectedMountIds)}>
                <RefreshCw size={14} />
                批量扫描
              </ActionButton>
            ) : null}
            <ActionButton
              ariaLabel="批量连接测试"
              onClick={() =>
                void (showMountManagement
                  ? runMountConnectionTest(selectedMountIds)
                  : subPage === 'mounts'
                    ? runLocalNodeConnectionTest(selectedMountIds)
                    : subPage === 'nas'
                      ? runNasConnectionTest(selectedMountIds)
                      : runCloudConnectionTest(selectedMountIds))
              }
            >
              <Radar size={14} />
              批量连接测试
            </ActionButton>
            <ActionButton ariaLabel="清空选择" onClick={() => setSelectedMountIds([])}>
              清空选择
            </ActionButton>
          </div>
        </div>
      ) : null}

      <div className="workspace-card storage-table-card">
        {loading ? (
          <div className="empty-state">
            <LoaderCircle className="spin" size={18} />
            <strong>正在加载存储配置</strong>
            <p>正在读取本地文件夹、NAS 和网盘配置。</p>
          </div>
        ) : showMountManagement ? (
          <MountFoldersTable
            items={pagedMounts}
            menuState={menuState}
            allVisibleSelected={allVisibleMountsSelected}
            page={pageBySubPage.mounts}
            pageCount={mountPageCount}
            pageSize={pageSizeBySubPage.mounts}
            total={visibleMounts.length}
            onOpenHistory={openHistory}
            onOpenIssueCenter={onOpenIssueCenter}
            onOpenTaskCenter={onOpenTaskCenter}
            onDelete={deleteMountFolder}
            onEdit={(item) =>
              setMountForm({
                draft: mountRecordToDraft(item),
                errors: {},
                saving: false,
              })
            }
            onMenuChange={setMenuState}
            onRunConnectionTest={(id) => void runMountConnectionTest([id])}
            onRunScan={(id) => void runMountScan([id])}
            onSetHeartbeat={(id, value) => setHeartbeatEditor({ ids: [id], value, saving: false })}
            onToggleSelect={(id) =>
              setSelectedMountIds((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]))
            }
            onToggleSelectVisible={() =>
              setSelectedMountIds((current) =>
                allVisibleMountsSelected
                  ? current.filter((id) => !pagedMounts.some((item) => item.id === id))
                  : Array.from(new Set([...current, ...pagedMounts.map((item) => item.id)])),
              )
            }
            onPageChange={(value) => setPageBySubPage((current) => ({ ...current, mounts: value }))}
            onPageSizeChange={(value) =>
              setPageSizeBySubPage((current) => ({ ...current, mounts: value }))
            }
            scanningIds={scanningIds}
            selectedIds={selectedMountIds}
            testingIds={testingIds}
          />
        ) : subPage === 'mounts' ? (
          <LocalNodesTable
            items={pagedLocalNodes}
            menuState={menuState}
            page={pageBySubPage.mounts}
            pageCount={mountPageCount}
            pageSize={pageSizeBySubPage.mounts}
            total={visibleLocalNodes.length}
            onDelete={(id) => void deleteLocalNode(id)}
            onEdit={(item) => setLocalNodeForm({ draft: localNodeRecordToDraft(item), errors: {}, saving: false })}
            onMenuChange={setMenuState}
            onPageChange={(value) => setPageBySubPage((current) => ({ ...current, mounts: value }))}
            onPageSizeChange={(value) => setPageSizeBySubPage((current) => ({ ...current, mounts: value }))}
            onRunConnectionTest={(id) => void runLocalNodeConnectionTest([id])}
            selectedIds={selectedMountIds}
            testingIds={testingIds}
            onToggleSelect={(id) =>
              setSelectedMountIds((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]))
            }
            onToggleSelectVisible={() =>
              setSelectedMountIds((current) =>
                allVisibleMountsSelected
                  ? current.filter((id) => !pagedLocalNodes.some((item) => item.id === id))
                  : Array.from(new Set([...current, ...pagedLocalNodes.map((item) => item.id)])),
              )
            }
            allVisibleSelected={allVisibleMountsSelected}
          />
        ) : subPage === 'nas' ? (
          <NasTable
            items={pagedNas}
            menuState={menuState}
            page={pageBySubPage.nas}
            pageCount={nasPageCount}
            pageSize={pageSizeBySubPage.nas}
            total={visibleNas.length}
            onDelete={(id) => void deleteNasNode(id)}
            onEdit={(item) => setNasForm({ draft: nasRecordToDraft(item), errors: {}, saving: false })}
            onMenuChange={setMenuState}
            onPageChange={(value) => setPageBySubPage((current) => ({ ...current, nas: value }))}
            onPageSizeChange={(value) => setPageSizeBySubPage((current) => ({ ...current, nas: value }))}
            onRunConnectionTest={(id) => void runNasConnectionTest([id])}
            testingIds={testingNasIds}
          />
        ) : (
          <CloudTable
            items={pagedCloud}
            menuState={menuState}
            page={pageBySubPage.cloud}
            pageCount={cloudPageCount}
            pageSize={pageSizeBySubPage.cloud}
            total={visibleCloud.length}
            onDelete={(id) => void deleteCloudNode(id)}
            onEdit={(item) => setCloudForm({ draft: cloudRecordToDraft(item), errors: {}, saving: false })}
            onMenuChange={setMenuState}
            onPageChange={(value) => setPageBySubPage((current) => ({ ...current, cloud: value }))}
            onPageSizeChange={(value) => setPageSizeBySubPage((current) => ({ ...current, cloud: value }))}
            onRunConnectionTest={(id) => void runCloudConnectionTest([id])}
            testingIds={testingCloudIds}
          />
        )}
      </div>

      {localNodeForm ? (
        <Sheet onClose={() => setLocalNodeForm(null)} title={localNodeForm.draft.id ? '编辑本地文件夹节点' : '新增本地文件夹节点'}>
          <div className="sheet-section">
            <div className="sheet-form">
              <Field label="节点名称" error={localNodeForm.errors.name}>
                <input
                  aria-label="节点名称"
                  value={localNodeForm.draft.name}
                  onChange={(event) => setLocalNodeForm(updateLocalNodeForm(localNodeForm, { name: event.target.value }))}
                />
              </Field>
              <Field label="根目录" error={localNodeForm.errors.rootPath}>
                <div className="inline-action-row">
                  <input aria-label="根目录" value={localNodeForm.draft.rootPath} readOnly />
                  <ActionButton onClick={() => void browseLocalFolder()}>浏览目录</ActionButton>
                </div>
              </Field>
            </div>
          </div>
          <div className="sheet-actions right">
            <ActionButton tone="primary" onClick={() => void saveLocalNode()}>
              {localNodeForm.saving ? <LoaderCircle className="spin" size={14} /> : null}
              保存本地文件夹节点
            </ActionButton>
          </div>
        </Sheet>
      ) : null}

      {mountForm ? (
        <Sheet onClose={() => setMountForm(null)} title={mountForm.draft.id ? '编辑挂载' : '新增挂载'}>
          <div className="sheet-section">
            <div className="sheet-form">
              <Field label="挂载名称" error={mountForm.errors.name}>
                <input
                  aria-label="挂载名称"
                  value={mountForm.draft.name}
                  onChange={(event) => setMountForm(updateMountForm(mountForm, { name: event.target.value }))}
                />
              </Field>
              <div className="form-split">
                <Field label="节点类型">
                  <select
                    aria-label="节点类型"
                    value={mountForm.draft.folderType ?? '本地'}
                    onChange={(event) => {
                      const nextType = event.target.value as MountSourceOption['sourceType'];
                      const nextNodeId = mountSourceOptions.find((item) => item.sourceType === nextType)?.id ?? '';
                      setMountForm(updateMountForm(mountForm, { folderType: nextType, nodeId: nextNodeId }));
                    }}
                  >
                    {mountSourceTypeOptions.map((type) => (
                      <option key={type} value={type}>
                        {type}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="所属节点" error={mountForm.errors.nodeId}>
                  <select
                    aria-label="所属节点"
                    value={mountForm.draft.nodeId}
                    onChange={(event) => setMountForm(updateMountForm(mountForm, { nodeId: event.target.value }))}
                  >
                    <option value="">请选择节点</option>
                    {visibleMountSourceOptions.map((node) => (
                      <option key={node.id} value={node.id}>
                        {node.label}
                      </option>
                    ))}
                  </select>
                </Field>
              </div>
              <Field label="所属资产库" error={mountForm.errors.libraryId}>
                <select
                  aria-label="所属资产库"
                  value={mountForm.draft.libraryId}
                  onChange={(event) => setMountForm(updateMountForm(mountForm, { libraryId: event.target.value }))}
                >
                  {libraries.map((library) => (
                    <option key={library.id} value={library.id}>
                      {library.name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="挂载子目录" error={mountForm.errors.relativePath}>
                <input
                  aria-label="挂载子目录"
                  placeholder="例如：ShanghaiLaunch\\RAW"
                  value={mountForm.draft.relativePath}
                  onChange={(event) => setMountForm(updateMountForm(mountForm, { relativePath: event.target.value }))}
                />
              </Field>
              <Field label="挂载模式">
                <select
                  aria-label="挂载模式"
                  value={mountForm.draft.mountMode}
                  onChange={(event) => setMountForm(updateMountForm(mountForm, { mountMode: event.target.value as StorageMountMode }))}
                >
                  <option value="可写">可写</option>
                  <option value="只读">只读</option>
                </select>
              </Field>
              <Field label="心跳周期">
                <select
                  aria-label="心跳周期"
                  value={mountForm.draft.heartbeatPolicy}
                  onChange={(event) => setMountForm(updateMountForm(mountForm, { heartbeatPolicy: event.target.value as StorageHeartbeatPolicy }))}
                >
                  {HEARTBEAT_OPTIONS.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
          </div>
          <div className="sheet-actions right">
            <ActionButton tone="primary" onClick={() => void saveMountFolder()}>
              {mountForm.saving ? <LoaderCircle className="spin" size={14} /> : null}
              保存挂载
            </ActionButton>
          </div>
        </Sheet>
      ) : null}

      {nasForm ? (
        <Sheet onClose={() => setNasForm(null)} title={nasForm.draft.id ? '编辑 NAS' : '新增 NAS'}>
          <div className="sheet-section">
            <div className="sheet-form">
              <Field label="名称" error={nasForm.errors.name}>
                <input aria-label="NAS 名称" value={nasForm.draft.name} onChange={(event) => setNasForm(updateNasForm(nasForm, { name: event.target.value }))} />
              </Field>
              <Field label="地址" error={nasForm.errors.address}>
                <input aria-label="NAS 地址" placeholder="例如：\\\\192.168.10.20\\media" value={nasForm.draft.address} onChange={(event) => setNasForm(updateNasForm(nasForm, { address: event.target.value }))} />
              </Field>
              <Field label="账号" error={nasForm.errors.username}>
                <input aria-label="NAS 账号" value={nasForm.draft.username} onChange={(event) => setNasForm(updateNasForm(nasForm, { username: event.target.value }))} />
              </Field>
              <Field label="密码">
                <input aria-label="NAS 密码" type="password" value={nasForm.draft.password} onChange={(event) => setNasForm(updateNasForm(nasForm, { password: event.target.value }))} />
              </Field>
            </div>
          </div>
          <div className="sheet-actions right">
            <ActionButton tone="primary" onClick={() => void saveNasNode()}>
              {nasForm.saving ? <LoaderCircle className="spin" size={14} /> : null}
              保存 NAS
            </ActionButton>
          </div>
        </Sheet>
      ) : null}

      {cloudForm ? (
        <Sheet onClose={() => setCloudForm(null)} title={cloudForm.draft.id ? '编辑网盘' : '新增网盘'}>
          <div className="sheet-section">
            <div className="sheet-form">
              <Field label="名称" error={cloudForm.errors.name}>
                <input aria-label="网盘名称" value={cloudForm.draft.name} onChange={(event) => setCloudForm(updateCloudForm(cloudForm, { name: event.target.value }))} />
              </Field>
              <Field label="网盘类型">
                <select
                  aria-label="网盘类型"
                  value={cloudForm.draft.vendor}
                  onChange={(event) => setCloudForm(updateCloudForm(cloudForm, { vendor: event.target.value as '115' }))}
                >
                  <option value="115">115</option>
                </select>
              </Field>
              <Field label="接入方式">
                <div className="mini-segmented" role="group" aria-label="网盘接入方式">
                  {CLOUD_ACCESS_OPTIONS.map((option) => (
                    <button key={option} className={cloudForm.draft.accessMethod === option ? 'active' : ''} type="button" onClick={() => setCloudForm(updateCloudForm(cloudForm, { accessMethod: option }))}>
                      {option}
                    </button>
                  ))}
                </div>
              </Field>
              {cloudForm.draft.tokenStatus || cloudForm.draft.accountAlias || cloudForm.draft.lastAuthAt || cloudForm.draft.lastAuthResult || cloudForm.draft.lastErrorCode || cloudForm.draft.lastErrorMessage ? (
                <div className="dialog-card page-stack">
                  <strong>当前凭据状态</strong>
                  {cloudForm.draft.tokenStatus ? <DenseRow label="Token 状态" value={cloudForm.draft.tokenStatus} /> : null}
                  {cloudForm.draft.accountAlias ? <DenseRow label="账号别名" value={cloudForm.draft.accountAlias} /> : null}
                  {cloudForm.draft.lastAuthAt ? <DenseRow label="最近鉴权时间" value={cloudForm.draft.lastAuthAt} /> : null}
                  {cloudForm.draft.lastAuthResult ? <DenseRow label="最近鉴权结果" value={cloudForm.draft.lastAuthResult} /> : null}
                  {cloudForm.draft.lastErrorCode ? <DenseRow label="最近错误代码" tone="critical" value={cloudForm.draft.lastErrorCode} /> : null}
                  {cloudForm.draft.lastErrorMessage ? <DenseRow label="最近失败原因" tone="critical" value={cloudForm.draft.lastErrorMessage} /> : null}
                </div>
              ) : null}
              {cloudForm.draft.accessMethod === '扫码登录获取 Token' ? (
                <>
                  <Field label="扫码登录类型">
                    <select aria-label="扫码登录类型" value={cloudForm.draft.qrChannel} onChange={(event) => setCloudForm(updateCloudForm(cloudForm, { qrChannel: event.target.value as StorageCloudQrChannel }))}>
                      {CLOUD_QR_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="扫码登录二维码">
                    <CloudQrCodePreview
                      draft={cloudForm.draft}
                      onSessionChange={(session) =>
                        setCloudForm((current) =>
                          current ? { ...current, draft: { ...current.draft, qrSession: session } } : current,
                        )
                      }
                    />
                  </Field>
                </>
              ) : null}
              <Field label="挂载目录" error={cloudForm.errors.mountDirectory}>
                <input aria-label="挂载目录" placeholder="例如：/MareArchive" value={cloudForm.draft.mountDirectory} onChange={(event) => setCloudForm(updateCloudForm(cloudForm, { mountDirectory: event.target.value }))} />
              </Field>
              {cloudForm.draft.accessMethod === '填入 Token' ? (
                <Field label="Token" error={cloudForm.errors.token}>
                  <input aria-label="网盘 Token" value={cloudForm.draft.token} onChange={(event) => setCloudForm(updateCloudForm(cloudForm, { token: event.target.value }))} />
                </Field>
              ) : null}
            </div>
          </div>
          <div className="sheet-actions right">
            <ActionButton tone="primary" onClick={() => void saveCloudNode()}>
              {cloudForm.saving ? <LoaderCircle className="spin" size={14} /> : null}
              保存网盘
            </ActionButton>
          </div>
        </Sheet>
      ) : null}

      {heartbeatEditor ? (
        <Dialog title="设置心跳" onClose={() => setHeartbeatEditor(null)}>
          <Field label="心跳周期">
            <select aria-label="心跳周期设置" value={heartbeatEditor.value} onChange={(event) => setHeartbeatEditor((current) => (current ? { ...current, value: event.target.value as StorageHeartbeatPolicy } : current))}>
              {HEARTBEAT_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </Field>
          <div className="sheet-actions right">
            <ActionButton tone="primary" onClick={() => void saveHeartbeat()}>
              {heartbeatEditor.saving ? <LoaderCircle className="spin" size={14} /> : null}
              保存心跳
            </ActionButton>
          </div>
        </Dialog>
      ) : null}

      {historyState ? (
        <Dialog title="扫描历史" onClose={() => setHistoryState(null)}>
          {historyState.loading ? (
            <div className="empty-state">
              <LoaderCircle className="spin" size={18} />
              <strong>正在加载扫描历史</strong>
            </div>
          ) : (
            <div className="storage-dialog-stack">
              {historyState.items.map((item) => (
                <section className="dialog-card" key={item.id}>
                  <div className="section-header">
                    <strong>{item.startedAt}</strong>
                    <TonePill tone={item.status === '成功' ? 'success' : item.status === '失败' ? 'critical' : 'info'}>{item.status}</TonePill>
                  </div>
                  <p className="muted-paragraph">{item.summary}</p>
                </section>
              ))}
            </div>
          )}
        </Dialog>
      ) : null}

      {connectionResults ? (
        <Dialog title="连接测试结果" onClose={() => setConnectionResults(null)}>
          <div className="storage-dialog-stack">
            {connectionResults.map((item) => (
              <section className="dialog-card" key={item.id}>
                <div className="section-header">
                  <strong>{item.name}</strong>
                  <TonePill tone={item.overallTone}>{item.overallTone === 'success' ? '可继续使用' : '建议处理后再继续'}</TonePill>
                </div>
                <p className="muted-paragraph">{item.summary}</p>
                <div className="dense-result-list">
                  {item.checks.map((check) => (
                    <div className="dense-result-row" key={`${item.id}-${check.label}`}>
                      <span>{check.label}</span>
                      <strong className={`tone-text-${check.status}`}>{check.detail}</strong>
                    </div>
                  ))}
                </div>
              </section>
            ))}
          </div>
        </Dialog>
      ) : null}
    </section>
  );
}

function MountFoldersTable({
  allVisibleSelected,
  items,
  menuState,
  page,
  pageCount,
  pageSize,
  onDelete,
  onEdit,
  onMenuChange,
  onOpenHistory,
  onOpenIssueCenter,
  onOpenTaskCenter,
  onRunConnectionTest,
  onRunScan,
  onSetHeartbeat,
  onToggleSelect,
  onToggleSelectVisible,
  onPageChange,
  onPageSizeChange,
  scanningIds,
  selectedIds,
  testingIds,
  total,
}: {
  allVisibleSelected: boolean;
  items: MountFolderRecord[];
  menuState: StorageMenuState;
  page: number;
  pageCount: number;
  pageSize: StoragePageSize;
  onDelete: (id: string) => void | Promise<void>;
  onEdit: (item: MountFolderRecord) => void;
  onMenuChange: (value: StorageMenuState) => void;
  onOpenHistory: (item: MountFolderRecord) => void;
  onOpenIssueCenter?: (context: { id: string; label: string; path?: string }) => void;
  onOpenTaskCenter?: (id: string) => void;
  onRunConnectionTest: (id: string) => void;
  onRunScan: (id: string) => void;
  onSetHeartbeat: (id: string, value: StorageHeartbeatPolicy) => void;
  onToggleSelect: (id: string) => void;
  onToggleSelectVisible: () => void;
  onPageChange: (value: number) => void;
  onPageSizeChange: (value: StoragePageSize) => void;
  scanningIds: string[];
  selectedIds: string[];
  testingIds: string[];
  total: number;
}) {
  if (items.length === 0) {
    return <EmptyState title="没有匹配的挂载" description="可以调整筛选条件，或新增一个挂载。" />;
  }

  return (
    <>
    <div className="storage-table-wrap">
      <table className="file-table storage-table">
        <thead>
          <tr>
            <th className="checkbox-cell">
              <input aria-label="选择当前挂载" checked={allVisibleSelected} type="checkbox" onChange={onToggleSelectVisible} />
            </th>
            <th>名称</th>
            <th>路径</th>
            <th>扫描状态</th>
            <th>最近扫描</th>
            <th>心跳周期</th>
            <th>容量 / 可用空间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.id} aria-selected={selectedIds.includes(item.id)}>
              <td className="checkbox-cell">
                <input aria-label={`选择挂载 ${item.name}`} checked={selectedIds.includes(item.id)} type="checkbox" onChange={() => onToggleSelect(item.id)} />
              </td>
              <td>
                <div className="storage-node-cell">
                  <div className="storage-node-title">
                    <strong>{item.name}</strong>
                    <TonePill tone={resolveMountStatusTone(item)}>{resolveMountStatus(item)}</TonePill>
                  </div>
                  <div className="endpoint-row">
                    {item.badges.map((badge) => (
                      <TonePill key={badge} tone="info">
                        {badge}
                      </TonePill>
                    ))}
                    <TonePill tone="warning">{item.libraryName}</TonePill>
                    {item.riskTags.map((tag) => (
                      <TonePill key={tag} tone="warning">
                        {tag}
                      </TonePill>
                    ))}
                  </div>
                </div>
              </td>
              <td>
                <div className="row-main">
                  <strong>{item.address}</strong>
                  <span>本地目录</span>
                </div>
              </td>
              <td>
                <div className="row-main">
                  <TonePill tone={item.scanTone}>{item.scanStatus}</TonePill>
                </div>
              </td>
              <td>{item.lastScanAt}</td>
              <td>
                <div className="row-main">
                  <strong>{item.heartbeatPolicy}</strong>
                  <span>{item.nextHeartbeatAt}</span>
                </div>
              </td>
              <td>
                <div className="storage-capacity-cell">
                  <strong>{item.capacitySummary}</strong>
                  <ProgressBar value={item.capacityPercent} />
                  <span>{item.freeSpaceSummary}</span>
                </div>
              </td>
              <td>
                <div className="row-actions storage-row-actions">
                  <IconButton ariaLabel={`连接测试 ${item.name}`} tooltip="连接测试" onClick={() => onRunConnectionTest(item.id)}>
                    {testingIds.includes(item.id) ? <LoaderCircle className="spin" size={15} /> : <Radar size={15} />}
                  </IconButton>
                  <IconButton ariaLabel={`立即扫描 ${item.name}`} tooltip="立即扫描" onClick={() => onRunScan(item.id)}>
                    {scanningIds.includes(item.id) ? <LoaderCircle className="spin" size={15} /> : <RefreshCw size={15} />}
                  </IconButton>
                  <IconButton ariaLabel={`编辑 ${item.name}`} tooltip="编辑" onClick={() => onEdit(item)}>
                    <Pencil size={15} />
                  </IconButton>
                  <div className="storage-menu-anchor">
                    <IconButton
                      ariaLabel={`更多操作 ${item.name}`}
                      tooltip="更多操作"
                      onClick={(event) => {
                        const rect = event.currentTarget.getBoundingClientRect();
                        onMenuChange(
                          menuState?.id === item.id
                            ? null
                            : {
                                type: 'mount',
                                id: item.id,
                                top: resolveFloatingMenuTop(rect, ROW_MENU_ESTIMATED_HEIGHT),
                                right: Math.max(12, window.innerWidth - rect.right),
                              },
                        );
                      }}
                    >
                      <CircleEllipsis size={15} />
                    </IconButton>
                    {menuState?.type === 'mount' && menuState.id === item.id
                      ? createPortal(
                      <div className="context-menu storage-menu-inline" style={{ position: 'fixed', top: menuState.top, right: menuState.right }}>
                        <button type="button" onClick={() => onSetHeartbeat(item.id, item.heartbeatPolicy)}>
                          设置心跳
                        </button>
                        <button type="button" onClick={() => onOpenHistory(item)}>
                          查看扫描历史
                        </button>
                        <button type="button" onClick={() => onOpenTaskCenter?.(item.id)}>
                          查看相关任务
                        </button>
                        <button
                          type="button"
                          onClick={() => onOpenIssueCenter?.({ id: item.id, label: item.name, path: item.address })}
                        >
                          查看相关异常
                        </button>
                        <button className="danger-text" type="button" onClick={() => void onDelete(item.id)}>
                          删除
                        </button>
                      </div>,
                      document.body,
                    ) : null}
                  </div>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
    <StoragePagination
      page={page}
      pageCount={pageCount}
      pageSize={pageSize}
      total={total}
      onPageChange={onPageChange}
      onPageSizeChange={onPageSizeChange}
    />
    </>
  );
}

function LocalNodesTable({
  allVisibleSelected,
  items,
  menuState,
  page,
  pageCount,
  pageSize,
  onDelete,
  onEdit,
  onMenuChange,
  onPageChange,
  onPageSizeChange,
  onRunConnectionTest,
  onToggleSelect,
  onToggleSelectVisible,
  selectedIds,
  testingIds,
  total,
}: {
  allVisibleSelected: boolean;
  items: LocalNodeRecord[];
  menuState: StorageMenuState;
  page: number;
  pageCount: number;
  pageSize: StoragePageSize;
  onDelete: (id: string) => void | Promise<void>;
  onEdit: (item: LocalNodeRecord) => void;
  onMenuChange: (value: StorageMenuState) => void;
  onPageChange: (value: number) => void;
  onPageSizeChange: (value: StoragePageSize) => void;
  onRunConnectionTest: (id: string) => void;
  onToggleSelect: (id: string) => void;
  onToggleSelectVisible: () => void;
  selectedIds: string[];
  testingIds: string[];
  total: number;
}) {
  if (items.length === 0) {
    return <EmptyState title="没有匹配的本地文件夹节点" description="可以调整筛选条件，或新增一个本地文件夹节点。" />;
  }

  return (
    <>
      <div className="storage-table-wrap">
        <table className="file-table storage-table">
          <thead>
            <tr>
              <th className="checkbox-cell">
                <input aria-label="选择当前本地文件夹节点" checked={allVisibleSelected} type="checkbox" onChange={onToggleSelectVisible} />
              </th>
              <th>名称</th>
              <th>根目录</th>
              <th>状态</th>
              <th>最近检测</th>
              <th>挂载数量</th>
              <th>容量 / 可用空间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} aria-selected={selectedIds.includes(item.id)}>
                <td className="checkbox-cell">
                  <input aria-label={`选择本地文件夹节点 ${item.name}`} checked={selectedIds.includes(item.id)} type="checkbox" onChange={() => onToggleSelect(item.id)} />
                </td>
                <td>
                  <div className="storage-node-cell">
                    <div className="storage-node-title">
                      <strong>{item.name}</strong>
                      <TonePill tone={item.healthTone}>{item.healthStatus}</TonePill>
                    </div>
                  </div>
                </td>
                <td>
                  <div className="row-main">
                    <strong>{item.rootPath}</strong>
                    <span>本地根目录</span>
                  </div>
                </td>
                <td>{item.healthStatus}</td>
                <td>{item.lastCheckAt}</td>
                <td>{item.mountCount}</td>
                <td>
                  <div className="storage-capacity-cell">
                    <strong>{item.capacitySummary}</strong>
                    <ProgressBar value={item.capacityPercent} />
                    <span>{item.freeSpaceSummary}</span>
                  </div>
                </td>
                <td>
                  <div className="row-actions storage-row-actions">
                    <IconButton ariaLabel={`连接测试 ${item.name}`} tooltip="连接测试" onClick={() => onRunConnectionTest(item.id)}>
                      {testingIds.includes(item.id) ? <LoaderCircle className="spin" size={15} /> : <Radar size={15} />}
                    </IconButton>
                    <IconButton ariaLabel={`编辑 ${item.name}`} tooltip="编辑" onClick={() => onEdit(item)}>
                      <Pencil size={15} />
                    </IconButton>
                    <div className="storage-menu-anchor">
                      <IconButton
                        ariaLabel={`更多操作 ${item.name}`}
                        tooltip="更多操作"
                        onClick={(event) => {
                          const rect = event.currentTarget.getBoundingClientRect();
                          onMenuChange(
                            menuState?.id === item.id
                              ? null
                              : {
                                  type: 'mount',
                                  id: item.id,
                                  top: resolveFloatingMenuTop(rect, ROW_MENU_ESTIMATED_HEIGHT),
                                  right: Math.max(12, window.innerWidth - rect.right),
                                },
                          );
                        }}
                      >
                        <CircleEllipsis size={15} />
                      </IconButton>
                      {menuState?.type === 'mount' && menuState.id === item.id
                        ? createPortal(
                            <div className="context-menu storage-menu-inline" style={{ position: 'fixed', top: menuState.top, right: menuState.right }}>
                              <button className="danger-text" type="button" onClick={() => void onDelete(item.id)}>
                                删除
                              </button>
                            </div>,
                            document.body,
                          )
                        : null}
                    </div>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <StoragePagination
        page={page}
        pageCount={pageCount}
        pageSize={pageSize}
        total={total}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />
    </>
  );
}

function NasTable({
  items,
  menuState,
  page,
  pageCount,
  pageSize,
  onDelete,
  onEdit,
  onMenuChange,
  onPageChange,
  onPageSizeChange,
  onRunConnectionTest,
  testingIds,
  total,
}: {
  items: NasRecord[];
  menuState: StorageMenuState;
  page: number;
  pageCount: number;
  pageSize: StoragePageSize;
  onDelete: (id: string) => void;
  onEdit: (item: NasRecord) => void;
  onMenuChange: (value: StorageMenuState) => void;
  onPageChange: (value: number) => void;
  onPageSizeChange: (value: StoragePageSize) => void;
  onRunConnectionTest: (id: string) => void;
  testingIds: string[];
  total: number;
}) {
  if (items.length === 0) {
    return <EmptyState title="没有 NAS 配置" description="可以新增一个 NAS 主机配置。" />;
  }

  return (
    <>
    <div className="storage-table-wrap">
      <table className="file-table storage-table storage-simple-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>地址</th>
            <th>账号密码</th>
            <th>鉴权状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.id}>
              <td>{item.name}</td>
              <td>
                <div className="row-main">
                  <strong>{item.address}</strong>
                  <span>{item.accessMode} · 已关联 {item.mountCount} 个挂载</span>
                </div>
              </td>
              <td>
                <div className="row-main">
                  <strong>{item.username}</strong>
                  <span>{item.passwordHint}</span>
                </div>
              </td>
              <td>
                <div className="row-main">
                  <TonePill tone={item.tone}>{item.status}</TonePill>
                  <span>{item.lastTestAt ?? '尚未测试'}</span>
                </div>
              </td>
              <td>
                <div className="row-actions storage-row-actions">
                  <IconButton ariaLabel={`连接测试 ${item.name}`} tooltip="连接测试" onClick={() => onRunConnectionTest(item.id)}>
                    {testingIds.includes(item.id) ? <LoaderCircle className="spin" size={15} /> : <Radar size={15} />}
                  </IconButton>
                  <IconButton ariaLabel={`编辑 ${item.name}`} tooltip="编辑" onClick={() => onEdit(item)}>
                    <Pencil size={15} />
                  </IconButton>
                  <div className="storage-menu-anchor">
                    <IconButton
                      ariaLabel={`更多操作 ${item.name}`}
                      tooltip="更多操作"
                      onClick={(event) => {
                        const rect = event.currentTarget.getBoundingClientRect();
                        onMenuChange(
                          menuState?.id === item.id
                            ? null
                            : {
                                type: 'nas',
                                id: item.id,
                                top: resolveFloatingMenuTop(rect, ROW_MENU_ESTIMATED_HEIGHT),
                                right: Math.max(12, window.innerWidth - rect.right),
                              },
                        );
                      }}
                    >
                      <CircleEllipsis size={15} />
                    </IconButton>
                    {menuState?.type === 'nas' && menuState.id === item.id
                      ? createPortal(
                      <div className="context-menu storage-menu-inline" style={{ position: 'fixed', top: menuState.top, right: menuState.right }}>
                        <button className="danger-text" type="button" onClick={() => onDelete(item.id)}>
                          删除
                        </button>
                      </div>,
                      document.body,
                    ) : null}
                  </div>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
    <StoragePagination
      page={page}
      pageCount={pageCount}
      pageSize={pageSize}
      total={total}
      onPageChange={onPageChange}
      onPageSizeChange={onPageSizeChange}
    />
    </>
  );
}

function CloudTable({
  items,
  menuState,
  page,
  pageCount,
  pageSize,
  onDelete,
  onEdit,
  onMenuChange,
  onPageChange,
  onPageSizeChange,
  onRunConnectionTest,
  testingIds,
  total,
}: {
  items: CloudRecord[];
  menuState: StorageMenuState;
  page: number;
  pageCount: number;
  pageSize: StoragePageSize;
  onDelete: (id: string) => void;
  onEdit: (item: CloudRecord) => void;
  onMenuChange: (value: StorageMenuState) => void;
  onPageChange: (value: number) => void;
  onPageSizeChange: (value: StoragePageSize) => void;
  onRunConnectionTest: (id: string) => void;
  testingIds: string[];
  total: number;
}) {
  if (items.length === 0) {
    return <EmptyState title="没有网盘配置" description="可以新增一个网盘接入配置。" />;
  }

  return (
    <>
      <div className="storage-table-wrap">
        <table className="file-table storage-table storage-simple-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>网盘类型</th>
              <th>接入方式</th>
              <th>挂载目录</th>
              <th>凭据治理</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{item.name}</td>
                <td>{item.vendor}</td>
                <td>{item.accessMethod}{item.qrChannel ? ` / ${item.qrChannel}` : ''}</td>
                <td>{item.mountDirectory}</td>
                <td>
                  <div className="page-stack">
                    <div className="row-main">
                      <TonePill tone={item.tone}>{item.status}</TonePill>
                      <span>{item.lastTestAt ?? '尚未测试'}</span>
                    </div>
                    <div className="endpoint-row">
                      <span>{item.tokenStatus}</span>
                      <span>{item.accountAlias ?? '未识别账号'}</span>
                    </div>
                    {item.lastAuthAt || item.lastAuthResult ? (
                      <div className="endpoint-row">
                        <span>{item.lastAuthAt ?? '尚无鉴权时间'}</span>
                        <span>{item.lastAuthResult ?? item.status}</span>
                      </div>
                    ) : null}
                    {item.lastErrorMessage ? <span className="selection-caption">{item.lastErrorMessage}</span> : null}
                  </div>
                </td>
                <td>
                  <div className="row-actions storage-row-actions">
                    <IconButton ariaLabel={`连接测试 ${item.name}`} tooltip="连接测试" onClick={() => onRunConnectionTest(item.id)}>
                      {testingIds.includes(item.id) ? <LoaderCircle className="spin" size={15} /> : <Radar size={15} />}
                    </IconButton>
                    <IconButton ariaLabel={`编辑 ${item.name}`} tooltip="编辑" onClick={() => onEdit(item)}>
                      <Pencil size={15} />
                    </IconButton>
                    <div className="storage-menu-anchor">
                      <IconButton
                        ariaLabel={`更多操作 ${item.name}`}
                        tooltip="更多操作"
                        onClick={(event) => {
                          const rect = event.currentTarget.getBoundingClientRect();
                          onMenuChange(
                            menuState?.id === item.id
                              ? null
                              : {
                                  type: 'cloud',
                                  id: item.id,
                                  top: resolveFloatingMenuTop(rect, ROW_MENU_ESTIMATED_HEIGHT),
                                  right: Math.max(12, window.innerWidth - rect.right),
                                },
                          );
                        }}
                      >
                        <CircleEllipsis size={15} />
                      </IconButton>
                      {menuState?.type === 'cloud' && menuState.id === item.id
                        ? createPortal(
                            <div className="context-menu storage-menu-inline" style={{ position: 'fixed', top: menuState.top, right: menuState.right }}>
                              <button className="danger-text" type="button" onClick={() => onDelete(item.id)}>
                                删除
                              </button>
                            </div>,
                            document.body,
                          )
                        : null}
                    </div>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <StoragePagination
        page={page}
        pageCount={pageCount}
        pageSize={pageSize}
        total={total}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />
    </>
  );
}

function StoragePagination({
  page,
  pageCount,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
}: {
  page: number;
  pageCount: number;
  pageSize: StoragePageSize;
  total: number;
  onPageChange: (value: number) => void;
  onPageSizeChange: (value: StoragePageSize) => void;
}) {
  return (
    <div className="storage-pagination">
      <span className="selection-caption">页 {page}/{pageCount}</span>
      <div className="storage-pagination-controls">
        <IconButton ariaLabel="首页" onClick={() => onPageChange(1)}>
          <ChevronLeft size={14} />
          <ChevronLeft size={14} />
        </IconButton>
        <IconButton ariaLabel="上一页" onClick={() => onPageChange(Math.max(1, page - 1))}>
          <ChevronLeft size={14} />
        </IconButton>
        <button className="storage-page-chip active" type="button">
          {page}
        </button>
        <IconButton ariaLabel="下一页" onClick={() => onPageChange(Math.min(pageCount, page + 1))}>
          <ChevronRight size={14} />
        </IconButton>
        <IconButton ariaLabel="末页" onClick={() => onPageChange(pageCount)}>
          <ChevronRight size={14} />
          <ChevronRight size={14} />
        </IconButton>
      </div>
      <label className="select-pill storage-page-size">
        <select aria-label="每页数量" value={String(pageSize)} onChange={(event) => onPageSizeChange(Number(event.target.value) as StoragePageSize)}>
          {PAGE_SIZE_OPTIONS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </label>
      <span className="selection-caption">项/页，共 {total} 项</span>
    </div>
  );
}

function Field({ children, error, label }: { children: React.ReactNode; error?: string; label: string }) {
  return (
    <label className="form-field">
      <span>{label}</span>
      {children}
      {error ? <small className="field-error">{error}</small> : null}
    </label>
  );
}

function Dialog({
  children,
  onClose,
  title,
}: {
  children: React.ReactNode;
  onClose: () => void;
  title: string;
}) {
  return (
    <div className="dialog-backdrop" role="presentation" onClick={onClose}>
      <section aria-label={title} className="dialog-panel" role="dialog" onClick={(event) => event.stopPropagation()}>
        <div className="sheet-header">
          <strong>{title}</strong>
          <IconButton ariaLabel="关闭" onClick={onClose}>
            <CircleDashed size={15} />
          </IconButton>
        </div>
        {children}
      </section>
    </div>
  );
}

function CloudQrCodePreview({
  draft,
  onSessionChange,
}: {
  draft: CloudDraft;
  onSessionChange: (session: import('../lib/storageNodesApi').CloudQRCodeSession | undefined) => void;
}) {
  const [qrSession, setQrSession] = useState<import('../lib/storageNodesApi').CloudQRCodeSession | null>(draft.qrSession ?? null);
  const [qrStatus, setQrStatus] = useState<string>('等待扫码');
  const [qrImageSrc, setQrImageSrc] = useState('');
  const [qrImageError, setQrImageError] = useState(false);
  const [refreshToken, setRefreshToken] = useState(0);
  const latestSessionCallback = useRef(onSessionChange);
  const requestKey = `${draft.qrChannel}:${refreshToken}`;

  useEffect(() => {
    latestSessionCallback.current = onSessionChange;
  }, [onSessionChange]);

  useEffect(() => {
    let disposed = false;

    


    setQrStatus('等待扫码');

    void storageNodesApi.createCloudQrSession(draft.qrChannel).then(async (session) => {
      if (disposed) return;
      setQrSession(session);
      latestSessionCallback.current(session);
    }).catch(() => {
      if (!disposed) {
        setQrStatus('二维码获取失败');
        latestSessionCallback.current(undefined);
      }
    });

    return () => {
      disposed = true;
    };
  }, [draft.qrChannel, requestKey]);

  useEffect(() => {
    let disposed = false;

    if (!qrSession?.qrcode) {
      setQrImageSrc('');
      setQrImageError(false);
      return () => {
        disposed = true;
      };
    }

    setQrImageError(false);
    void QRCode.toDataURL(qrSession.qrcode, {
      width: 168,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: {
        dark: '#111827',
        light: '#FFFFFF',
      },
    })
      .then((value) => {
        if (disposed) {
          return;
        }
        setQrImageSrc(value);
      })
      .catch(() => {
        if (disposed) {
          return;
        }
        setQrImageSrc('');
        setQrImageError(true);
      });

    return () => {
      disposed = true;
    };
  }, [qrSession]);

  useEffect(() => {
    if (!qrSession) return;
    let disposed = false;
    let timer = 0;
    const poll = async () => {
      try {
        const status = await storageNodesApi.getCloudQrSessionStatus(qrSession);
        if (disposed) {
          return;
        }
        setQrStatus(status.message);
        if (status.status === 'CONFIRMED' || status.status === 'EXPIRED' || status.status === 'CANCELED') {
          return;
        }
      } catch {
        if (disposed) {
          return;
        }
        setQrStatus('二维码状态查询超时，正在重试');
      }

      timer = window.setTimeout(() => {
        void poll();
      }, 2000);
    };

    timer = window.setTimeout(() => {
      void poll();
    }, 2000);
    return () => {
      disposed = true;
      window.clearTimeout(timer);
    };
  }, [qrSession]);

  return (
    <div className="cloud-qr-preview">
      <div className="cloud-qr-visual">
        {qrImageSrc ? (
          <img alt="115 扫码登录二维码" className="cloud-qr-image" src={qrImageSrc} />
        ) : (
          <div className="cloud-qr-placeholder">{qrImageError ? '二维码渲染失败，请刷新重试' : '二维码生成中'}</div>
        )}
      </div>
      <div className="cloud-qr-copy">
        <button
          className="cloud-qr-refresh"
          type="button"
          onClick={() => {
            setQrSession(null);
            setQrImageSrc('');
            setQrImageError(false);
            latestSessionCallback.current(undefined);
            setQrStatus('正在刷新二维码');
            setRefreshToken((current) => current + 1);
          }}
        >
          刷新二维码
        </button>
        <strong>请使用 {draft.qrChannel} 扫码</strong>
        <span>{qrStatus}</span>
      </div>
    </div>
  );
}

function updateMountForm(current: MountFormState, patch: Partial<MountFolderDraft>): MountFormState {
  return { ...current, draft: { ...current.draft, ...patch }, errors: {} };
}

function updateLocalNodeForm(current: LocalNodeFormState, patch: Partial<LocalNodeDraft>): LocalNodeFormState {
  return { ...current, draft: { ...current.draft, ...patch }, errors: {} };
}

function updateNasForm(current: NasFormState, patch: Partial<NasDraft>): NasFormState {
  return { ...current, draft: { ...current.draft, ...patch }, errors: {} };
}

function updateCloudForm(current: CloudFormState, patch: Partial<CloudDraft>): CloudFormState {
  return { ...current, draft: { ...current.draft, ...patch }, errors: {} };
}

function mountRecordToDraft(item: MountFolderRecord): MountFolderDraft {
  return {
    id: item.id,
    name: item.name,
    libraryId: item.libraryId,
    nodeId: item.nodeId,
    mountMode: item.mountMode,
    heartbeatPolicy: item.heartbeatPolicy,
    relativePath: item.relativePath,
    notes: item.notes,
    folderType: item.folderType,
  };
}

function localNodeRecordToDraft(item: LocalNodeRecord): LocalNodeDraft {
  return {
    id: item.id,
    name: item.name,
    rootPath: item.rootPath,
    notes: item.notes,
  };
}

function nasRecordToDraft(item: NasRecord): NasDraft {
  return {
    id: item.id,
    name: item.name,
    address: item.address,
    username: item.username,
    password: '',
    notes: item.notes,
  };
}

function cloudRecordToDraft(item: CloudRecord): CloudDraft {
  return {
    id: item.id,
    name: item.name,
    vendor: '115',
    accessMethod: item.accessMethod,
    qrChannel: item.qrChannel ?? '微信小程序',
    mountDirectory: item.mountDirectory,
    token: item.token ?? '',
    tokenStatus: item.tokenStatus,
    accountAlias: item.accountAlias,
    lastAuthAt: item.lastAuthAt,
    lastAuthResult: item.lastAuthResult,
    lastErrorCode: item.lastErrorCode,
    lastErrorMessage: item.lastErrorMessage,
    lastTestAt: item.lastTestAt,
    status: item.status,
    tone: item.tone,
    notes: item.notes,
  };
}

function validateMountDraft(draft: MountFolderDraft) {
  const errors: Partial<Record<string, string>> = {};
  if (!draft.name.trim()) errors.name = '请输入挂载名称';
  if (!draft.nodeId.trim()) errors.nodeId = '请选择所属节点';
  if (!draft.libraryId.trim()) errors.libraryId = '请选择资产库';
  if (!draft.relativePath.trim()) errors.relativePath = '请输入挂载子目录';
  return errors;
}

function validateLocalNodeDraft(draft: LocalNodeDraft) {
  const errors: Partial<Record<string, string>> = {};
  if (!draft.name.trim()) errors.name = '请输入节点名称';
  if (!draft.rootPath.trim()) errors.rootPath = '请选择根目录';
  return errors;
}

function validateNasDraft(draft: NasDraft) {
  const errors: Partial<Record<string, string>> = {};
  if (!draft.name.trim()) errors.name = '请输入 NAS 名称';
  if (!draft.address.trim()) errors.address = '请输入 NAS 地址';
  if (!draft.username.trim()) errors.username = '请输入账号';
  return errors;
}

function validateCloudDraft(draft: CloudDraft) {
  const errors: Partial<Record<string, string>> = {};
  if (!draft.name.trim()) errors.name = '请输入网盘名称';
  if (!draft.mountDirectory.trim()) errors.mountDirectory = '请输入挂载目录';
  if (draft.accessMethod === '填入 Token' && !draft.token.trim()) errors.token = '请输入 Token';
  return errors;
}

function resolveMountStatus(item: MountFolderRecord): StatusFilter {
  if (!item.enabled) return '已停用';
  if (item.scanStatus === '扫描中') return '扫描中';
  if (item.riskTags.length > 0 || item.authTone === 'warning' || item.authTone === 'critical') return '异常';
  return '可用';
}

function resolveNodeStatus(item: LocalNodeRecord): StatusFilter {
  if (!item.enabled) return '已停用';
  if (item.healthTone === 'critical') return '异常';
  if (item.healthTone === 'warning') return '扫描中';
  return '可用';
}

function resolveMountStatusTone(item: MountFolderRecord): FeedbackTone {
  const status = resolveMountStatus(item);
  if (status === '异常') return 'critical';
  if (status === '扫描中') return 'warning';
  if (status === '已停用') return 'info';
  return 'success';
}

function extractErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

function resolveFloatingMenuTop(rect: DOMRect, menuHeight: number) {
  const preferredTop = rect.bottom + 8;
  const maxTop = Math.max(12, window.innerHeight - menuHeight - 12);
  return Math.min(preferredTop, maxTop);
}
