import { useEffect, useMemo, useState } from 'react';
import {
  ArrowRightLeft,
  FolderOpen,
  HardDrive,
  Search,
} from 'lucide-react';
import type {
  IssueCapabilities,
  IssueNature,
  IssueRecord,
  IssueSourceDomain,
  IssueStatus,
  Library,
  Severity,
} from '../data';
import { ActionButton, DenseRow, EmptyState, SelectPill, Sheet, TonePill } from '../components/Shared';

const CATEGORY_OPTIONS = ['全部', '冲突', '传输', '校验', '节点与权限', '容量与资源', '扫描与解析', '清理与治理'] as const;
const NATURE_OPTIONS = ['全部问题', '阻塞型异常', '提醒型风险'] as const;
const SOURCE_OPTIONS = ['全部来源', '传输任务', '其他任务', '文件中心', '存储节点', '系统治理'] as const;
const STATUS_OPTIONS = ['全部', '待处理', '待确认', '处理中', '已忽略', '已解决', '已归档'] as const;
const SEVERITY_OPTIONS = ['全部级别', '高优先级', '需尽快处理', '信息'] as const;
const SORT_OPTIONS = ['默认排序', '最近更新时间', '严重级别', '名称'] as const;

type IssueActionType = 'retry' | 'confirm' | 'ignore' | 'archive';
type IssueFilterContext = {
  issueId?: string;
  taskId?: string;
  endpointId?: string;
  fileNodeId?: string;
  path?: string;
  label?: string;
} | null;

export type IssueFocusRequest = {
  issueId?: string;
  taskId?: string;
  sourceDomain?: IssueSourceDomain;
  libraryId?: string;
  endpointId?: string;
  fileNodeId?: string;
  path?: string;
  label?: string;
} | null;

