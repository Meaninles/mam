import { useEffect, useMemo, useState } from 'react';
import {
  Eye,
  Info,
  RefreshCw,
  ScanSearch,
  Search,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react';
import type {
  ImportDeviceSessionRecord,
  ImportDraftRecord,
  ImportReportSnapshot,
  ImportSourceNodeRecord,
  ImportTargetEndpointRecord,
  IssueRecord,
} from '../data';
import {
  filterImportDevices,
  resolveImportFileFilterMatch,
  resolveImportFileStatusTone,
  resolveImportSubmitState,
  sortImportDevices,
  sortImportReports,
  type ImportCenterFilterValue,
  type ImportCenterSortValue,
  type ImportFileFilterValue,
} from '../lib/importCenter';
import { ActionButton, DenseRow, EmptyState, IconButton, SelectPill, Sheet, TonePill } from '../components/Shared';

type SessionViewState = {
  fileFilter: ImportFileFilterValue;
  searchText: string;
};

export function ImportCenterPage(props: {
  devices: ImportDeviceSessionRecord[];
  drafts: ImportDraftRecord[];
  issues: IssueRecord[];
  reports: ImportReportSnapshot[];
  sourceNodes: ImportSourceNodeRecord[];
  targetEndpoints: ImportTargetEndpointRecord[];
  onApplyTargetToAll: (deviceSessionId: string, targetEndpointId: string) => void;
  onRemoveTargetFromAll: (deviceSessionId: string, targetEndpointId: string) => void;
  onOpenFileCenter: (libraryId: string) => void;
  onOpenIssueCenter: (issueIds: string[]) => void;
  onOpenStorageNodes: (endpointIds: string[]) => void;
  onOpenTaskCenter: (taskId?: string) => void;
  onRefreshDevices: () => void;
  onRefreshPrecheck: (draftId: string) => void;
  onRescanDevice: (deviceSessionId: string) => void;
  onSaveDraft: (draftId: string) => void;
  onSetSourceTargets: (sourceNodeId: string, targetEndpointIds: string[]) => void;
  onSubmitImport: (deviceSessionId: string) => string | null;
}) {
  const {
    devices,
    drafts,
    reports,
    sourceNodes,
    targetEndpoints,
    onApplyTargetToAll,
    onRemoveTargetFromAll,
    onOpenFileCenter,
    onOpenIssueCenter,
    onOpenStorageNodes,
    onOpenTaskCenter,
    onRefreshDevices,
    onRefreshPrecheck,
    onRescanDevice,
    onSaveDraft,
    onSetSourceTargets,
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
  const [sessionViewState, setSessionViewState] = useState<Record<string, SessionViewState>>({});

  const draftById = useMemo(() => new Map(drafts.map((draft) => [draft.id, draft])), [drafts]);
  const deviceById = useMemo(() => new Map(devices.map((device) => [device.id, device])), [devices]);
  const reportById = useMemo(() => new Map(reports.map((report) => [report.id, report])), [reports]);
  const targetById = useMemo(() => new Map(targetEndpoints.map((target) => [target.id, target])), [targetEndpoints]);

  const visibleDevices = useMemo(
    () => sortImportDevices(filterImportDevices(devices, filterValue, searchText), sortValue),
    [devices, filterValue, searchText, sortValue],
  );

  useEffect(() => {
    if (selectedDeviceId && deviceById.has(selectedDeviceId)) {
      return;
    }

    const fallbackDevice = visibleDevices.find((device) => device.sessionStatus !== '已拔出') ?? visibleDevices[0] ?? null;
    setSelectedDeviceId(fallbackDevice?.id ?? null);
  }, [deviceById, selectedDeviceId, visibleDevices]);

  const selectedDevice = selectedDeviceId ? deviceById.get(selectedDeviceId) ?? null : null;
  const selectedDraft = selectedDevice?.activeDraftId ? draftById.get(selectedDevice.activeDraftId) ?? null : null;
  const selectedReport = selectedDevice?.latestReportId ? reportById.get(selectedDevice.latestReportId) ?? null : null;
  const selectedSourceNodes = useMemo(
    () => (selectedDevice ? sourceNodes.filter((node) => node.deviceSessionId === selectedDevice.id) : []),
    [selectedDevice, sourceNodes],
  );
  const availableTargets = useMemo(() => {
    if (!selectedDevice) return [];
    return selectedDevice.availableTargetEndpointIds
      .map((targetId) => targetById.get(targetId))
      .filter((target): target is ImportTargetEndpointRecord => Boolean(target));
  }, [selectedDevice, targetById]);

  const currentSessionState = selectedDevice
    ? sessionViewState[selectedDevice.id] ?? { fileFilter: '全部', searchText: '' }
    : { fileFilter: '全部' as ImportFileFilterValue, searchText: '' };

  const filteredSessionFiles = useMemo(
    () =>
      selectedSourceNodes.filter((file) =>
        resolveImportFileFilterMatch(file, currentSessionState.fileFilter, currentSessionState.searchText),
      ),
    [currentSessionState.fileFilter, currentSessionState.searchText, selectedSourceNodes],
  );

  const submitState =
    selectedDevice && selectedDraft ? resolveImportSubmitState(selectedDevice, selectedDraft, selectedSourceNodes) : null;

  const reportSheet = reportSheetId ? reportById.get(reportSheetId) ?? null : null;
  const deviceDetail = deviceDetailId ? deviceById.get(deviceDetailId) ?? null : null;
  const precheckDraft = precheckDraftId ? draftById.get(precheckDraftId) ?? null : null;
  const submitDevice = submitDeviceId ? deviceById.get(submitDeviceId) ?? null : null;
  const recentReports = useMemo(() => sortImportReports(reports), [reports]);
  const actionableSourceNodes = useMemo(
    () => selectedSourceNodes.filter((node) => node.status !== '已跳过'),
    [selectedSourceNodes],
  );

  function updateSessionState(deviceSessionId: string, nextState: Partial<SessionViewState>) {
    setSessionViewState((current) => ({
      ...current,
      [deviceSessionId]: {
        ...(current[deviceSessionId] ?? { fileFilter: '全部', searchText: '' }),
        ...nextState,
      },
    }));
  }

  return (
    <section className="page-stack import-center-page">
      <div className="toolbar-card action-toolbar">
        <div className="toolbar-group wrap">
          <div className="import-inline-pills" role="group" aria-label="会话状态筛选">
            {(['全部', '待扫描', '可导入', '导入中', '异常'] as ImportCenterFilterValue[]).map((item) => (
              <button
                key={item}
                className={filterValue === item ? 'active' : ''}
                type="button"
                onClick={() => setFilterValue(item)}
              >
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
                  onClick={() => setSelectedDeviceId(device.id)}
                >
                  <div className="import-device-text">
                    <strong>{device.deviceLabel}</strong>
                    <span
                      className="import-device-meta"
                      title={`${device.mountPath} · 可用空间 ${device.capacitySummary.available}`}
                    >
                      {device.mountPath} · 可用空间 {device.capacitySummary.available}
                    </span>
                  </div>
                  <div className="import-device-actions">
                    <IconButton ariaLabel={`打开会话 ${device.deviceLabel}`} tooltip="打开会话" onClick={() => setSelectedDeviceId(device.id)}>
                      <Eye size={15} />
                    </IconButton>
                    <IconButton ariaLabel={`重新扫描 ${device.deviceLabel}`} tooltip="重新扫描" onClick={() => onRescanDevice(device.id)}>
                      <RefreshCw size={15} />
                    </IconButton>
                    <IconButton
                      ariaLabel={`查看预检 ${device.deviceLabel}`}
                      tooltip="查看预检"
                      onClick={() => setPrecheckDraftId(device.activeDraftId ?? null)}
                    >
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
                  <DenseRow label="来源路径" value={selectedDevice.mountPath} />
                  <DenseRow
                    label="容量 / 可用空间"
                    value={`${selectedDevice.capacitySummary.total} / ${selectedDevice.capacitySummary.available}`}
                  />
                  <DenseRow label="文件数 / 文件夹数" value={`${selectedDevice.fileCount} / ${selectedDevice.folderCount}`} />
                  <DenseRow label="最近扫描" value={selectedDraft?.precheckSummary.updatedAt ?? selectedDevice.lastSeenAt} />
                </div>
                <div className="toolbar-group wrap import-primary-actions">
                  <ActionButton onClick={() => onRescanDevice(selectedDevice.id)}>
                    <ScanSearch size={14} />
                    重新扫描
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
                  <strong>目标编排</strong>
                </header>
                {availableTargets.length === 0 ? (
                  <EmptyState title="当前设备还没有可用目标端" description="请先检查存储节点状态，或等待扫描完成后再继续。" />
                ) : (
                  <>
                    <div className="import-target-grid">
                      {availableTargets.map((target) => {
                        const appliedCount = actionableSourceNodes.filter((node) => node.targetEndpointIds.includes(target.id)).length;
                        const totalCount = actionableSourceNodes.length;
                        const coverageState =
                          totalCount === 0 || appliedCount === 0
                            ? 'none'
                            : appliedCount === totalCount
                              ? 'all'
                              : 'partial';

                        return (
                          <article className={`import-target-card ${coverageState}`} key={target.id}>
                            <div className="row-main">
                              <div className="import-report-head">
                                <strong>{target.label}</strong>
                                <TonePill tone={target.tone}>{target.statusLabel}</TonePill>
                              </div>
                              <span>
                                {target.type} · {target.availableSpace}
                              </span>
                              <span>
                                {coverageState === 'all'
                                  ? '当前设备全部导入到该节点'
                                  : coverageState === 'partial'
                                    ? `当前设备部分文件导入到该节点（${appliedCount}/${totalCount}）`
                                    : '当前设备未导入到该节点'}
                              </span>
                            </div>
                            <div className="toolbar-group wrap">
                              <ActionButton onClick={() => onApplyTargetToAll(selectedDevice.id, target.id)}>应用到全部文件</ActionButton>
                              <ActionButton onClick={() => onRemoveTargetFromAll(selectedDevice.id, target.id)}>全部取消</ActionButton>
                              <ActionButton onClick={() => onOpenStorageNodes([target.endpointId])}>查看目标端</ActionButton>
                            </div>
                          </article>
                        );
                      })}
                    </div>
                  </>
                )}
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
                      {selectedDraft.precheckSummary.items.slice(0, 4).map((item) => (
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
                  <EmptyState title="当前设备还没有预检数据" description="可以重新扫描设备，或等待草稿恢复后继续。" />
                )}
              </div>

              <div className="workspace-card">
                <header className="section-header">
                  <strong>文件清单</strong>
                  <div className="toolbar-group wrap">
                    <div className="import-inline-pills" role="group" aria-label="文件状态筛选">
                      {(['全部', '可导入', '已导入', '冲突', '异常'] as ImportFileFilterValue[]).map((item) => (
                        <button
                          key={item}
                          className={currentSessionState.fileFilter === item ? 'active' : ''}
                          type="button"
                          onClick={() => updateSessionState(selectedDevice.id, { fileFilter: item })}
                        >
                          {item}
                        </button>
                      ))}
                    </div>
                    <label className="search-field import-inline-search" htmlFor={`import-file-search-${selectedDevice.id}`}>
                      <Search size={14} />
                      <input
                        id={`import-file-search-${selectedDevice.id}`}
                        placeholder="搜索文件名或路径"
                        type="search"
                        value={currentSessionState.searchText}
                        onChange={(event) => updateSessionState(selectedDevice.id, { searchText: event.target.value })}
                      />
                    </label>
                  </div>
                </header>
                {filteredSessionFiles.length === 0 ? (
                  <EmptyState title="当前筛选下没有匹配文件" description="可以调整状态筛选、搜索词，或重新扫描设备。" />
                ) : (
                  <div className="storage-table-wrap">
                    <table className="file-table storage-table import-file-table">
                      <thead>
                        <tr>
                          <th scope="col">文件</th>
                          <th scope="col">类型</th>
                          <th scope="col">大小</th>
                          <th scope="col">目标端</th>
                          <th scope="col">状态</th>
                        </tr>
                      </thead>
                      <tbody>
                        {filteredSessionFiles.map((file) => (
                          <tr key={file.id}>
                            <td>
                              <div className="inline-file">
                                <strong>{file.name}</strong>
                                <span>{file.relativePath}</span>
                              </div>
                            </td>
                            <td>{file.fileKind}</td>
                            <td>{file.size}</td>
                            <td>
                              <div className="checkbox-row">
                                {availableTargets.map((target) => {
                                  const checked = file.targetEndpointIds.includes(target.id);
                                  const disabled = file.status === '已跳过';
                                  return (
                                    <label
                                      key={`${file.id}-${target.id}`}
                                      className={`target-check${checked ? ' checked' : ''}${disabled ? ' disabled' : ''}`}
                                    >
                                      <input
                                        checked={checked}
                                        disabled={disabled}
                                        type="checkbox"
                                        onChange={() => {
                                          const nextTargets = checked
                                            ? file.targetEndpointIds.filter((targetId) => targetId !== target.id)
                                            : [...file.targetEndpointIds, target.id];
                                          onSetSourceTargets(file.id, nextTargets);
                                        }}
                                      />
                                      <span>{target.label}</span>
                                    </label>
                                  );
                                })}
                              </div>
                            </td>
                            <td>
                              <TonePill tone={resolveImportFileStatusTone(file.status)}>{file.status}</TonePill>
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
                      <span>成功文件</span>
                      <strong>{selectedReport.successCount}</strong>
                    </div>
                    <div className="import-overview-metric">
                      <span>失败文件</span>
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
                    <ActionButton onClick={() => onOpenFileCenter(selectedDevice.libraryId)}>去文件中心</ActionButton>
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
                        : '预检通过后即可提交当前设备会话。'}
                  </span>
                </div>
                <div className="toolbar-group wrap">
                  <ActionButton onClick={() => selectedDraft && onSaveDraft(selectedDraft.id)}>保存草稿</ActionButton>
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
            {precheckDraft.precheckSummary.items.map((item) => (
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
          <p className="muted-paragraph">提交后会正式创建导入作业，并把运行期管理交给任务中心继续承接。</p>
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
