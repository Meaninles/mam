import { useEffect, useMemo, useState } from 'react';
import {
  ChevronRight,
  Eye,
  File,
  FileAudio2,
  FileImage,
  FileText,
  FolderOpen,
  Info,
  RefreshCw,
  Search,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react';
import type {
  ImportDeviceSessionRecord,
  ImportDraftRecord,
  ImportReportSnapshot,
  ImportTargetEndpointRecord,
  IssueRecord,
} from '../data';
import {
  filterImportDevices,
  resolveImportSubmitState,
  sortImportDevices,
  sortImportReports,
  type ImportCenterFilterValue,
  type ImportCenterSortValue,
} from '../lib/importCenter';
import { ActionButton, DenseRow, EmptyState, IconButton, SelectPill, Sheet, TonePill } from '../components/Shared';

export type ImportBrowserNode = {
  id: string;
  deviceSessionId: string;
  entryType: 'FILE' | 'DIRECTORY';
  name: string;
  relativePath: string;
  fileKind: string;
  size?: string;
  modifiedAt: string;
  isHidden: boolean;
  hasChildren: boolean;
  targetEndpointIds: string[];
};

type BrowserState = {
  currentPath: string;
  items: ImportBrowserNode[];
  total: number;
  hasMore: boolean;
};

export function ImportCenterPage(props: {
  libraries: Array<{ id: string; name: string }>;
  devices: ImportDeviceSessionRecord[];
  drafts: ImportDraftRecord[];
  issues: IssueRecord[];
  reports: ImportReportSnapshot[];
  targetEndpoints: ImportTargetEndpointRecord[];
  browserState: BrowserState | null;
  browserLoading: boolean;
  onBrowseSession: (deviceSessionId: string, path?: string) => void;
  onOpenFolder: (deviceSessionId: string, path: string) => void;
  onGoToParentFolder: (deviceSessionId: string, path: string) => void;
  onOpenFileCenter: (libraryId: string) => void;
  onOpenIssueCenter: (issueIds: string[]) => void;
  onOpenStorageNodes: (endpointIds: string[]) => void;
  onOpenTaskCenter: (taskId?: string) => void;
  onRefreshDevices: () => void;
  onSelectLibrary: (draftId: string, libraryId: string) => void;
  onRefreshPrecheck: (draftId: string) => void;
  onSaveSelectionTargets: (
    deviceSessionId: string,
    payload: { entryType: string; name: string; relativePath: string; targetEndpointIds: string[] },
  ) => void;
  onSubmitImport: (deviceSessionId: string) => void;
}) {
  const {
    devices,
    drafts,
    reports,
    targetEndpoints,
    libraries,
    browserState,
    browserLoading,
    onBrowseSession,
    onOpenFolder,
    onGoToParentFolder,
    onOpenFileCenter,
    onOpenIssueCenter,
    onOpenTaskCenter,
    onRefreshDevices,
    onSelectLibrary,
    onRefreshPrecheck,
    onSaveSelectionTargets,
    onSubmitImport,
  } = props;

  const [selectedDeviceId, setSelectedDeviceId] = useState<string | null>(null);
  const [filterValue, setFilterValue] = useState<ImportCenterFilterValue>('全部');
  const [searchText, setSearchText] = useState('');
  const [sortValue, setSortValue] = useState<ImportCenterSortValue>('最近接入');
  const [deviceDetailId, setDeviceDetailId] = useState<string | null>(null);
  const [precheckDraftId, setPrecheckDraftId] = useState<string | null>(null);
  const [reportSheetId, setReportSheetId] = useState<string | null>(null);
  const [submitDeviceId, setSubmitDeviceId] = useState<string | null>(null);
  const [browserSearchText, setBrowserSearchText] = useState('');
  const [selectedBrowserPaths, setSelectedBrowserPaths] = useState<string[]>([]);

  const draftById = useMemo(() => new Map(drafts.map((draft) => [draft.id, draft])), [drafts]);
  const deviceById = useMemo(() => new Map(devices.map((device) => [device.id, device])), [devices]);
  const reportById = useMemo(() => new Map(reports.map((report) => [report.id, report])), [reports]);
  const targetById = useMemo(() => new Map(targetEndpoints.map((target) => [target.id, target])), [targetEndpoints]);

  const visibleDevices = useMemo(
    () => sortImportDevices(filterImportDevices(devices, filterValue, searchText), sortValue),
    [devices, filterValue, searchText, sortValue],
  );

  useEffect(() => {
    const fallbackDevice = visibleDevices.find((device) => device.sessionStatus !== '已拔出') ?? visibleDevices[0] ?? null;
    const nextDeviceId = selectedDeviceId && deviceById.has(selectedDeviceId) ? selectedDeviceId : fallbackDevice?.id ?? null;
    if (nextDeviceId && nextDeviceId !== selectedDeviceId) {
      setSelectedDeviceId(nextDeviceId);
      onBrowseSession(nextDeviceId);
    }
    if (!nextDeviceId) {
      setSelectedDeviceId(null);
    }
  }, [deviceById, onBrowseSession, selectedDeviceId, visibleDevices]);

  const selectedDevice = selectedDeviceId ? deviceById.get(selectedDeviceId) ?? null : null;
  const selectedDraft = selectedDevice?.activeDraftId ? draftById.get(selectedDevice.activeDraftId) ?? null : null;
  const selectedReport = selectedDevice?.latestReportId ? reportById.get(selectedDevice.latestReportId) ?? null : null;
  const availableTargets = useMemo(() => {
    if (!selectedDevice) return [];
    return selectedDevice.availableTargetEndpointIds
      .map((targetId) => targetById.get(targetId))
      .filter((target): target is ImportTargetEndpointRecord => Boolean(target));
  }, [selectedDevice, targetById]);

  const browserItems = useMemo(() => {
    const items = browserState?.items ?? [];
    const keyword = browserSearchText.trim().toLowerCase();
    if (!keyword) {
      return items;
    }
    return items.filter((item) => [item.name, item.relativePath, item.fileKind].join(' ').toLowerCase().includes(keyword));
  }, [browserSearchText, browserState?.items]);

  useEffect(() => {
    setSelectedBrowserPaths([]);
  }, [selectedDeviceId, browserState?.currentPath]);

  const selectedEntries = useMemo(
    () => browserItems.filter((item) => item.targetEndpointIds.length > 0),
    [browserItems],
  );
  const selectedBrowserItems = useMemo(
    () => browserItems.filter((item) => selectedBrowserPaths.includes(item.relativePath)),
    [browserItems, selectedBrowserPaths],
  );
  const allVisibleSelected = browserItems.length > 0 && browserItems.every((item) => selectedBrowserPaths.includes(item.relativePath));
  const submitState =
    selectedDevice && selectedDraft ? resolveImportSubmitState(selectedDevice, selectedDraft, selectedEntries as any) : null;

  const reportSheet = reportSheetId ? reportById.get(reportSheetId) ?? null : null;
  const deviceDetail = deviceDetailId ? deviceById.get(deviceDetailId) ?? null : null;
  const precheckDraft = precheckDraftId ? draftById.get(precheckDraftId) ?? null : null;
  const submitDevice = submitDeviceId ? deviceById.get(submitDeviceId) ?? null : null;
  const recentReports = useMemo(() => sortImportReports(reports), [reports]);
  const currentPath = browserState?.currentPath ?? '/';
  const browserBreadcrumbs = useMemo(() => {
    if (currentPath === '/' || currentPath.trim() === '') {
      return [{ label: '/', path: '' }];
    }
    const segments = currentPath.replace(/^\/+/, '').split('/').filter(Boolean);
    return [{ label: '/', path: '' }, ...segments.map((segment, index) => ({ label: segment, path: segments.slice(0, index + 1).join('/') }))];
  }, [currentPath]);

  return (
    <section className="page-stack import-center-page">
      <div className="toolbar-card action-toolbar">
        <div className="toolbar-group wrap">
          <div className="import-inline-pills" role="group" aria-label="会话状态筛选">
            {(['全部', '待扫描', '可导入', '导入中', '异常'] as ImportCenterFilterValue[]).map((item) => (
              <button key={item} className={filterValue === item ? 'active' : ''} type="button" onClick={() => setFilterValue(item)}>
                {item}
              </button>
            ))}
          </div>
          <label className="search-field import-device-search" htmlFor="import-device-search">
            <Search size={14} />
            <input
              id="import-device-search"
              placeholder="搜索设备名称或地址"
              type="search"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
          <SelectPill
            ariaLabel="导入设备排序方式"
            options={['最近接入', '状态优先', '名称', '剩余空间']}
            value={sortValue}
            onChange={(value) => setSortValue(value as ImportCenterSortValue)}
          />
        </div>
        <div className="toolbar-group wrap">
          <ActionButton onClick={onRefreshDevices}>刷新设备</ActionButton>
          <ActionButton onClick={() => recentReports[0] && setReportSheetId(recentReports[0].id)}>最近摘要</ActionButton>
        </div>
      </div>

      <div className="import-center-layout">
        <div className="workspace-card import-device-list-card">
          {visibleDevices.length === 0 ? (
            <EmptyState title="当前没有匹配的待导入端" description="可以调整状态筛选、搜索词或刷新设备池。" />
          ) : (
            <div className="compact-list">
              {visibleDevices.map((device) => (
                <article
                  className={`import-device-row simple${selectedDeviceId === device.id ? ' active' : ''}`}
                  key={device.id}
                  onClick={() => {
                    setSelectedDeviceId(device.id);
                    onBrowseSession(device.id);
                  }}
                >
                  <div className="import-device-text">
                    <strong title={device.deviceLabel}>{device.deviceLabel}</strong>
                    <span className="import-device-meta" title={`${device.mountPath} · 可用空间 ${device.capacitySummary.available}`}>
                      {device.mountPath} · 可用空间 {device.capacitySummary.available}
                    </span>
                  </div>
                  <div className="import-device-actions">
                    <IconButton ariaLabel={`打开会话 ${device.deviceLabel}`} tooltip="打开会话" onClick={() => onBrowseSession(device.id)}>
                      <Eye size={15} />
                    </IconButton>
                    <IconButton ariaLabel={`查看预检 ${device.deviceLabel}`} tooltip="查看预检" onClick={() => setPrecheckDraftId(device.activeDraftId ?? null)}>
                      <ShieldCheck size={15} />
                    </IconButton>
                    <IconButton ariaLabel={`查看详情 ${device.deviceLabel}`} tooltip="查看详情" onClick={() => setDeviceDetailId(device.id)}>
                      <Info size={15} />
                    </IconButton>
                  </div>
                </article>
              ))}
            </div>
          )}
        </div>

        <div className="import-workspace-stack">
          {!selectedDevice ? (
            <EmptyState title="当前没有可查看的设备会话" description="插入设备后可以在左侧列表中继续处理。" />
          ) : (
            <>
              <div className="workspace-card">
                <header className="section-header">
                  <strong>{selectedDevice.deviceLabel}</strong>
                  <TonePill tone={resolveDeviceTone(selectedDevice.sessionStatus)}>{selectedDevice.sessionStatus}</TonePill>
                </header>
                <div className="single-column import-summary-grid">
                  <label className="form-field">
                    <span>导入到资产库</span>
                    <select
                      aria-label="导入到资产库"
                      value={selectedDraft?.libraryId ?? ''}
                      onChange={(event) => selectedDraft && onSelectLibrary(selectedDraft.id, event.target.value)}
                    >
                      <option value="">请选择资产库</option>
                      {libraries.map((library) => (
                        <option key={library.id} value={library.id}>
                          {library.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <DenseRow label="来源路径" value={selectedDevice.mountPath} />
                  <DenseRow label="容量 / 可用空间" value={`${selectedDevice.capacitySummary.total} / ${selectedDevice.capacitySummary.available}`} />
                  <DenseRow label="最近在线" value={selectedDevice.lastSeenAt} />
                </div>
                <div className="toolbar-group wrap import-primary-actions">
                  <ActionButton onClick={() => onBrowseSession(selectedDevice.id, currentPath === '/' ? undefined : currentPath.slice(1))}>
                    <RefreshCw size={14} />
                    刷新当前层级
                  </ActionButton>
                  <ActionButton onClick={() => selectedDraft && onRefreshPrecheck(selectedDraft.id)}>
                    <ShieldCheck size={14} />
                    重新预检
                  </ActionButton>
                  {selectedReport ? <ActionButton onClick={() => onOpenTaskCenter(selectedReport.taskId)}>打开任务中心</ActionButton> : null}
                </div>
              </div>

              <div className="workspace-card">
                <header className="section-header">
                  <strong>预检摘要</strong>
                  <div className="toolbar-group wrap">
                    <TonePill tone="success">{selectedDraft?.precheckSummary.passedCount ?? 0} 通过</TonePill>
                    <TonePill tone="warning">{selectedDraft?.precheckSummary.riskCount ?? 0} 风险</TonePill>
                    <TonePill tone="critical">{selectedDraft?.precheckSummary.blockingCount ?? 0} 阻塞</TonePill>
                  </div>
                </header>
                {selectedDraft ? (
                  <>
                    <div className="dense-result-list">
                      {(selectedDraft.precheckSummary.items ?? []).slice(0, 4).map((item) => (
                        <div className="dense-result-row" key={item.id}>
                          <div className="row-main">
                            <strong>{item.label}</strong>
                            <span>{item.detail}</span>
                          </div>
                          <TonePill tone={resolveCheckTone(item.status)}>
                            {item.status === 'passed' ? '通过' : item.status === 'risk' ? '风险' : '阻塞'}
                          </TonePill>
                        </div>
                      ))}
                    </div>
                    <div className="toolbar-group wrap">
                      <ActionButton onClick={() => setPrecheckDraftId(selectedDraft.id)}>查看预检详情</ActionButton>
                      {selectedDevice.issueIds.length > 0 ? (
                        <ActionButton onClick={() => onOpenIssueCenter(selectedDevice.issueIds)}>
                          <TriangleAlert size={14} />
                          打开异常中心
                        </ActionButton>
                      ) : null}
                    </div>
                  </>
                ) : (
                  <EmptyState title="当前设备还没有预检数据" description="请先选择资产库和导入对象，然后再触发预检。" />
                )}
              </div>

              <div className="workspace-card">
                <header className="section-header">
                  <div className="address-bar import-browser-breadcrumbs">
                    {browserBreadcrumbs.map((item) => (
                      <button
                        key={`${item.label}-${item.path}`}
                        type="button"
                        onClick={() => onBrowseSession(selectedDevice.id, item.path || undefined)}
                      >
                        {item.label}
                      </button>
                    ))}
                  </div>
                  <div className="toolbar-group wrap">
                    <label className="search-field import-inline-search" htmlFor={`import-browser-search-${selectedDevice.id}`}>
                      <Search size={14} />
                      <input
                        id={`import-browser-search-${selectedDevice.id}`}
                        placeholder="筛选当前层级名称"
                        type="search"
                        value={browserSearchText}
                        onChange={(event) => setBrowserSearchText(event.target.value)}
                      />
                    </label>
                  </div>
                </header>

                <div className="toolbar-group wrap">
                  <ActionButton
                    disabled={currentPath === '/'}
                    onClick={() => onGoToParentFolder(selectedDevice.id, currentPath)}
                  >
                    返回上一级
                  </ActionButton>
                </div>

                {selectedBrowserItems.length > 0 && selectedDraft?.libraryId ? (
                  <div className="toolbar-card action-toolbar import-selection-toolbar">
                    <div className="toolbar-group wrap">
                      <span className="selection-caption">已选 {selectedBrowserItems.length} 项</span>
                    </div>
                    <div className="toolbar-group wrap">
                      {availableTargets.map((target) => (
                        <ActionButton
                          key={`batch-target-${target.id}`}
                          onClick={() => {
                            selectedBrowserItems.forEach((item) => {
                              const nextTargets = Array.from(new Set([...item.targetEndpointIds, target.id]));
                              onSaveSelectionTargets(selectedDevice.id, {
                                entryType: item.entryType,
                                name: item.name,
                                relativePath: item.relativePath,
                                targetEndpointIds: nextTargets,
                              });
                            });
                          }}
                        >
                          批量同步到 {target.label}
                        </ActionButton>
                      ))}
                    </div>
                  </div>
                ) : null}

                {browserLoading ? (
                  <EmptyState title="正在加载当前层级" description="仅加载当前路径下的直接文件夹和文件，请稍候。" />
                ) : browserItems.length === 0 ? (
                  <EmptyState title="当前层级没有可显示内容" description="可以切换目录、刷新当前层级，或返回上一级继续选择。" />
                ) : (
                  <div className="storage-table-wrap">
                    <table className="file-table storage-table import-file-table file-center-table">
                      <colgroup>
                        <col className="import-col-checkbox" />
                        <col className="import-col-name" />
                        <col className="import-col-modified" />
                        <col className="import-col-targets" />
                      </colgroup>
                      <thead>
                        <tr>
                          <th className="checkbox-cell">
                            <input
                              aria-label="选择当前层级全部对象"
                              checked={allVisibleSelected}
                              type="checkbox"
                              onChange={() =>
                                setSelectedBrowserPaths((current) =>
                                  allVisibleSelected
                                    ? current.filter((path) => !browserItems.some((item) => item.relativePath === path))
                                    : Array.from(new Set([...current, ...browserItems.map((item) => item.relativePath)])),
                                )
                              }
                            />
                          </th>
                          <th scope="col">名称</th>
                          <th scope="col">修改时间</th>
                          <th scope="col">目标端</th>
                        </tr>
                      </thead>
                      <tbody>
                        {browserItems.map((item) => (
                          <tr
                            key={item.id}
                            aria-selected={selectedBrowserPaths.includes(item.relativePath)}
                            onDoubleClick={() => item.entryType === 'DIRECTORY' && onOpenFolder(selectedDevice.id, item.relativePath)}
                          >
                            <td className="checkbox-cell">
                              <input
                                aria-label={`选择 ${item.name}`}
                                checked={selectedBrowserPaths.includes(item.relativePath)}
                                type="checkbox"
                                onChange={() =>
                                  setSelectedBrowserPaths((current) =>
                                    current.includes(item.relativePath)
                                      ? current.filter((path) => path !== item.relativePath)
                                      : [...current, item.relativePath],
                                  )
                                }
                              />
                            </td>
                            <td>
                              <div className="file-center-name-cell">
                                <div className={`file-center-icon ${item.entryType === 'DIRECTORY' ? 'folder' : 'file'}`}>
                                  <ImportNodeIcon item={item} />
                                </div>
                                <div className="storage-node-cell">
                                  <div className="storage-node-title">
                                    <button
                                      className="link-button import-node-link"
                                      title={item.name}
                                      type="button"
                                      onClick={() => item.entryType === 'DIRECTORY' && onOpenFolder(selectedDevice.id, item.relativePath)}
                                    >
                                      <strong title={item.name}>{item.name}</strong>
                                      {item.entryType === 'DIRECTORY' && item.hasChildren ? <ChevronRight size={14} /> : null}
                                    </button>
                                  </div>
                                  <div className="endpoint-row file-center-tag-row">
                                    {item.isHidden ? <TonePill tone="warning">隐藏</TonePill> : null}
                                    {item.size ? <span className="selection-caption">{item.size}</span> : null}
                                  </div>
                                  <span title={item.relativePath}>{item.relativePath}</span>
                                </div>
                              </div>
                            </td>
                            <td>{formatImportDateTime(item.modifiedAt)}</td>
                            <td>
                              {selectedDraft?.libraryId ? (
                                <div className="checkbox-row">
                                  {availableTargets.map((target) => {
                                    const checked = item.targetEndpointIds.includes(target.id);
                                    return (
                                      <label key={`${item.id}-${target.id}`} className={`target-check${checked ? ' checked' : ''}`}>
                                        <input
                                          checked={checked}
                                          type="checkbox"
                                          onChange={() => {
                                            const nextTargets = checked
                                              ? item.targetEndpointIds.filter((targetId) => targetId !== target.id)
                                              : [...item.targetEndpointIds, target.id];
                                            onSaveSelectionTargets(selectedDevice.id, {
                                              entryType: item.entryType,
                                              name: item.name,
                                              relativePath: item.relativePath,
                                              targetEndpointIds: nextTargets,
                                            });
                                          }}
                                        />
                                        <span>{target.label}</span>
                                      </label>
                                    );
                                  })}
                                </div>
                              ) : (
                                <span className="selection-caption">请先选择资产库</span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>

              {selectedReport ? (
                <div className="workspace-card">
                  <header className="section-header">
                    <strong>结果摘要</strong>
                    <TonePill tone={resolveReportTone(selectedReport.status)}>{selectedReport.status}</TonePill>
                  </header>
                  <div className="import-result-grid">
                    <div className="import-overview-metric">
                      <span>成功对象</span>
                      <strong>{selectedReport.successCount}</strong>
                    </div>
                    <div className="import-overview-metric">
                      <span>失败对象</span>
                      <strong>{selectedReport.failedCount}</strong>
                    </div>
                    <div className="import-overview-metric">
                      <span>待补处理</span>
                      <strong>{selectedReport.partialCount}</strong>
                    </div>
                    <div className="import-overview-metric">
                      <span>最近更新时间</span>
                      <strong>{selectedReport.latestUpdatedAt}</strong>
                    </div>
                  </div>
                  <p className="muted-text">{selectedReport.verifySummary}</p>
                  <div className="toolbar-group wrap">
                    <ActionButton onClick={() => onOpenTaskCenter(selectedReport.taskId)}>打开任务中心</ActionButton>
                    <ActionButton onClick={() => setReportSheetId(selectedReport.id)}>查看导入报告</ActionButton>
                    {selectedDraft?.libraryId ? <ActionButton onClick={() => onOpenFileCenter(selectedDraft.libraryId)}>去文件中心</ActionButton> : null}
                    {selectedReport.issueIds.length > 0 ? (
                      <ActionButton onClick={() => onOpenIssueCenter(selectedReport.issueIds)}>打开异常中心</ActionButton>
                    ) : null}
                  </div>
                </div>
              ) : null}

              <div className="workspace-card import-submit-card">
                <div className="row-main">
                  <strong>提交区</strong>
                  <span>
                    {submitState?.reason
                      ? submitState.reason
                      : submitState?.actionLabel === '查看任务'
                        ? '当前会话已经进入运行期管理，后续请去任务中心继续跟踪。'
                        : '当前层级只做轻量浏览；提交导入后会在后台递归展开所选目录与文件。'}
                  </span>
                </div>
                <div className="toolbar-group wrap">
                  <ActionButton onClick={() => selectedDraft && onRefreshPrecheck(selectedDraft.id)}>重新预检</ActionButton>
                  {submitState?.actionLabel === '查看任务' ? (
                    <ActionButton tone="primary" onClick={() => onOpenTaskCenter(selectedReport?.taskId)}>
                      查看任务
                    </ActionButton>
                  ) : (
                    <ActionButton disabled={submitState?.disabled} tone="primary" onClick={() => setSubmitDeviceId(selectedDevice.id)}>
                      提交导入
                    </ActionButton>
                  )}
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {deviceDetail ? (
        <Sheet onClose={() => setDeviceDetailId(null)} title={deviceDetail.deviceLabel}>
          <div className="sheet-section">
            <DenseRow label="设备类型" value={deviceDetail.deviceType} />
            <DenseRow label="来源路径" value={deviceDetail.mountPath} />
            <DenseRow label="接入时间" value={deviceDetail.connectedAt} />
            <DenseRow label="最后在线" value={deviceDetail.lastSeenAt} />
            <DenseRow label="容量 / 可用空间" value={`${deviceDetail.capacitySummary.total} / ${deviceDetail.capacitySummary.available}`} />
            <DenseRow label="扫描状态" value={deviceDetail.scanStatus} />
            <DenseRow label="会话状态" value={deviceDetail.sessionStatus} />
          </div>
        </Sheet>
      ) : null}

      {precheckDraft ? (
        <Sheet onClose={() => setPrecheckDraftId(null)} title="预检详情">
          <div className="dense-result-list">
            {(precheckDraft.precheckSummary.items ?? []).map((item) => (
              <div className="dense-result-row" key={item.id}>
                <div className="row-main">
                  <strong>{item.label}</strong>
                  <span>{item.detail}</span>
                </div>
                <TonePill tone={resolveCheckTone(item.status)}>
                  {item.status === 'passed' ? '通过' : item.status === 'risk' ? '风险' : '阻塞'}
                </TonePill>
              </div>
            ))}
          </div>
        </Sheet>
      ) : null}

      {reportSheet ? (
        <Sheet onClose={() => setReportSheetId(null)} title="导入报告快照">
          <div className="dense-result-list">
            {reportSheet.targetSummaries.map((target) => (
              <div className="dense-result-row" key={`${reportSheet.id}-${target.endpointId}`}>
                <div className="row-main">
                  <strong>{target.label}</strong>
                  <span>
                    {target.transferredSize} · {target.successCount} 成功 / {target.failedCount} 失败
                  </span>
                </div>
                <TonePill tone={target.failedCount > 0 ? 'warning' : 'success'}>{target.status}</TonePill>
              </div>
            ))}
          </div>
        </Sheet>
      ) : null}

      {submitDevice ? (
        <ConfirmDialog
          notes={[
            selectedDraft?.precheckSummary.blockingCount ? `当前还有 ${selectedDraft.precheckSummary.blockingCount} 个阻塞项。` : null,
            submitState?.reason ?? null,
          ].filter((item): item is string => Boolean(item))}
          onCancel={() => setSubmitDeviceId(null)}
          onConfirm={() => {
            onSubmitImport(submitDevice.id);
            setSubmitDeviceId(null);
          }}
          title="确认提交导入"
        />
      ) : null}
    </section>
  );
}

function ConfirmDialog(props: { notes: string[]; onCancel: () => void; onConfirm: () => void; title: string }) {
  const { notes, onCancel, onConfirm, title } = props;

  return (
    <div className="dialog-backdrop" role="presentation" onClick={onCancel}>
      <section aria-label={title} className="dialog-panel compact-confirm-dialog" role="dialog" onClick={(event) => event.stopPropagation()}>
        <div className="sheet-header">
          <strong>{title}</strong>
        </div>
        <div className="dialog-card">
          <p className="muted-paragraph">提交后会在后台递归扫描所选文件夹和文件，并创建真实导入作业。</p>
          {notes.map((note) => (
            <p className="confirm-warning" key={note}>
              {note}
            </p>
          ))}
        </div>
        <div className="sheet-actions right">
          <ActionButton onClick={onCancel}>取消</ActionButton>
          <ActionButton tone="primary" onClick={onConfirm}>
            确认提交
          </ActionButton>
        </div>
      </section>
    </div>
  );
}

function resolveDeviceTone(status: string) {
  if (status === '异常待处理' || status === '已拔出') return 'critical';
  if (status === '导入中' || status === '部分完成' || status === '扫描中') return 'warning';
  if (status === '可导入') return 'success';
  return 'info';
}

function resolveCheckTone(status: 'passed' | 'risk' | 'blocking') {
  if (status === 'blocking') return 'critical';
  if (status === 'risk') return 'warning';
  return 'success';
}

function resolveReportTone(status: string) {
  if (status === '失败') return 'critical';
  if (status === '部分成功' || status === '运行中' || status === '已排队') return 'warning';
  return 'success';
}

function ImportNodeIcon({ item }: { item: ImportBrowserNode }) {
  if (item.entryType === 'DIRECTORY') {
    return <FolderOpen size={16} />;
  }
  if (item.fileKind === '图片') {
    return <FileImage size={16} />;
  }
  if (item.fileKind === '音频') {
    return <FileAudio2 size={16} />;
  }
  if (item.fileKind === '文档') {
    return <FileText size={16} />;
  }
  return <File size={16} />;
}

function formatImportDateTime(value?: string) {
  if (!value) {
    return '—';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}