export function IssuesPage({
  issues,
  libraries,
  focusRequest,
  onClearFocusRequest,
  onConsumeFocusRequest,
  onIssueAction,
  onClearHistory,
  onOpenFileCenter,
  onOpenStorageNodes,
  onOpenTaskCenter,
}: {
  issues: IssueRecord[];
  libraries: Library[];
  focusRequest?: IssueFocusRequest;
  onClearFocusRequest?: () => void;
  onConsumeFocusRequest?: () => void;
  onIssueAction: (ids: string[], action: IssueActionType) => void;
  onClearHistory: (ids: string[]) => void;
  onOpenFileCenter: (issue: IssueRecord) => void;
  onOpenStorageNodes: (issue: IssueRecord) => void;
  onOpenTaskCenter: (issue: IssueRecord) => void;
}) {
  const [categoryFilter, setCategoryFilter] = useState<(typeof CATEGORY_OPTIONS)[number]>('全部');
  const [natureFilter, setNatureFilter] = useState<(typeof NATURE_OPTIONS)[number]>('全部问题');
  const [sourceFilter, setSourceFilter] = useState<(typeof SOURCE_OPTIONS)[number]>('全部来源');
  const [libraryFilter, setLibraryFilter] = useState('全部资产库');
  const [statusFilter, setStatusFilter] = useState<(typeof STATUS_OPTIONS)[number]>('全部');
  const [severityFilter, setSeverityFilter] = useState<(typeof SEVERITY_OPTIONS)[number]>('全部级别');
  const [sortValue, setSortValue] = useState<(typeof SORT_OPTIONS)[number]>('默认排序');
  const [searchText, setSearchText] = useState('');
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [detailIssueId, setDetailIssueId] = useState<string | null>(null);
  const [menuIssueId, setMenuIssueId] = useState<string | null>(null);
  const [contextFilter, setContextFilter] = useState<IssueFilterContext>(null);

  const libraryNameMap = useMemo(
    () => new Map(libraries.map((library) => [library.id, library.name])),
    [libraries],
  );

  const libraryOptions = useMemo(() => ['全部资产库', ...libraries.map((library) => library.name)], [libraries]);
  const issueOrder = useMemo(() => new Map(issues.map((issue, index) => [issue.id, index])), [issues]);

  useEffect(() => {
    if (!focusRequest) {
      return;
    }

    setCategoryFilter('全部');
    setNatureFilter('全部问题');
    setStatusFilter('全部');
    setSeverityFilter('全部级别');
    setSortValue('默认排序');
    setSearchText('');
    setSourceFilter(focusRequest.sourceDomain ?? '全部来源');
    setLibraryFilter(
      focusRequest.libraryId ? (libraryNameMap.get(focusRequest.libraryId) ?? '全部资产库') : '全部资产库',
    );
    setContextFilter({
      issueId: focusRequest.issueId,
      taskId: focusRequest.taskId,
      endpointId: focusRequest.endpointId,
      fileNodeId: focusRequest.fileNodeId,
      path: focusRequest.path,
      label: focusRequest.label,
    });
    setSelectedIds(focusRequest.issueId ? [focusRequest.issueId] : []);
    if (focusRequest.issueId) {
      setDetailIssueId(focusRequest.issueId);
    }
    onConsumeFocusRequest?.();
  }, [focusRequest, libraryNameMap, onConsumeFocusRequest]);

  useEffect(() => {
    if (!menuIssueId) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      if (target.closest('.issue-action-anchor')) return;
      if (target.closest('.context-menu')) return;
      setMenuIssueId(null);
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, [menuIssueId]);

  useEffect(() => {
    setSelectedIds((current) => current.filter((id) => issues.some((issue) => issue.id === id)));
    if (detailIssueId && !issues.some((issue) => issue.id === detailIssueId)) {
      setDetailIssueId(null);
    }
  }, [detailIssueId, issues]);

  const filteredIssues = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    const filtered = issues.filter((issue) => {
      const matchesCategory = categoryFilter === '全部' ? true : issue.category === categoryFilter;
      const matchesNature =
        natureFilter === '全部问题'
          ? true
          : natureFilter === '阻塞型异常'
            ? issue.nature === 'BLOCKING'
            : issue.nature === 'RISK';
      const matchesSource = sourceFilter === '全部来源' ? true : issue.sourceDomain === sourceFilter;
      const matchesLibrary =
        libraryFilter === '全部资产库' ? true : libraryNameMap.get(issue.libraryId) === libraryFilter;
      const matchesStatus = statusFilter === '全部' ? true : issue.status === statusFilter;
      const matchesSeverity = matchSeverityFilter(issue.severity, severityFilter);
      const matchesContext = matchContextFilter(issue, contextFilter);
      const matchesSearch = keyword ? createIssueSearchText(issue, libraryNameMap).includes(keyword) : true;

      return (
        matchesCategory &&
        matchesNature &&
        matchesSource &&
        matchesLibrary &&
        matchesStatus &&
        matchesSeverity &&
        matchesContext &&
        matchesSearch
      );
    });

    return [...filtered].sort((left, right) => compareIssues(left, right, sortValue, issueOrder));
  }, [
    categoryFilter,
    contextFilter,
    issueOrder,
    issues,
    libraryFilter,
    libraryNameMap,
    natureFilter,
    searchText,
    severityFilter,
    sortValue,
    sourceFilter,
    statusFilter,
  ]);

  const detailIssue = useMemo(
    () => filteredIssues.find((issue) => issue.id === detailIssueId) ?? issues.find((issue) => issue.id === detailIssueId) ?? null,
    [detailIssueId, filteredIssues, issues],
  );

  const selectedIssues = filteredIssues.filter((issue) => selectedIds.includes(issue.id));
  const batchActions = useMemo(() => buildBatchActions(selectedIssues), [selectedIssues]);
  const visibleHistoryIds = filteredIssues
    .filter((issue) => isHistoricalStatus(issue.status))
    .map((issue) => issue.id);
  const blockingCount = filteredIssues.filter((issue) => issue.nature === 'BLOCKING').length;
  const riskCount = filteredIssues.filter((issue) => issue.nature === 'RISK').length;
  const historyCount = filteredIssues.filter((issue) => isHistoricalStatus(issue.status)).length;

  return (
    <section className="page-stack issues-page">
      <div className="toolbar-card action-toolbar issue-toolbar">
        <div className="toolbar-group wrap issue-toolbar-main">
          <SelectPill
            ariaLabel="异常类型筛选"
            options={[...CATEGORY_OPTIONS]}
            value={categoryFilter}
            onChange={(value) => setCategoryFilter(value as (typeof CATEGORY_OPTIONS)[number])}
          />
          <SelectPill
            ariaLabel="问题性质筛选"
            options={[...NATURE_OPTIONS]}
            value={natureFilter}
            onChange={(value) => setNatureFilter(value as (typeof NATURE_OPTIONS)[number])}
          />
          <SelectPill
            ariaLabel="来源域筛选"
            options={[...SOURCE_OPTIONS]}
            value={sourceFilter}
            onChange={(value) => setSourceFilter(value as (typeof SOURCE_OPTIONS)[number])}
          />
          <SelectPill ariaLabel="资产库筛选" options={libraryOptions} value={libraryFilter} onChange={setLibraryFilter} />
          <SelectPill
            ariaLabel="异常状态筛选"
            options={[...STATUS_OPTIONS]}
            value={statusFilter}
            onChange={(value) => setStatusFilter(value as (typeof STATUS_OPTIONS)[number])}
          />
          <SelectPill
            ariaLabel="严重级别筛选"
            options={[...SEVERITY_OPTIONS]}
            value={severityFilter}
            onChange={(value) => setSeverityFilter(value as (typeof SEVERITY_OPTIONS)[number])}
          />
          <SelectPill
            ariaLabel="排序方式"
            options={[...SORT_OPTIONS]}
            value={sortValue}
            onChange={(value) => setSortValue(value as (typeof SORT_OPTIONS)[number])}
          />
          <label className="search-field issue-search-field" htmlFor="issue-search">
            <Search size={14} />
            <input
              id="issue-search"
              aria-label="异常搜索"
              placeholder="搜索标题、类型、任务、文件、路径、端点"
              type="search"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
        </div>
      </div>

      {selectedIssues.length > 0 ? (
        <div className="toolbar-card selection-toolbar issue-selection-toolbar">
          <span className="selection-caption">已选择 {selectedIssues.length} 条异常</span>
          <div className="toolbar-group wrap">
            {batchActions.map((action) => (
              <ActionButton key={action.id} ariaLabel={action.label} onClick={() => onIssueAction(selectedIds, action.id)}>
                {action.label}
              </ActionButton>
            ))}
            {selectedIssues.every((issue) => isHistoricalStatus(issue.status)) ? (
              <ActionButton ariaLabel="清理选中历史" tone="danger" onClick={() => onClearHistory(selectedIds)}>
                清理选中历史
              </ActionButton>
            ) : null}
            <ActionButton onClick={() => setSelectedIds([])}>清空选择</ActionButton>
          </div>
        </div>
      ) : null}

      {contextFilter?.label ? (
        <div className="toolbar-card issue-context-banner">
          <span>{contextFilter.label}</span>
          <ActionButton
            onClick={() => {
              setContextFilter(null);
              onClearFocusRequest?.();
            }}
          >
            清除筛选
          </ActionButton>
        </div>
      ) : null}

      <div className="workspace-card compact-list issue-list-card">
        <div className="section-header issue-list-header">
          <div className="issue-list-summary">
            <strong>匹配 {filteredIssues.length} 条异常</strong>
            <div className="endpoint-row">
              <span className="tag-summary-badge warning">
                <span>阻塞型异常</span>
                <strong>{blockingCount}</strong>
              </span>
              <span className="tag-summary-badge info">
                <span>提醒型风险</span>
                <strong>{riskCount}</strong>
              </span>
              <span className="tag-summary-badge success">
                <span>历史项</span>
                <strong>{historyCount}</strong>
              </span>
            </div>
          </div>
          <div className="toolbar-group wrap">
            {visibleHistoryIds.length > 0 ? (
              <ActionButton tone="danger" onClick={() => onClearHistory(visibleHistoryIds)}>
                清理当前历史
              </ActionButton>
            ) : null}
          </div>
        </div>

        {filteredIssues.length === 0 ? (
          <EmptyState
            title={issues.length === 0 ? '当前没有待处理异常' : '当前筛选条件下没有匹配异常'}
            description={
              issues.length === 0
                ? '当前工作区下没有需要人工处理或回查的异常记录。'
                : '可以调整筛选条件、清除任务上下文，或切换到历史状态查看已处理记录。'
            }
          />
        ) : (
          <>
            {filteredIssues.map((issue) => {
              const primaryAction = resolvePrimaryAction(issue);
              const relatedActions = buildIssueMenuActions(issue);
              const libraryLabel = libraryNameMap.get(issue.libraryId) ?? issue.libraryId;

              return (
                <article className="list-row issue-record-row" key={issue.id}>
                  <label className="issue-select-cell">
                    <input
                      aria-label={`选择 ${issue.title}`}
                      checked={selectedIds.includes(issue.id)}
                      type="checkbox"
                      onChange={() =>
                        setSelectedIds((current) =>
                          current.includes(issue.id) ? current.filter((id) => id !== issue.id) : [...current, issue.id],
                        )
                      }
                    />
                  </label>

                  <div className="row-main issue-main">
                    <div className="issue-title-row">
                      <strong>{issue.title}</strong>
                      <div className="endpoint-row">
                        <span className="transfer-task-type-pill">{issue.category}</span>
                        <span className={`issue-nature-pill ${issue.nature === 'BLOCKING' ? 'blocking' : 'risk'}`}>
                          {getNatureLabel(issue.nature)}
                        </span>
                        <TonePill tone={issue.severity}>{getSeverityLabel(issue.severity)}</TonePill>
                      </div>
                    </div>
                    <span>{issue.summary}</span>
                    <div className="endpoint-row issue-meta-row">
                      <TonePill tone="info">{issue.sourceDomain}</TonePill>
                      <TonePill tone="warning">{libraryLabel}</TonePill>
                      <span className="issue-object-text" title={issue.objectLabel}>
                        {issue.objectLabel}
                      </span>
                    </div>
                  </div>

                  <div className="row-main issue-side-cell">
                    <strong>{issue.status}</strong>
                    <span>{issue.updatedAt}</span>
                  </div>

                  <div className="row-main issue-side-cell">
                    <strong>{issue.actionLabel}</strong>
                    <span>{issue.action}</span>
                  </div>

                  <div className="row-actions issue-row-actions">
                    <ActionButton onClick={() => setDetailIssueId(issue.id)}>详情</ActionButton>
                    {primaryAction ? (
                      <ActionButton onClick={() => onIssueAction([issue.id], primaryAction.id)}>{primaryAction.label}</ActionButton>
                    ) : null}
                    <div className="issue-action-anchor">
                      <ActionButton onClick={() => setMenuIssueId((current) => (current === issue.id ? null : issue.id))}>
                        更多
                      </ActionButton>
                      {menuIssueId === issue.id ? (
                        <div className="context-menu issue-action-menu">
                          {relatedActions.map((action) =>
                            action.kind === 'command' ? (
                              <button
                                key={`${issue.id}-${action.id}`}
                                type="button"
                                onClick={() => {
                                  setMenuIssueId(null);
                                  onIssueAction([issue.id], action.id);
                                }}
                              >
                                {action.label}
                              </button>
                            ) : (
                              <button
                                key={`${issue.id}-${action.id}`}
                                type="button"
                                onClick={() => {
                                  setMenuIssueId(null);
                                  if (action.id === 'open-task') onOpenTaskCenter(issue);
                                  if (action.id === 'open-file') onOpenFileCenter(issue);
                                  if (action.id === 'open-storage') onOpenStorageNodes(issue);
                                }}
                              >
                                {action.label}
                              </button>
                            ),
                          )}
                        </div>
                      ) : null}
                    </div>
                  </div>
                </article>
              );
            })}
          </>
        )}
      </div>

      {detailIssue ? (
        <IssueDetailSheet
          issue={detailIssue}
          libraryLabel={libraryNameMap.get(detailIssue.libraryId) ?? detailIssue.libraryId}
          onAction={onIssueAction}
          onClose={() => setDetailIssueId(null)}
          onOpenFileCenter={onOpenFileCenter}
          onOpenStorageNodes={onOpenStorageNodes}
          onOpenTaskCenter={onOpenTaskCenter}
        />
      ) : null}
    </section>
  );
}

