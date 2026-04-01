import { useEffect, useMemo, useState } from 'react';
import {
  CircleAlert,
  CircleCheckBig,
  CircleDashed,
  CircleEllipsis,
  HardDrive,
  LoaderCircle,
  Pencil,
  Plus,
  Radar,
  RefreshCw,
  ShieldCheck,
} from 'lucide-react';
import {
  ActionButton,
  EmptyState,
  FeedbackBanner,
  IconButton,
  ProgressBar,
  SelectPill,
  Sheet,
  TonePill,
} from '../components/Shared';
import type {
  StorageCloudAccessMethod,
  StorageCloudQrChannel,
  StorageConnectionTestResult,
  StorageCredentialDraft,
  StorageHeartbeatPolicy,
  StorageNodeDraft,
  StorageNodeRecord,
  StorageNodeType,
  StorageScanHistoryItem,
} from '../lib/storageNodesApi';
import { storageNodesApi } from '../lib/storageNodesApi';

type FeedbackState = {
  message: string;
  tone: 'success' | 'warning' | 'critical' | 'info';
};

type StatusFilter = '全部' | '可用' | '异常' | '已停用' | '扫描中';

type FormState = {
  mode: 'create' | 'edit';
  draft: StorageNodeDraft;
  errors: Partial<Record<string, string>>;
  formConnectionResult: StorageConnectionTestResult | null;
  testing: boolean;
  saving: boolean;
};

type HeartbeatState = {
  ids: string[];
  title: string;
  saving: boolean;
  value: StorageHeartbeatPolicy;
};

type HistoryState = {
  loading: boolean;
  nodeName: string;
  items: StorageScanHistoryItem[];
};

type MenuState = {
  nodeId: string;
};

const TYPE_OPTIONS: Array<'全部' | StorageNodeType> = ['全部', '本机磁盘', 'NAS/SMB', '网盘'];
const STATUS_OPTIONS: StatusFilter[] = ['全部', '可用', '异常', '已停用', '扫描中'];
const HEARTBEAT_OPTIONS: StorageHeartbeatPolicy[] = ['从不', '每周（深夜）', '每日（深夜）', '每小时'];
const CLOUD_ACCESS_OPTIONS: StorageCloudAccessMethod[] = ['填入 Token', '扫码登录获取 Token'];
const CLOUD_QR_CHANNEL_OPTIONS: StorageCloudQrChannel[] = ['微信小程序', '支付宝小程序', '电视端'];

const EMPTY_LOCAL_DRAFT: StorageNodeDraft = {
  name: '',
  nodeType: '本机磁盘',
  notes: '',
  mountMode: '可写',
  heartbeatPolicy: '从不',
  detail: {
    kind: 'local',
    rootPath: '',
  },
};