function IssueDetailSheet({
  issue,
  libraryLabel,
  onAction,
  onClose,
  onOpenFileCenter,
  onOpenStorageNodes,
  onOpenTaskCenter,
}: {
  issue: IssueRecord;
  libraryLabel: string;
  onAction: (ids: string[], action: IssueActionType) => void;
  onClose: () => void;
  onOpenFileCenter: (issue: IssueRecord) => void;
  onOpenStorageNodes: (issue: IssueRecord) => void;
  onOpenTaskCenter: (issue: IssueRecord) => void;
}) {
  const actions = buildIssueMenuActions(issue);

  return (
    <Sheet onClose={onClose} title={issue.title}>
      <div className="sheet-section">
        <div className="row-actions">
          {actions
            .filter((action) => action.kind === 'command')
            .map((action) => (
              <ActionButton key={action.id} onClick={() => onAction([issue.id], action.id)}>
                {action.label}
              </ActionButton>
            ))}
        </div>
      </div>

      <div className="sheet-section">
        <DenseRow label="异常类型" value={issue.type} />
        <DenseRow label="一级分类" value={issue.category} />
        <DenseRow label="问题性质" value={getNatureLabel(issue.nature)} />
        <DenseRow label="严重级别" tone={issue.severity} value={getSeverityLabel(issue.severity)} />
        <DenseRow label="当前状态" tone={resolveStatusTone(issue.status)} value={issue.status} />
        <DenseRow label="所属资产库" value={libraryLabel} />
        <DenseRow label="首次发现时间" value={issue.createdAt} />
        <DenseRow label="最近更新时间" value={issue.updatedAt} />
        <DenseRow label="当前建议动作" value={issue.actionLabel} />
      </div>

      <div className="sheet-section">
        <strong>来源上下文</strong>
        <DenseRow label="来源域" value={issue.sourceDomain} />
        <DenseRow label="来源摘要" value={issue.source.sourceLabel ?? '—'} />
        <DenseRow label="关联任务" value={issue.source.taskTitle ?? '—'} />
        <DenseRow label="关联任务子项" value={issue.source.taskItemTitle ?? '—'} />
        <DenseRow label="关联端点" value={issue.source.endpointLabel ?? '—'} />
        <DenseRow label="关联路径" value={issue.source.path ?? '—'} />
        <DenseRow label="影响对象" value={issue.objectLabel} />
      </div>

      <div className="sheet-section">
        <strong>影响范围</strong>
        <DenseRow label="影响资产数" value={`${issue.impact.assetCount}`} />
        <DenseRow label="影响副本数" value={`${issue.impact.replicaCount}`} />
        <DenseRow label="影响目录数" value={`${issue.impact.folderCount}`} />
        <DenseRow label="影响端点数" value={`${issue.impact.endpointCount}`} />
        <DenseRow label="阻塞正式状态提交" value={issue.impact.blocksStatusCommit ? '是' : '否'} />
        <DenseRow label="阻塞任务继续执行" value={issue.impact.blocksTaskExecution ? '是' : '否'} />
      </div>

      <div className="sheet-section">
        <strong>处理建议</strong>
        <p className="muted-paragraph">{issue.suggestion}</p>
        <p className="muted-paragraph">{issue.detail}</p>
      </div>

      <div className="sheet-section">
        <strong>处理历史</strong>
        <div className="issue-history-list">
          {issue.histories.map((history) => (
            <div className="issue-history-item" key={history.id}>
              <div>
                <strong>{history.action}</strong>
                <p>{history.result}</p>
              </div>
              <div className="notice-meta">
                <span>{history.operatorLabel}</span>
                <span>{history.createdAt}</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="sheet-actions right">
        {issue.capabilities.canOpenStorageNodes ? (
          <ActionButton onClick={() => onOpenStorageNodes(issue)}>
            <HardDrive size={14} />
            打开存储节点
          </ActionButton>
        ) : null}
        {issue.capabilities.canOpenFileCenter ? (
          <ActionButton onClick={() => onOpenFileCenter(issue)}>
            <FolderOpen size={14} />
            打开文件中心
          </ActionButton>
        ) : null}
        {issue.capabilities.canOpenTaskCenter ? (
          <ActionButton onClick={() => onOpenTaskCenter(issue)}>
            <ArrowRightLeft size={14} />
            打开任务中心
          </ActionButton>
        ) : null}
      </div>
    </Sheet>
  );
}

function buildBatchActions(issues: IssueRecord[]) {
  if (issues.length === 0) {
    return [];
  }

  const allCan = (predicate: (capabilities: IssueCapabilities, issue: IssueRecord) => boolean) =>
    issues.every((issue) => predicate(issue.capabilities, issue));

  const actions: Array<{ id: IssueActionType; label: string }> = [];

  if (allCan((capabilities, issue) => canRetryIssue(issue, capabilities))) {
    actions.push({ id: 'retry', label: '批量重试' });
  }
  if (allCan((capabilities, issue) => canConfirmIssue(issue, capabilities))) {
    actions.push({ id: 'confirm', label: '批量标记已确认' });
  }
  if (allCan((capabilities, issue) => canIgnoreIssue(issue, capabilities))) {
    actions.push({ id: 'ignore', label: '批量忽略' });
  }
  if (allCan((capabilities, issue) => canArchiveIssue(issue, capabilities))) {
    actions.push({ id: 'archive', label: '批量归档' });
  }

  return actions;
}

function buildIssueMenuActions(issue: IssueRecord) {
  const actions: Array<
    | { kind: 'command'; id: IssueActionType; label: string }
    | { kind: 'link'; id: 'open-task' | 'open-file' | 'open-storage'; label: string }
  > = [];

  if (canRetryIssue(issue, issue.capabilities)) actions.push({ kind: 'command', id: 'retry', label: '重试' });
  if (canConfirmIssue(issue, issue.capabilities)) actions.push({ kind: 'command', id: 'confirm', label: '标记已确认' });
  if (canIgnoreIssue(issue, issue.capabilities)) actions.push({ kind: 'command', id: 'ignore', label: '忽略' });
  if (canArchiveIssue(issue, issue.capabilities)) actions.push({ kind: 'command', id: 'archive', label: '归档' });
  if (issue.capabilities.canOpenTaskCenter) actions.push({ kind: 'link', id: 'open-task', label: '打开任务中心' });
  if (issue.capabilities.canOpenFileCenter) actions.push({ kind: 'link', id: 'open-file', label: '打开文件中心' });
  if (issue.capabilities.canOpenStorageNodes) actions.push({ kind: 'link', id: 'open-storage', label: '打开存储节点' });

  return actions;
}

function resolvePrimaryAction(issue: IssueRecord) {
  if (canRetryIssue(issue, issue.capabilities)) return { id: 'retry' as const, label: '重试' };
  if (canConfirmIssue(issue, issue.capabilities)) return { id: 'confirm' as const, label: '标记确认' };
  if (canIgnoreIssue(issue, issue.capabilities)) return { id: 'ignore' as const, label: '忽略' };
  if (canArchiveIssue(issue, issue.capabilities)) return { id: 'archive' as const, label: '归档' };
  return null;
}

function canRetryIssue(issue: IssueRecord, capabilities: IssueCapabilities) {
  return Boolean(capabilities.canRetry) && ['待处理', '处理中'].includes(issue.status);
}

function canConfirmIssue(issue: IssueRecord, capabilities: IssueCapabilities) {
  return Boolean(capabilities.canConfirm) && ['待处理', '待确认'].includes(issue.status);
}

function canIgnoreIssue(issue: IssueRecord, capabilities: IssueCapabilities) {
  return Boolean(capabilities.canIgnore) && ['待处理', '待确认', '处理中'].includes(issue.status);
}

function canArchiveIssue(issue: IssueRecord, capabilities: IssueCapabilities) {
  return Boolean(capabilities.canArchive) && ['已解决', '已忽略'].includes(issue.status);
}

function isHistoricalStatus(status: IssueStatus) {
  return ['已忽略', '已解决', '已归档'].includes(status);
}

function getNatureLabel(nature: IssueNature) {
  return nature === 'BLOCKING' ? '阻塞型异常' : '提醒型风险';
}

function getSeverityLabel(severity: Severity) {
  if (severity === 'critical') return '高优先级';
  if (severity === 'warning') return '需尽快处理';
  if (severity === 'success') return '已解决';
  return '信息';
}

function resolveStatusTone(status: IssueStatus): Severity {
  if (status === '待处理') return 'critical';
  if (status === '待确认' || status === '处理中') return 'warning';
  if (status === '已解决') return 'success';
  return 'info';
}

function matchSeverityFilter(severity: Severity, filter: (typeof SEVERITY_OPTIONS)[number]) {
  if (filter === '全部级别') return true;
  if (filter === '高优先级') return severity === 'critical';
  if (filter === '需尽快处理') return severity === 'warning';
  return severity === 'info' || severity === 'success';
}

function createIssueSearchText(issue: IssueRecord, libraryNameMap: Map<string, string>) {
  return [
    issue.title,
    issue.summary,
    issue.type,
    issue.category,
    issue.asset,
    issue.objectLabel,
    issue.detail,
    issue.action,
    issue.source.taskTitle,
    issue.source.taskItemTitle,
    issue.source.endpointLabel,
    issue.source.path,
    issue.source.sourceLabel,
    libraryNameMap.get(issue.libraryId),
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
}

function matchContextFilter(issue: IssueRecord, contextFilter: IssueFilterContext) {
  if (!contextFilter) {
    return true;
  }

  if (contextFilter.issueId && issue.id !== contextFilter.issueId) {
    return false;
  }
  if (contextFilter.taskId && issue.taskId !== contextFilter.taskId && issue.source.taskId !== contextFilter.taskId) {
    return false;
  }
  if (contextFilter.endpointId && issue.source.endpointId !== contextFilter.endpointId) {
    return false;
  }
  if (contextFilter.fileNodeId && issue.source.fileNodeId !== contextFilter.fileNodeId) {
    return false;
  }
  if (contextFilter.path && !(issue.source.path ?? '').toLowerCase().includes(contextFilter.path.toLowerCase())) {
    return false;
  }
  return true;
}

function compareIssues(
  left: IssueRecord,
  right: IssueRecord,
  sortValue: (typeof SORT_OPTIONS)[number],
  issueOrder: Map<string, number>,
) {
  if (sortValue === '名称') {
    return left.title.localeCompare(right.title, 'zh-CN');
  }

  if (sortValue === '严重级别') {
    const severityDelta = severityOrder(left.severity) - severityOrder(right.severity);
    if (severityDelta !== 0) {
      return severityDelta;
    }
    return left.title.localeCompare(right.title, 'zh-CN');
  }

  if (sortValue === '最近更新时间') {
    return getTimestampRank(right.updatedAt) - getTimestampRank(left.updatedAt);
  }

  const natureDelta = natureOrder(left.nature) - natureOrder(right.nature);
  if (natureDelta !== 0) {
    return natureDelta;
  }

  const statusDelta = statusOrder(left.status) - statusOrder(right.status);
  if (statusDelta !== 0) {
    return statusDelta;
  }

  const severityDelta = severityOrder(left.severity) - severityOrder(right.severity);
  if (severityDelta !== 0) {
    return severityDelta;
  }

  const updatedDelta = getTimestampRank(right.updatedAt) - getTimestampRank(left.updatedAt);
  if (updatedDelta !== 0) {
    return updatedDelta;
  }

  return (issueOrder.get(left.id) ?? 0) - (issueOrder.get(right.id) ?? 0);
}

function natureOrder(nature: IssueNature) {
  return nature === 'BLOCKING' ? 0 : 1;
}

function statusOrder(status: IssueStatus) {
  if (status === '待处理') return 0;
  if (status === '待确认') return 1;
  if (status === '处理中') return 2;
  if (status === '已忽略') return 3;
  if (status === '已解决') return 4;
  return 5;
}

function severityOrder(severity: Severity) {
  if (severity === 'critical') return 0;
  if (severity === 'warning') return 1;
  if (severity === 'info') return 2;
  return 3;
}

function getTimestampRank(value: string) {
  if (value === '刚刚') return 10_000_000_000;

  const minuteMatch = value.match(/^(\d+)\s*分钟前$/);
  if (minuteMatch) {
    return 9_000_000_000 - Number(minuteMatch[1]);
  }

  const todayMatch = value.match(/^今天\s+(\d{2}):(\d{2})$/);
  if (todayMatch) {
    return 8_000_000_000 + Number(todayMatch[1]) * 60 + Number(todayMatch[2]);
  }

  const yesterdayMatch = value.match(/^昨天\s+(\d{2}):(\d{2})$/);
  if (yesterdayMatch) {
    return 7_000_000_000 + Number(yesterdayMatch[1]) * 60 + Number(yesterdayMatch[2]);
  }

  const parsed = Date.parse(value.replace(' ', 'T'));
  return Number.isFinite(parsed) ? parsed : 0;
}