export function StorageNodesPage({
  onOpenIssueCenter,
  onOpenTaskCenter,
}: {
  onOpenIssueCenter?: (nodeId: string) => void;
  onOpenTaskCenter?: (nodeId: string) => void;
}) {
  const [loading, setLoading] = useState(true);
  const [nodes, setNodes] = useState<StorageNodeRecord[]>([]);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [typeFilter, setTypeFilter] = useState<'全部' | StorageNodeType>('全部');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('全部');
  const [searchText, setSearchText] = useState('');
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [formState, setFormState] = useState<FormState | null>(null);
  const [credentialState, setCredentialState] = useState<StorageCredentialDraft | null>(null);
  const [heartbeatState, setHeartbeatState] = useState<HeartbeatState | null>(null);
  const [connectionResults, setConnectionResults] = useState<StorageConnectionTestResult[] | null>(null);
  const [historyState, setHistoryState] = useState<HistoryState | null>(null);
  const [placeholderMessage, setPlaceholderMessage] = useState<string | null>(null);
  const [menuState, setMenuState] = useState<MenuState | null>(null);
  const [testingIds, setTestingIds] = useState<string[]>([]);
  const [scanningIds, setScanningIds] = useState<string[]>([]);
  const [bulkLoading, setBulkLoading] = useState<null | 'scan' | 'test' | 'enable' | 'disable'>(null);

  useEffect(() => {
    void refreshDashboard();
  }, []);

  useEffect(() => {
    if (!feedback) {
      return;
    }

    const timer = window.setTimeout(() => setFeedback(null), 3200);
    return () => window.clearTimeout(timer);
  }, [feedback]);

  const visibleNodes = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();

    return nodes.filter((node) => {
      const matchesType = typeFilter === '全部' ? true : node.nodeType === typeFilter;
      const matchesStatus = statusFilter === '全部' ? true : resolveStatusFilter(node) === statusFilter;
      const matchesSearch = keyword
        ? [
            node.name,
            node.address,
            node.notes,
            node.nodeType,
            node.libraryBindings.join(' '),
            node.riskTags.join(' '),
            node.authStatus,
          ]
            .join(' ')
            .toLowerCase()
            .includes(keyword)
        : true;

      return matchesType && matchesStatus && matchesSearch;
    });
  }, [nodes, searchText, statusFilter, typeFilter]);

  const selectedVisibleIds = visibleNodes.filter((node) => selectedIds.includes(node.id)).map((node) => node.id);
  const allVisibleSelected = visibleNodes.length > 0 && selectedVisibleIds.length === visibleNodes.length;

  async function refreshDashboard() {
    setLoading(true);
    try {
      const snapshot = await storageNodesApi.loadDashboard();
      setNodes(snapshot.nodes);
      setSelectedIds((current) => current.filter((id) => snapshot.nodes.some((node) => node.id === id)));
    } catch (error) {
      setFeedback({
        message: extractErrorMessage(error, '加载存储节点失败，请稍后重试'),
        tone: 'critical',
      });
    } finally {
      setLoading(false);
    }
  }

  function beginCreate() {
    setFormState({
      mode: 'create',
      draft: structuredClone(EMPTY_LOCAL_DRAFT),
      errors: {},
      formConnectionResult: null,
      testing: false,
      saving: false,
    });
  }

  function beginEdit(node: StorageNodeRecord) {
    setMenuState(null);
    setFormState({
      mode: 'edit',
      draft: createDraftFromNode(node),
      errors: {},
      formConnectionResult: null,
      testing: false,
      saving: false,
    });
  }

  function updateDraft(nextDraft: StorageNodeDraft) {
    setFormState((current) =>
      current
        ? {
            ...current,
            draft: nextDraft,
            errors: {},
          }
        : current,
    );
  }

  async function saveDraft() {
    if (!formState) {
      return;
    }

    const errors = validateDraft(formState.draft);
    if (Object.keys(errors).length > 0) {
      setFormState((current) => (current ? { ...current, errors } : current));
      return;
    }

    setFormState((current) => (current ? { ...current, saving: true } : current));

    try {
      const result = await storageNodesApi.saveNode({ draft: formState.draft });
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setFormState(null);
    } catch (error) {
      setFormState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '保存节点失败'), tone: 'critical' });
    }
  }

  async function testDraftConnection() {
    if (!formState) {
      return;
    }

    const errors = validateDraft(formState.draft);
    if (Object.keys(errors).length > 0) {
      setFormState((current) => (current ? { ...current, errors } : current));
      return;
    }

    setFormState((current) => (current ? { ...current, testing: true, formConnectionResult: null } : current));

    try {
      const result = buildDraftConnectionResult(formState.draft);
      await waitBriefly();
      setFormState((current) =>
        current
          ? {
              ...current,
              testing: false,
              formConnectionResult: result,
            }
          : current,
      );
    } catch (error) {
      setFormState((current) =>
        current
          ? {
              ...current,
              testing: false,
              formConnectionResult: {
                nodeId: current.draft.id ?? 'draft',
                nodeName: current.draft.name || '未命名节点',
                overallTone: 'critical',
                summary: extractErrorMessage(error, '连接测试失败'),
                checks: [],
                suggestion: '检查表单配置后重试',
                testedAt: '刚刚',
              },
            }
          : current,
      );
    }
  }

  function toggleSelection(id: string) {
    setSelectedIds((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]));
  }

  function toggleSelectVisible() {
    setSelectedIds((current) => {
      if (allVisibleSelected) {
        return current.filter((id) => !visibleNodes.some((node) => node.id === id));
      }

      return Array.from(new Set([...current, ...visibleNodes.map((node) => node.id)]));
    });
  }

  async function runScan(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要扫描的节点', tone: 'info' });
      return;
    }

    setBulkLoading(ids.length > 1 ? 'scan' : null);
    setScanningIds(ids);
    setMenuState(null);

    try {
      const result = await storageNodesApi.runScan({ ids });
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'info' });
      setSelectedIds((current) => current.filter((id) => !ids.includes(id)));
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '扫描发起失败'), tone: 'critical' });
    } finally {
      setScanningIds([]);
      setBulkLoading(null);
    }
  }

  async function runConnectionTest(ids: string[]) {
    if (ids.length === 0) {
      setFeedback({ message: '请先选择需要测试的节点', tone: 'info' });
      return;
    }

    setBulkLoading(ids.length > 1 ? 'test' : null);
    setTestingIds(ids);
    setMenuState(null);

    try {
      const result = await storageNodesApi.runConnectionTest({ ids });
      await refreshDashboard();
      setFeedback({
        message: result.message,
        tone: result.results.some((item) => item.overallTone === 'critical') ? 'warning' : 'success',
      });
      setConnectionResults(result.results);
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '连接测试失败'), tone: 'critical' });
    } finally {
      setTestingIds([]);
      setBulkLoading(null);
    }
  }

  async function updateEnabled(ids: string[], enabled: boolean) {
    if (ids.length === 0) {
      setFeedback({ message: enabled ? '请先选择需要启用的节点' : '请先选择需要停用的节点', tone: 'info' });
      return;
    }

    setBulkLoading(enabled ? 'enable' : 'disable');
    setMenuState(null);

    try {
      const result = await storageNodesApi.updateEnabled({ ids, enabled });
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setSelectedIds((current) => current.filter((id) => !ids.includes(id)));
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, enabled ? '启用失败' : '停用失败'), tone: 'critical' });
    } finally {
      setBulkLoading(null);
    }
  }

  function openHeartbeatEditor(ids: string[]) {
    const relatedNodes = nodes.filter((node) => ids.includes(node.id));
    if (relatedNodes.length === 0) {
      return;
    }

    setMenuState(null);
    setHeartbeatState({
      ids,
      title: ids.length > 1 ? `批量设置心跳（${ids.length} 个节点）` : '设置心跳',
      saving: false,
      value: relatedNodes[0]?.heartbeatPolicy ?? '从不',
    });
  }

  async function saveHeartbeat() {
    if (!heartbeatState) {
      return;
    }

    setHeartbeatState((current) => (current ? { ...current, saving: true } : current));

    try {
      const result = await storageNodesApi.updateHeartbeat({
        ids: heartbeatState.ids,
        heartbeatPolicy: heartbeatState.value,
      });
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setHeartbeatState(null);
      setSelectedIds((current) => current.filter((id) => !heartbeatState.ids.includes(id)));
    } catch (error) {
      setHeartbeatState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: extractErrorMessage(error, '心跳策略更新失败'), tone: 'critical' });
    }
  }

  async function openHistory(node: StorageNodeRecord) {
    setMenuState(null);
    setHistoryState({
      loading: true,
      nodeName: node.name,
      items: [],
    });

    try {
      const result = await storageNodesApi.loadScanHistory({ id: node.id });
      setHistoryState({
        loading: false,
        nodeName: node.name,
        items: result.items,
      });
    } catch (error) {
      setHistoryState({
        loading: false,
        nodeName: node.name,
        items: [],
      });
      setFeedback({ message: extractErrorMessage(error, '加载扫描历史失败'), tone: 'critical' });
    }
  }

  function openCredentialSheet(node: StorageNodeRecord) {
    if (node.detail.kind === 'local') {
      setPlaceholderMessage('本地节点无需鉴权配置，后续会在这里补充更多高级策略。');
      return;
    }

    setMenuState(null);
    setCredentialState(createCredentialDraft(node));
  }

  async function saveCredentialDraft() {
    if (!credentialState) {
      return;
    }

    if (credentialState.authMode !== '账号密码' && !credentialState.token.trim()) {
      setFeedback({ message: credentialState.authMode === '填入 Token' ? 'Token 不能为空' : '请先生成扫码登录会话', tone: 'warning' });
      return;
    }

    if (credentialState.authMode === '账号密码' && !credentialState.username.trim()) {
      setFeedback({ message: '用户名不能为空', tone: 'warning' });
      return;
    }

    try {
      const result = await storageNodesApi.saveCredentials(credentialState);
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
      setCredentialState(null);
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '保存鉴权失败'), tone: 'critical' });
    }
  }

  async function removeNode(node: StorageNodeRecord) {
    setMenuState(null);

    try {
      const result = await storageNodesApi.deleteNode({ id: node.id });
      await refreshDashboard();
      setFeedback({ message: result.message, tone: 'success' });
    } catch (error) {
      setFeedback({ message: extractErrorMessage(error, '删除节点失败'), tone: 'critical' });
    }
  }

  function openRelatedTasks(nodeId: string) {
    setMenuState(null);
    if (onOpenTaskCenter) {
      onOpenTaskCenter(nodeId);
      setFeedback({ message: '已切换到任务中心，可继续查看与该节点相关的任务', tone: 'info' });
      return;
    }

    setPlaceholderMessage('任务中心联动接口已预留，接入真实后端后可直接按节点过滤跳转。');
  }

  function openRelatedIssues(nodeId: string) {
    setMenuState(null);
    if (onOpenIssueCenter) {
      onOpenIssueCenter(nodeId);
      setFeedback({ message: '已切换到异常中心，可继续查看与该节点相关的异常', tone: 'info' });
      return;
    }

    setPlaceholderMessage('异常中心联动接口已预留，接入真实后端后可直接按节点过滤跳转。');
  }

  return (
    <section className="page-stack storage-page">
      {feedback ? <FeedbackBanner message={feedback.message} tone={feedback.tone} /> : null}

      <div className="toolbar-card storage-topbar">
        <div className="toolbar-group wrap storage-toolbar-main">
          <SelectPill ariaLabel="按类型筛选" options={TYPE_OPTIONS} value={typeFilter} onChange={(value) => setTypeFilter(value as '全部' | StorageNodeType)} />
          <SelectPill ariaLabel="按状态筛选" options={STATUS_OPTIONS} value={statusFilter} onChange={(value) => setStatusFilter(value as StatusFilter)} />
          <label className="search-field">
            <input
              aria-label="搜索节点"
              placeholder="搜索名称、路径、资产库或风险"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
        </div>
      </div>

      <div className="toolbar-card action-toolbar">
        <div className="toolbar-group wrap">
          <span className="selection-caption">{nodes.length} 个节点</span>
          <span className="selection-caption">{nodes.filter((node) => resolveStatusFilter(node) === '异常').length} 个异常风险</span>
        </div>
        <div className="toolbar-group wrap">
          <ActionButton onClick={beginCreate}>
            <Plus size={14} />
            新增节点
          </ActionButton>
          <ActionButton ariaLabel="批量扫描" onClick={() => void runScan(selectedIds)}>
            {bulkLoading === 'scan' ? <LoaderCircle className="spin" size={14} /> : <RefreshCw size={14} />}
            批量扫描
          </ActionButton>
          <ActionButton ariaLabel="批量连接测试" onClick={() => void runConnectionTest(selectedIds)}>
            {bulkLoading === 'test' ? <LoaderCircle className="spin" size={14} /> : <Radar size={14} />}
            批量连接测试
          </ActionButton>
        </div>
      </div>

      {selectedIds.length > 0 ? (
        <div className="toolbar-card selection-toolbar">
          <span className="selection-caption">已选择 {selectedIds.length} 个节点</span>
          <div className="toolbar-group wrap">
            <ActionButton ariaLabel="批量扫描" onClick={() => void runScan(selectedIds)}>
              <RefreshCw size={14} />
              批量扫描
            </ActionButton>
            <ActionButton ariaLabel="批量连接测试" onClick={() => void runConnectionTest(selectedIds)}>
              <Radar size={14} />
              批量连接测试
            </ActionButton>
            <ActionButton ariaLabel="批量启用" onClick={() => void updateEnabled(selectedIds, true)}>
              {bulkLoading === 'enable' ? <LoaderCircle className="spin" size={14} /> : <CircleCheckBig size={14} />}
              批量启用
            </ActionButton>
            <ActionButton ariaLabel="批量停用" onClick={() => void updateEnabled(selectedIds, false)}>
              {bulkLoading === 'disable' ? <LoaderCircle className="spin" size={14} /> : <CircleAlert size={14} />}
              批量停用
            </ActionButton>
            <ActionButton ariaLabel="批量设置心跳" onClick={() => openHeartbeatEditor(selectedIds)}>
              <HardDrive size={14} />
              批量设置心跳
            </ActionButton>
            <ActionButton ariaLabel="清空选择" onClick={() => setSelectedIds([])}>
              清空选择
            </ActionButton>
          </div>
        </div>
      ) : null}

      <div className="workspace-card storage-table-card">
        {loading ? (
          <div className="empty-state">
            <LoaderCircle className="spin" size={18} />
            <strong>正在加载存储节点</strong>
            <p>正在从当前数据源读取节点信息与最近状态。</p>
          </div>
        ) : visibleNodes.length === 0 ? (
          <EmptyState
            title="没有匹配的存储节点"
            description="可以调整筛选条件，或直接新增一个节点配置。"
            action={
              <ActionButton onClick={beginCreate}>
                <Plus size={14} />
                新增节点
              </ActionButton>
            }
          />
        ) : (
          <div className="storage-table-wrap">
            <table className="file-table storage-table">
              <thead>
                <tr>
                  <th className="checkbox-cell">
                    <input
                      aria-label="选择当前筛选结果"
                      checked={allVisibleSelected}
                      type="checkbox"
                      onChange={toggleSelectVisible}
                    />
                  </th>
                  <th>节点名称</th>
                  <th>节点类型</th>
                  <th>地址 / 路径</th>
                  <th>扫描状态</th>
                  <th>最近扫描</th>
                  <th>心跳周期</th>
                  <th>容量 / 可用空间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleNodes.map((node) => {
                  const rowSelected = selectedIds.includes(node.id);
                  const operationalStatus = resolveStatusFilter(node);
                  const connectionLoading = testingIds.includes(node.id);
                  const scanLoading = scanningIds.includes(node.id);

                  return (
                    <tr key={node.id} aria-selected={rowSelected}>
                      <td className="checkbox-cell">
                        <input
                          aria-label={`选择节点 ${node.name}`}
                          checked={rowSelected}
                          type="checkbox"
                          onChange={() => toggleSelection(node.id)}
                        />
                      </td>
                      <td>
                        <div className="storage-node-cell">
                          <div className="storage-node-title">
                            <strong>{node.name}</strong>
                            <TonePill tone={resolveToneByStatus(operationalStatus)}>{operationalStatus}</TonePill>
                          </div>
                          <div className="endpoint-row">
                            {node.badges.map((badge) => (
                              <TonePill key={badge} tone={badge === '只读' ? 'warning' : 'info'}>
                                {badge}
                              </TonePill>
                            ))}
                            {node.riskTags.map((tag) => (
                              <TonePill key={tag} tone="warning">
                                {tag}
                              </TonePill>
                            ))}
                          </div>
                          <span>{node.authStatus}</span>
                        </div>
                      </td>
                      <td>{node.nodeType}</td>
                      <td>
                        <div className="row-main">
                          <strong>{node.address}</strong>
                          <span>{node.libraryBindings.length > 0 ? `已绑定 ${node.libraryBindings.length} 个资产库` : '暂未绑定资产库'}</span>
                        </div>
                      </td>
                      <td>
                        <div className="row-main">
                          <TonePill tone={node.scanTone}>{node.scanStatus}</TonePill>
                          <span>{node.enabled ? '允许扫描' : '已停用，不参与扫描'}</span>
                        </div>
                      </td>
                      <td>{node.lastScanAt}</td>
                      <td>
                        <div className="row-main">
                          <strong>{node.heartbeatPolicy}</strong>
                          <span>{node.nextHeartbeatAt}</span>
                        </div>
                      </td>
                      <td>
                        <div className="storage-capacity-cell">
                          <strong>{node.capacitySummary}</strong>
                          <ProgressBar value={node.capacityPercent} />
                          <span>{node.freeSpaceSummary}</span>
                        </div>
                      </td>
                      <td>
                        <div className="row-actions storage-row-actions">
                          <IconButton ariaLabel={`连接测试 ${node.name}`} tooltip="连接测试" onClick={() => void runConnectionTest([node.id])}>
                            {connectionLoading ? <LoaderCircle className="spin" size={15} /> : <Radar size={15} />}
                          </IconButton>
                          <IconButton ariaLabel={`立即扫描 ${node.name}`} tooltip="立即扫描" onClick={() => void runScan([node.id])}>
                            {scanLoading ? <LoaderCircle className="spin" size={15} /> : <RefreshCw size={15} />}
                          </IconButton>
                          <IconButton ariaLabel={`编辑 ${node.name}`} tooltip="编辑" onClick={() => beginEdit(node)}>
                            <Pencil size={15} />
                          </IconButton>
                          <div className="storage-menu-anchor">
                            <IconButton ariaLabel={`更多操作 ${node.name}`} tooltip="更多操作" onClick={() => setMenuState((current) => (current?.nodeId === node.id ? null : { nodeId: node.id }))}>
                              <CircleEllipsis size={15} />
                            </IconButton>
                            {menuState?.nodeId === node.id ? (
                              <div className="context-menu storage-menu-inline">
                                <button type="button" onClick={() => void updateEnabled([node.id], true)}>
                                  启用
                                </button>
                                <button type="button" onClick={() => void updateEnabled([node.id], false)}>
                                  停用
                                </button>
                                <button type="button" onClick={() => openHeartbeatEditor([node.id])}>
                                  设置心跳
                                </button>
                                <button type="button" onClick={() => openCredentialSheet(node)}>
                                  鉴权设置
                                </button>
                                <button type="button" onClick={() => void openHistory(node)}>
                                  查看扫描历史
                                </button>
                                <button type="button" onClick={() => openRelatedTasks(node.id)}>
                                  查看相关任务
                                </button>
                                <button type="button" onClick={() => openRelatedIssues(node.id)}>
                                  查看相关异常
                                </button>
                                <button type="button" onClick={() => setPlaceholderMessage('节点模板 / 复制配置即将支持，当前版本已预留入口。')}>
                                  节点模板 / 复制配置
                                </button>
                                <button className="danger-text" type="button" onClick={() => void removeNode(node)}>
                                  删除节点
                                </button>
                              </div>
                            ) : null}
                          </div>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {formState ? (
        <Sheet onClose={() => setFormState(null)} title={formState.mode === 'create' ? '新增存储节点' : '编辑存储节点'}>
          <div className="sheet-section">
            <strong>基础信息</strong>
            <div className="sheet-form">
              <Field label="节点名称" error={formState.errors.name}>
                <input
                  aria-label="节点名称"
                  value={formState.draft.name}
                  onChange={(event) => updateDraft({ ...formState.draft, name: event.target.value })}
                />
              </Field>
              {formState.mode === 'create' ? (
                <Field label="节点类型" error={formState.errors.nodeType}>
                  <div aria-label="节点类型" className="mini-segmented" role="group">
                    {(['本机磁盘', 'NAS/SMB', '网盘'] as StorageNodeType[]).map((option) => (
                      <button
                        key={option}
                        className={option === formState.draft.nodeType ? 'active' : ''}
                        type="button"
                        onClick={() => updateDraft(resetDraftType(formState.draft, option))}
                      >
                        {option}
                      </button>
                    ))}
                  </div>
                </Field>
              ) : (
                <Field label="节点类型">
                  <div className="field-static-value">{formState.draft.nodeType}</div>
                </Field>
              )}
              <Field label="节点备注">
                <input
                  aria-label="节点备注"
                  value={formState.draft.notes}
                  onChange={(event) => updateDraft({ ...formState.draft, notes: event.target.value })}
                />
              </Field>
            </div>
          </div>

          <div className="sheet-section">
            <strong>类型专属配置</strong>
            <div className="sheet-form">{renderDraftDetailFields(formState.draft, updateDraft, formState.errors)}</div>
          </div>

          <div className="sheet-section">
            <strong>管理选项</strong>
            <div className="sheet-form">
              <Field label="挂载模式">
                <select
                  aria-label="挂载模式"
                  value={formState.draft.mountMode}
                  onChange={(event) => updateDraft({ ...formState.draft, mountMode: event.target.value as StorageNodeDraft['mountMode'] })}
                >
                  <option value="可写">可写</option>
                  <option value="只读">只读</option>
                </select>
              </Field>
              <Field label="心跳周期">
                <select
                  aria-label="心跳周期"
                  value={formState.draft.heartbeatPolicy}
                  onChange={(event) => updateDraft({ ...formState.draft, heartbeatPolicy: event.target.value as StorageHeartbeatPolicy })}
                >
                  {HEARTBEAT_OPTIONS.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            {formState.formConnectionResult ? (
              <div className={`form-test-result ${formState.formConnectionResult.overallTone}`}>
                <strong>{formState.formConnectionResult.summary}</strong>
                <div className="dense-result-list">
                  {formState.formConnectionResult.checks.map((check) => (
                    <div className="dense-result-row" key={check.label}>
                      <span>{check.label}</span>
                      <strong>{check.detail}</strong>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </div>

          <div className="sheet-actions right">
            <ActionButton ariaLabel="测试表单连接" onClick={() => void testDraftConnection()}>
              {formState.testing ? <LoaderCircle className="spin" size={14} /> : <Radar size={14} />}
              {formState.testing ? '测试中...' : '测试连接'}
            </ActionButton>
            <ActionButton ariaLabel="保存节点" tone="primary" onClick={() => void saveDraft()}>
              {formState.saving ? <LoaderCircle className="spin" size={14} /> : null}
              保存节点
            </ActionButton>
          </div>
        </Sheet>
      ) : null}

      {credentialState ? (
        <Sheet onClose={() => setCredentialState(null)} title="鉴权设置">
          <div className="sheet-section">
            <strong>{credentialState.nodeName}</strong>
            <p className="muted-paragraph">鉴权信息将保存在当前 mock 数据源中，后续切换真实后端时可直接复用表单与交互。</p>
            <div className="sheet-form">
              <Field label="鉴权方式">
                <select
                  aria-label="鉴权方式"
                  value={credentialState.authMode}
                  onChange={(event) =>
                    setCredentialState((current) =>
                      current
                        ? {
                            ...current,
                            authMode: event.target.value as StorageCredentialDraft['authMode'],
                          }
                        : current,
                    )
                  }
                >
                  <option value="账号密码">账号密码</option>
                  <option value="填入 Token">填入 Token</option>
                  <option value="扫码登录获取 Token">扫码登录获取 Token</option>
                </select>
              </Field>

              {credentialState.authMode === '账号密码' ? (
                <>
                  <Field label="用户名">
                    <input
                      aria-label="用户名"
                      value={credentialState.username}
                      onChange={(event) =>
                        setCredentialState((current) =>
                          current
                            ? {
                                ...current,
                                username: event.target.value,
                              }
                            : current,
                        )
                      }
                    />
                  </Field>
                  <Field label="密码">
                    <input
                      aria-label="密码"
                      type="password"
                      value={credentialState.password}
                      onChange={(event) =>
                        setCredentialState((current) =>
                          current
                            ? {
                                ...current,
                                password: event.target.value,
                              }
                            : current,
                        )
                      }
                    />
                  </Field>
                </>
              ) : credentialState.authMode === '填入 Token' ? (
                <Field label="Token">
                  <input
                    aria-label="Token"
                    value={credentialState.token}
                    onChange={(event) =>
                      setCredentialState((current) =>
                        current
                          ? {
                              ...current,
                              token: event.target.value,
                            }
                          : current,
                      )
                    }
                  />
                </Field>
              ) : (
                <>
                  <Field label="扫码登录类型">
                    <select
                      aria-label="扫码登录类型"
                      value={credentialState.qrChannel}
                      onChange={(event) =>
                        setCredentialState((current) =>
                          current
                            ? {
                                ...current,
                                qrChannel: event.target.value as StorageCloudQrChannel,
                              }
                            : current,
                        )
                      }
                    >
                      {CLOUD_QR_CHANNEL_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <div className="inline-warning">
                    生成扫码会话后会自动回填 Token，当前为前端 mock 流程。
                  </div>
                  <ActionButton
                    onClick={() =>
                      setCredentialState((current) =>
                        current
                          ? {
                              ...current,
                              token: `mock-token-${Date.now()}`,
                            }
                          : current,
                      )
                    }
                  >
                    生成扫码登录会话
                  </ActionButton>
                </>
              )}
            </div>
          </div>

          <div className="sheet-actions right">
            <ActionButton ariaLabel="保存鉴权" tone="primary" onClick={() => void saveCredentialDraft()}>
              <ShieldCheck size={14} />
              保存鉴权
            </ActionButton>
          </div>
        </Sheet>
      ) : null}

      {heartbeatState ? (
        <Dialog title={heartbeatState.title} onClose={() => setHeartbeatState(null)}>
          <div className="sheet-form">
            <Field label="心跳周期设置">
              <select
                aria-label="心跳周期设置"
                value={heartbeatState.value}
                onChange={(event) =>
                  setHeartbeatState((current) =>
                    current
                      ? {
                          ...current,
                          value: event.target.value as StorageHeartbeatPolicy,
                        }
                      : current,
                  )
                }
              >
                {HEARTBEAT_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </select>
            </Field>
            <p className="muted-paragraph">保存后会同时更新节点列表中的当前周期、下次执行时间与最近结果摘要。</p>
          </div>

          <div className="sheet-actions right">
            <ActionButton ariaLabel="保存心跳" tone="primary" onClick={() => void saveHeartbeat()}>
              {heartbeatState.saving ? <LoaderCircle className="spin" size={14} /> : <HardDrive size={14} />}
              保存心跳
            </ActionButton>
          </div>
        </Dialog>
      ) : null}

      {connectionResults ? (
        <Dialog title="连接测试结果" onClose={() => setConnectionResults(null)}>
          <div className="storage-dialog-stack">
            {connectionResults.map((result) => (
              <section className="dialog-card" key={result.nodeId}>
                <div className="section-header">
                  <div className="row-main">
                    <strong>{result.nodeName}</strong>
                    <span>{result.testedAt}</span>
                  </div>
                  <TonePill tone={result.overallTone}>{result.overallTone === 'success' ? '可继续使用' : '建议处理后再继续'}</TonePill>
                </div>
                <p className="muted-paragraph">{result.summary}</p>
                <div className="dense-result-list">
                  {result.checks.map((check) => (
                    <div className="dense-result-row" key={`${result.nodeId}-${check.label}`}>
                      <span>{check.label}</span>
                      <strong className={`tone-text-${check.status}`}>{check.detail}</strong>
                    </div>
                  ))}
                </div>
                {result.suggestion ? <div className="inline-warning">{result.suggestion}</div> : null}
              </section>
            ))}
          </div>
        </Dialog>
      ) : null}

      {historyState ? (
        <Dialog title="扫描历史" onClose={() => setHistoryState(null)}>
          <div className="storage-dialog-stack">
            <div className="row-main">
              <strong>{historyState.nodeName}</strong>
              <span>最近扫描历史与结果摘要</span>
            </div>
            {historyState.loading ? (
              <div className="empty-state">
                <LoaderCircle className="spin" size={18} />
                <strong>正在加载扫描历史</strong>
              </div>
            ) : (
              historyState.items.map((item) => (
                <section className="dialog-card" key={item.id}>
                  <div className="section-header">
                    <strong>{item.startedAt}</strong>
                    <TonePill tone={item.status === '成功' ? 'success' : item.status === '失败' ? 'critical' : 'info'}>
                      {item.status}
                    </TonePill>
                  </div>
                  <p className="muted-paragraph">{item.summary}</p>
                  <div className="dense-result-list">
                    <div className="dense-result-row">
                      <span>触发方式</span>
                      <strong>{item.trigger}</strong>
                    </div>
                    <div className="dense-result-row">
                      <span>完成时间</span>
                      <strong>{item.finishedAt}</strong>
                    </div>
                  </div>
                </section>
              ))
            )}
          </div>
        </Dialog>
      ) : null}

      {placeholderMessage ? (
        <Dialog title="即将支持" onClose={() => setPlaceholderMessage(null)}>
          <p className="muted-paragraph">{placeholderMessage}</p>
        </Dialog>
      ) : null}
    </section>
  );
}

function renderDraftDetailFields(
  draft: StorageNodeDraft,
  onChange: (draft: StorageNodeDraft) => void,
  errors: Partial<Record<string, string>>,
) {
  if (draft.detail.kind === 'local') {
    const detail = draft.detail;
    return (
      <Field label="根路径" error={errors.rootPath}>
        <input
          aria-label="根路径"
          value={detail.rootPath}
          onChange={(event) =>
            onChange({
              ...draft,
              detail: {
                ...detail,
                rootPath: event.target.value,
              },
            })
          }
        />
      </Field>
    );
  }

  if (draft.detail.kind === 'nas') {
    const detail = draft.detail;
    return (
      <>
        <Field label="主机/IP" error={errors.host}>
          <input
            aria-label="主机/IP"
            placeholder="例如：192.168.10.20"
            value={detail.host}
            onChange={(event) =>
              onChange({
                ...draft,
                detail: {
                  ...detail,
                  host: event.target.value,
                },
              })
            }
          />
        </Field>
        <Field label="共享目录" error={errors.shareName}>
          <input
            aria-label="共享目录"
            placeholder="例如：media"
            value={detail.shareName}
            onChange={(event) =>
              onChange({
                ...draft,
                detail: {
                  ...detail,
                  shareName: event.target.value,
                },
              })
            }
          />
        </Field>
        <Field label="用户名">
          <input
            aria-label="用户名"
            value={detail.username}
            onChange={(event) =>
              onChange({
                ...draft,
                detail: {
                  ...detail,
                  username: event.target.value,
                },
              })
            }
          />
        </Field>
        <Field label="密码">
          <input
            aria-label="密码"
            type="password"
            value={detail.password}
            onChange={(event) =>
              onChange({
                ...draft,
                detail: {
                  ...detail,
                  password: event.target.value,
                },
              })
            }
          />
        </Field>
      </>
    );
  }

  const detail = draft.detail;
  return (
    <>
      <Field label="厂商">
        <select
          aria-label="网盘类型"
          value={detail.vendor}
          onChange={(event) =>
            onChange({
              ...draft,
              detail: {
                ...detail,
                vendor: event.target.value as '115',
              },
            })
          }
        >
          <option value="115">115</option>
        </select>
      </Field>
      <Field label="接入方式">
        <div aria-label="接入方式" className="mini-segmented" role="group">
          {CLOUD_ACCESS_OPTIONS.map((option) => (
            <button
              key={option}
              className={option === detail.accessMethod ? 'active' : ''}
              type="button"
              onClick={() =>
                onChange({
                  ...draft,
                  detail: {
                    ...detail,
                    accessMethod: option,
                    token: option === '填入 Token' ? detail.token : detail.token,
                  },
                })
              }
            >
              {option}
            </button>
          ))}
        </div>
      </Field>
      <Field label="账号别名">
        <input
          aria-label="账号别名"
          value={detail.accountAlias}
          onChange={(event) =>
            onChange({
              ...draft,
              detail: {
                ...detail,
                accountAlias: event.target.value,
              },
            })
          }
        />
      </Field>
      <Field label="挂载目录" error={errors.mountDirectory}>
        <input
          aria-label="挂载目录"
          placeholder="例如：/MareArchive"
          value={detail.mountDirectory}
          onChange={(event) =>
            onChange({
              ...draft,
              detail: {
                ...detail,
                mountDirectory: event.target.value,
              },
            })
          }
        />
      </Field>
      {detail.accessMethod === '填入 Token' ? (
        <Field label="Token" error={errors.token}>
          <input
            aria-label="Token"
            placeholder="例如：eyJhbGciOi..."
            value={detail.token}
            onChange={(event) =>
              onChange({
                ...draft,
                detail: {
                  ...detail,
                  token: event.target.value,
                },
              })
            }
          />
        </Field>
      ) : (
        <>
          <Field label="扫码登录类型">
            <select
              aria-label="扫码登录类型"
              value={detail.qrChannel}
              onChange={(event) =>
                onChange({
                  ...draft,
                  detail: {
                    ...detail,
                    qrChannel: event.target.value as StorageCloudQrChannel,
                  },
                })
              }
            >
              {CLOUD_QR_CHANNEL_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </Field>
          <div className="inline-warning">
            当前仅提供 mock 扫码登录流程，生成会话后会自动回填 Token。
          </div>
          <ActionButton
            onClick={() =>
              onChange({
                ...draft,
                detail: {
                  ...detail,
                  token: `mock-token-${detail.qrChannel}-${Date.now()}`,
                },
              })
            }
          >
            生成扫码登录会话
          </ActionButton>
        </>
      )}
    </>
  );
}

function Field({
  children,
  error,
  label,
}: {
  children: React.ReactNode;
  error?: string;
  label: string;
}) {
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
      <section
        aria-label={title}
        className="dialog-panel"
        role="dialog"
        onClick={(event) => event.stopPropagation()}
      >
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

function resolveStatusFilter(node: StorageNodeRecord): StatusFilter {
  if (!node.enabled) {
    return '已停用';
  }
  if (node.scanStatus === '扫描中') {
    return '扫描中';
  }
  if (node.scanTone === 'critical' || node.authTone === 'critical' || node.riskTags.length > 0) {
    return '异常';
  }
  return '可用';
}

function resolveToneByStatus(status: StatusFilter) {
  if (status === '异常') {
    return 'critical';
  }
  if (status === '扫描中') {
    return 'warning';
  }
  if (status === '已停用') {
    return 'info';
  }
  return 'success';
}

function createDraftFromNode(node: StorageNodeRecord): StorageNodeDraft {
  if (node.detail.kind === 'local') {
    return {
      id: node.id,
      name: node.name,
      nodeType: node.nodeType,
      notes: node.notes,
      mountMode: node.mountMode,
      heartbeatPolicy: node.heartbeatPolicy,
      detail: {
        kind: 'local',
        rootPath: node.detail.rootPath,
      },
    };
  }

  if (node.detail.kind === 'nas') {
    return {
      id: node.id,
      name: node.name,
      nodeType: node.nodeType,
      notes: node.notes,
      mountMode: node.mountMode,
      heartbeatPolicy: node.heartbeatPolicy,
      detail: {
        kind: 'nas',
        protocol: node.detail.protocol,
        host: node.detail.host,
        shareName: node.detail.shareName,
        username: node.detail.username,
        password: '',
      },
    };
  }

  return {
    id: node.id,
    name: node.name,
    nodeType: node.nodeType,
    notes: node.notes,
    mountMode: node.mountMode,
    heartbeatPolicy: node.heartbeatPolicy,
    detail: {
      kind: 'cloud',
      vendor: node.detail.vendor,
      accountAlias: node.detail.accountAlias,
      mountDirectory: node.detail.mountDirectory,
      accessMethod: node.detail.accessMethod,
      qrChannel: node.detail.qrChannel ?? '微信小程序',
      token: '',
    },
  };
}

function createCredentialDraft(node: StorageNodeRecord): StorageCredentialDraft {
  if (node.detail.kind === 'nas') {
    return {
      id: node.id,
      nodeName: node.name,
      authMode: '账号密码',
      username: node.detail.username,
      password: '',
      token: '',
      qrChannel: '微信小程序',
    };
  }

  if (node.detail.kind !== 'cloud') {
    return {
      id: node.id,
      nodeName: node.name,
      authMode: '填入 Token',
      username: '',
      password: '',
      token: '',
      qrChannel: '微信小程序',
    };
  }

  return {
    id: node.id,
    nodeName: node.name,
    authMode: node.detail.accessMethod,
    username: '',
    password: '',
    token: '',
    qrChannel: node.detail.qrChannel ?? '微信小程序',
  };
}

function resetDraftType(draft: StorageNodeDraft, nodeType: StorageNodeType): StorageNodeDraft {
  if (nodeType === draft.nodeType) {
    return draft;
  }

  if (nodeType === '本机磁盘') {
    return {
      id: draft.id,
      name: draft.name,
      nodeType,
      notes: draft.notes,
      mountMode: draft.mountMode,
      heartbeatPolicy: draft.heartbeatPolicy,
      detail: {
        kind: 'local',
        rootPath: '',
      },
    };
  }

  if (nodeType === 'NAS/SMB') {
    return {
      id: draft.id,
      name: draft.name,
      nodeType,
      notes: draft.notes,
      mountMode: draft.mountMode,
      heartbeatPolicy: draft.heartbeatPolicy,
      detail: {
        kind: 'nas',
        protocol: 'SMB',
        host: '',
        shareName: '',
        username: '',
        password: '',
      },
    };
  }

  return {
    id: draft.id,
    name: draft.name,
    nodeType,
    notes: draft.notes,
    mountMode: draft.mountMode,
    heartbeatPolicy: draft.heartbeatPolicy,
    detail: {
      kind: 'cloud',
      vendor: '115',
      accountAlias: '',
      mountDirectory: '',
      accessMethod: '填入 Token',
      qrChannel: '微信小程序',
      token: '',
    },
  };
}

function validateDraft(draft: StorageNodeDraft) {
  const errors: Partial<Record<string, string>> = {};

  if (!draft.name.trim()) {
    errors.name = '请输入节点名称';
  }
  if (draft.detail.kind === 'local' && !draft.detail.rootPath.trim()) {
    errors.rootPath = '请输入根路径';
  }
  if (draft.detail.kind === 'nas') {
    if (!draft.detail.host.trim()) {
      errors.host = '请输入主机/IP';
    }
    if (!draft.detail.shareName.trim()) {
      errors.shareName = '请输入共享目录';
    }
  }
  if (draft.detail.kind === 'cloud') {
    if (!draft.detail.mountDirectory.trim()) {
      errors.mountDirectory = '请输入挂载目录';
    }
    if (!draft.detail.token.trim()) {
      errors.token = draft.detail.accessMethod === '填入 Token' ? '请输入 Token' : '请先生成扫码登录会话';
    }
  }

  return errors;
}

function buildDraftConnectionResult(draft: StorageNodeDraft): StorageConnectionTestResult {
  const checks = [];

  if (draft.detail.kind === 'local') {
    checks.push({ label: '可达性', status: 'success' as const, detail: `路径 ${draft.detail.rootPath} 可访问。` });
    checks.push({ label: '鉴权状态', status: 'success' as const, detail: '本地节点无需鉴权。' });
  } else if (draft.detail.kind === 'nas') {
    checks.push({ label: '可达性', status: 'success' as const, detail: `主机 ${draft.detail.host} 可达。` });
    checks.push({
      label: '鉴权状态',
      status: draft.detail.username ? ('success' as const) : ('warning' as const),
      detail: draft.detail.username ? '已填写账号信息。' : '建议补充账号信息后再保存。',
    });
  } else {
    checks.push({ label: '可达性', status: 'success' as const, detail: `网盘类型 ${draft.detail.vendor} 接入通道可用。` });
    checks.push({
      label: '鉴权状态',
      status: draft.detail.token ? ('success' as const) : ('warning' as const),
      detail:
        draft.detail.accessMethod === '扫码登录获取 Token'
          ? draft.detail.token
            ? `已通过${draft.detail.qrChannel}生成 Token。`
            : `请先完成${draft.detail.qrChannel}扫码登录。`
          : draft.detail.token
            ? 'Token 已填写。'
            : 'Token 为空，保存前请补齐。',
    });
  }

  checks.push({ label: '读权限', status: 'success' as const, detail: '模拟环境中可读取目标目录。' });
  checks.push({
    label: '写权限',
    status: draft.mountMode === '只读' ? ('warning' as const) : ('success' as const),
    detail: draft.mountMode === '只读' ? '当前为只读挂载。' : '当前配置允许写入。',
  });
  checks.push({ label: '目标目录可访问', status: 'success' as const, detail: '目录检查通过。' });

  return {
    nodeId: draft.id ?? 'draft',
    nodeName: draft.name || '未命名节点',
    overallTone: checks.some((item) => item.status === 'warning') ? 'warning' : 'success',
    summary: '连接测试已完成，当前结果来自表单即时模拟，可用于提前确认接入配置方向。',
    checks,
    suggestion: checks.some((item) => item.status === 'warning') ? '检查配置后保存' : '可继续保存节点',
    testedAt: '刚刚',
  };
}

function extractErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

function waitBriefly() {
  return new Promise((resolve) => window.setTimeout(resolve, 420));
}
