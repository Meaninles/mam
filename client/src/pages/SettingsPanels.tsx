import type {
  ImportDeviceSessionRecord,
  ImportReportSnapshot,
  IssueRecord,
  NoticeRecord,
  SettingSection,
  TaskRecord,
} from '../data';
import { ActionButton, DenseRow, InlineSettingControl, TonePill } from '../components/Shared';
import type { RuntimeComponentRecord } from '../lib/integrationsApi';

type SectionChangeHandler = (sectionId: string, rowId: string, value: string) => void;
type DependencyIndicatorTone = 'success' | 'warning' | 'critical';

export function WorkspaceSettingsPanel({
  activeViewLabel,
  openViews,
  sections,
  onChangeSetting,
}: {
  activeViewLabel: string;
  openViews: string[];
  sections: SettingSection[];
  onChangeSetting: SectionChangeHandler;
}) {
  return (
    <section className="page-stack settings-strategy-page">
      <div className="settings-strategy-layout">
        <div className="page-stack">
          <SettingsSectionCards onChangeSetting={onChangeSetting} sections={sections} />
        </div>
        <section className="workspace-card settings-preview-card">
          <header className="section-header">
            <div>
              <strong>当前工作区预览</strong>
              <p className="muted-paragraph">用于直观确认默认打开页、会话保活和跳转聚焦策略是否符合当前工作方式。</p>
            </div>
          </header>
          <div className="settings-preview-list">
            {openViews.map((label) => (
              <article className="settings-preview-item" key={label}>
                <div className="settings-preview-head">
                  <strong>{label}</strong>
                  <TonePill tone={label === activeViewLabel ? 'success' : 'info'}>
                    {label === activeViewLabel ? '当前激活' : '会话保活'}
                  </TonePill>
                </div>
                <p>切换到其他一级页面后，继续保留当前页面的筛选、分页、搜索和焦点上下文。</p>
              </article>
            ))}
          </div>
        </section>
      </div>
    </section>
  );
}

export function ImportArchiveSettingsPanel({
  deviceSessions,
  reports,
  sections,
  onChangeSetting,
}: {
  deviceSessions: ImportDeviceSessionRecord[];
  reports: ImportReportSnapshot[];
  sections: SettingSection[];
  onChangeSetting: SectionChangeHandler;
}) {
  const activeSessions = deviceSessions.filter((session) => session.sessionStatus !== '已拔出');

  return (
    <section className="page-stack settings-strategy-page">
      <div className="settings-strategy-layout">
        <div className="page-stack">
          <SettingsSectionCards onChangeSetting={onChangeSetting} sections={sections} />
        </div>
        <section className="workspace-card settings-preview-card">
          <header className="section-header">
            <div>
              <strong>导入会话示例</strong>
              <p className="muted-paragraph">展示当前真实导入会话与导入报告，便于检查预检、目标编排和结果摘要的默认口径。</p>
            </div>
          </header>
          <div className="settings-preview-list">
            {activeSessions.slice(0, 3).map((session) => (
              <article className="settings-preview-item" key={session.id}>
                <div className="settings-preview-head">
                  <strong>{session.deviceLabel}</strong>
                  <TonePill tone={resolveImportSessionTone(session.sessionStatus)}>{session.sessionStatus}</TonePill>
                </div>
                <p>{session.description}</p>
                <div className="endpoint-row">
                  <span>{session.fileCount} 个文件</span>
                  <span>{session.availableTargetEndpointIds.length} 个目标端</span>
                </div>
              </article>
            ))}
            {reports.slice(0, 2).map((report) => (
              <article className="settings-preview-item subtle" key={report.id}>
                <div className="settings-preview-head">
                  <strong>{report.title}</strong>
                  <TonePill tone={resolveReportTone(report.status)}>{report.status}</TonePill>
                </div>
                <p>{report.verifySummary}</p>
                <div className="endpoint-row">
                  <span>成功 {report.successCount}</span>
                  <span>失败 {report.failedCount}</span>
                </div>
              </article>
            ))}
          </div>
        </section>
      </div>
    </section>
  );
}

export function DependencyServicesSettingsPanel({
  aria2Runtime,
  cd2Gateway,
  cd2Runtime,
  onChangeCD2Gateway,
  onSaveCD2Gateway,
  onTestCD2Gateway,
}: {
  aria2Runtime: RuntimeComponentRecord | null;
  cd2Gateway: {
    baseUrl: string;
    hasPassword: boolean;
    password: string;
    runtimeStatus: string;
    saving: boolean;
    testing: boolean;
    username: string;
  };
  cd2Runtime: RuntimeComponentRecord | null;
  onChangeCD2Gateway: (field: 'baseUrl' | 'username' | 'password', value: string) => void;
  onSaveCD2Gateway: () => void;
  onTestCD2Gateway: () => void;
}) {
  const cd2Indicator = resolveDependencyIndicator({
    message: cd2Runtime?.message,
    serviceName: 'CloudDrive2',
    status: cd2Runtime?.status ?? cd2Gateway.runtimeStatus,
  });
  const aria2Indicator = resolveDependencyIndicator({
    message: aria2Runtime?.message,
    serviceName: 'aria2',
    status: aria2Runtime?.status,
  });

  return (
    <section className="page-stack settings-strategy-page dependency-services-page">
      <div className="settings-strategy-layout">
        <div className="page-stack dependency-services-stack">
          <section className="content-card dependency-service-card">
            <header className="section-header dependency-service-card-header">
              <div className="dependency-service-title">
                <div
                  aria-label={`CloudDrive2 状态：${cd2Indicator.label}`}
                  className={`system-runtime-indicator has-status-tooltip ${cd2Indicator.tone}`}
                  data-testid="dependency-service-indicator-CloudDrive2"
                  data-tooltip={`CloudDrive2：${cd2Indicator.tooltip}`}
                />
                <div>
                  <strong>CloudDrive2</strong>
                  <p className="muted-paragraph">负责 115 网盘鉴权、目录校验和远程上传编排。</p>
                </div>
              </div>
              <TonePill tone={resolveGatewayTone(cd2Runtime?.status ?? cd2Gateway.runtimeStatus)}>
                {renderGatewayStatus(cd2Runtime?.status ?? cd2Gateway.runtimeStatus)}
              </TonePill>
            </header>

            <div className="setting-list">
              <div className="setting-row editable">
                <div className="setting-copy">
                  <span>服务地址</span>
                  <small>中心服务通过该地址访问 CloudDrive2。</small>
                </div>
                <input
                  aria-label="CloudDrive2 服务地址"
                  className="setting-input"
                  type="text"
                  value={cd2Gateway.baseUrl}
                  onChange={(event) => onChangeCD2Gateway('baseUrl', event.target.value)}
                />
              </div>
              <div className="setting-row editable">
                <div className="setting-copy">
                  <span>账号</span>
                </div>
                <input
                  aria-label="CloudDrive2 账号"
                  className="setting-input"
                  type="text"
                  value={cd2Gateway.username}
                  onChange={(event) => onChangeCD2Gateway('username', event.target.value)}
                />
              </div>
              <div className="setting-row editable">
                <div className="setting-copy">
                  <span>密码</span>
                  <small>如已保存凭据，可留空以保持现有密码。</small>
                </div>
                <input
                  aria-label="CloudDrive2 密码"
                  className="setting-input"
                  type="password"
                  value={cd2Gateway.password}
                  onChange={(event) => onChangeCD2Gateway('password', event.target.value)}
                />
              </div>
            </div>

            <div className="toolbar-group wrap">
              <ActionButton disabled={cd2Gateway.testing} onClick={onTestCD2Gateway}>
                {cd2Gateway.testing ? '测试中…' : '连接测试'}
              </ActionButton>
              <ActionButton disabled={cd2Gateway.saving} tone="primary" onClick={onSaveCD2Gateway}>
                {cd2Gateway.saving ? '保存中…' : '保存 CloudDrive2'}
              </ActionButton>
            </div>

            <div className="page-stack dependency-service-meta">
              <DenseRow label="凭据状态" value={cd2Gateway.hasPassword ? '已保存凭据' : '尚未保存'} />
              {cd2Runtime?.lastCheckedAt ? <DenseRow label="最近检测" value={formatSettingsTimeLabel(cd2Runtime.lastCheckedAt)} /> : null}
              {cd2Runtime?.lastErrorCode ? <DenseRow label="最近错误代码" tone="critical" value={cd2Runtime.lastErrorCode} /> : null}
              {cd2Runtime?.lastErrorMessage ? <DenseRow label="最近错误" tone="critical" value={cd2Runtime.lastErrorMessage} /> : null}
            </div>
          </section>

          <section className="content-card dependency-service-card">
            <header className="section-header dependency-service-card-header">
              <div className="dependency-service-title">
                <div
                  aria-label={`aria2 状态：${aria2Indicator.label}`}
                  className={`system-runtime-indicator has-status-tooltip ${aria2Indicator.tone}`}
                  data-testid="dependency-service-indicator-aria2"
                  data-tooltip={`aria2：${aria2Indicator.tooltip}`}
                />
                <div>
                  <strong>aria2</strong>
                  <p className="muted-paragraph">负责从 115 下行到本地或 NAS 的下载执行。</p>
                </div>
              </div>
              <TonePill tone={resolveGatewayTone(aria2Runtime?.status ?? 'UNKNOWN')}>
                {renderGatewayStatus(aria2Runtime?.status ?? 'UNKNOWN')}
              </TonePill>
            </header>

            <div className="page-stack dependency-service-meta">
              <DenseRow label="配置方式" value="由中心服务托管" />
              {aria2Runtime?.lastCheckedAt ? <DenseRow label="最近检测" value={formatSettingsTimeLabel(aria2Runtime.lastCheckedAt)} /> : null}
              {aria2Runtime?.lastErrorCode ? <DenseRow label="最近错误代码" tone="critical" value={aria2Runtime.lastErrorCode} /> : null}
              {aria2Runtime?.lastErrorMessage ? <DenseRow label="最近错误" tone="critical" value={aria2Runtime.lastErrorMessage} /> : null}
            </div>
          </section>
        </div>

        <section className="workspace-card settings-preview-card dependency-service-summary-card">
          <header className="section-header">
            <div>
              <strong>依赖服务状态</strong>
            </div>
          </header>
          <div className="settings-preview-list">
            {[
              {
                indicator: cd2Indicator,
                name: 'CloudDrive2',
                runtime: cd2Runtime,
              },
              {
                indicator: aria2Indicator,
                name: 'aria2',
                runtime: aria2Runtime,
              },
            ].map((service) => (
              <article className="settings-preview-item dependency-service-summary-item" key={service.name}>
                <div className="settings-preview-head dependency-service-summary-head">
                  <div className="dependency-service-title">
                    <div
                      aria-label={`${service.name} 状态：${service.indicator.label}`}
                      className={`system-runtime-indicator has-status-tooltip ${service.indicator.tone}`}
                      data-tooltip={`${service.name}：${service.indicator.tooltip}`}
                    />
                    <strong>{service.name}</strong>
                  </div>
                  <TonePill tone={resolveGatewayTone(service.runtime?.status ?? 'UNKNOWN')}>
                    {renderGatewayStatus(service.runtime?.status ?? 'UNKNOWN')}
                  </TonePill>
                </div>
                <p>{service.runtime?.message ?? `${service.name} 尚未检测`}</p>
                {service.runtime?.lastCheckedAt ? (
                  <div className="endpoint-row">
                    <span>最近检测</span>
                    <span>{formatSettingsTimeLabel(service.runtime.lastCheckedAt)}</span>
                  </div>
                ) : null}
              </article>
            ))}
          </div>
        </section>
      </div>
    </section>
  );
}

export function NotificationSettingsPanel({
  notices,
  sections,
  onChangeSetting,
}: {
  notices: NoticeRecord[];
  sections: SettingSection[];
  onChangeSetting: SectionChangeHandler;
}) {
  return (
    <section className="page-stack settings-strategy-page">
      <div className="settings-strategy-layout">
        <div className="page-stack">
          <SettingsSectionCards onChangeSetting={onChangeSetting} sections={sections} />
        </div>
        <section className="workspace-card settings-preview-card">
          <header className="section-header">
            <div>
              <strong>通知示例</strong>
              <p className="muted-paragraph">直接复用当前通知中心的数据，检查不同提醒和处置场景下的默认表现。</p>
            </div>
          </header>
          <div className="settings-preview-list">
            {notices.slice(0, 4).map((notice) => (
              <article className="settings-preview-item" key={notice.id}>
                <div className="settings-preview-head">
                  <strong>{notice.title}</strong>
                  <TonePill tone={notice.kind === 'ACTION_REQUIRED' ? 'warning' : 'info'}>
                    {notice.kind === 'ACTION_REQUIRED' ? '处置类' : '提醒类'}
                  </TonePill>
                </div>
                <p>{notice.summary}</p>
                <div className="endpoint-row">
                  <span>{notice.objectLabel}</span>
                  <span>{renderNoticeStatus(notice.status)}</span>
                </div>
              </article>
            ))}
          </div>
        </section>
      </div>
    </section>
  );
}

export function IssueGovernanceSettingsPanel({
  issues,
  sections,
  onChangeSetting,
}: {
  issues: IssueRecord[];
  sections: SettingSection[];
  onChangeSetting: SectionChangeHandler;
}) {
  return (
    <section className="page-stack settings-strategy-page">
      <div className="settings-strategy-layout">
        <div className="page-stack">
          <SettingsSectionCards onChangeSetting={onChangeSetting} sections={sections} />
        </div>
        <section className="workspace-card settings-preview-card">
          <header className="section-header">
            <div>
              <strong>治理快照</strong>
              <p className="muted-paragraph">复用异常中心现有问题组合，检查保留策略和历史清理口径。</p>
            </div>
          </header>
          <div className="settings-preview-list">
            {issues.slice(0, 4).map((issue) => (
              <article className="settings-preview-item" key={issue.id}>
                <div className="settings-preview-head">
                  <strong>{issue.title}</strong>
                  <TonePill tone={resolveIssueTone(issue.status, issue.nature)}>{issue.status}</TonePill>
                </div>
                <p>{issue.summary}</p>
                <div className="endpoint-row">
                  <span>{issue.category}</span>
                  <span>{issue.sourceDomain}</span>
                </div>
              </article>
            ))}
          </div>
          <div className="sheet-section">
            <DenseRow label="当前保留条数" value={getRowValue(sections, 'issue-history-count')} />
            <DenseRow label="当前保留时长" value={getRowValue(sections, 'issue-history-age')} />
            <DenseRow label="自动归档已解决" value={getRowValue(sections, 'issue-auto-archive')} />
          </div>
        </section>
      </div>
    </section>
  );
}

export function BackgroundTaskSettingsPanel({
  sections,
  tasks,
  onChangeSetting,
}: {
  sections: SettingSection[];
  tasks: TaskRecord[];
  onChangeSetting: SectionChangeHandler;
}) {
  return (
    <section className="page-stack settings-strategy-page">
      <div className="settings-strategy-layout">
        <div className="page-stack">
          <SettingsSectionCards onChangeSetting={onChangeSetting} sections={sections} />
        </div>
        <section className="workspace-card settings-preview-card">
          <header className="section-header">
            <div>
              <strong>后台任务示例</strong>
              <p className="muted-paragraph">复用任务中心中的后台任务数据，覆盖扫描、解析、校验和清理几类组合。</p>
            </div>
          </header>
          <div className="settings-preview-list">
            {tasks
              .filter((task) => task.kind === 'other')
              .slice(0, 4)
              .map((task) => (
                <article className="settings-preview-item" key={task.id}>
                  <div className="settings-preview-head">
                    <strong>{task.title}</strong>
                    <TonePill tone={task.statusTone}>{task.status}</TonePill>
                  </div>
                  <p>{task.resultSummary ?? task.scopeLabel ?? task.source ?? '后台任务运行中'}</p>
                  <div className="endpoint-row">
                    <span>{task.otherTaskType ?? task.type}</span>
                    <span>{task.phaseLabel ?? task.eta}</span>
                  </div>
                </article>
              ))}
          </div>
        </section>
      </div>
    </section>
  );
}

function SettingsSectionCards({
  sections,
  onChangeSetting,
}: {
  sections: SettingSection[];
  onChangeSetting: SectionChangeHandler;
}) {
  return (
    <>
      {sections.map((section) => (
        <section className="content-card" key={section.id}>
          <header className="section-header">
            <strong>{section.title}</strong>
          </header>
          <div className="setting-list">
            {section.rows.map((row) => (
              <div className="setting-row editable" key={row.id}>
                <div className="setting-copy">
                  <span>{row.label}</span>
                  {row.description ? <small>{row.description}</small> : null}
                </div>
                <InlineSettingControl
                  control={row.control}
                  label={row.label}
                  options={row.options}
                  value={row.value}
                  onChange={(value) => onChangeSetting(section.id, row.id, value)}
                />
              </div>
            ))}
          </div>
        </section>
      ))}
    </>
  );
}

function getRowValue(sections: SettingSection[], rowId: string) {
  return sections.flatMap((section) => section.rows).find((row) => row.id === rowId)?.value ?? '—';
}

function renderNoticeStatus(status: NoticeRecord['status']) {
  if (status === 'UNREAD') return '未消费';
  if (status === 'READ') return '已读';
  if (status === 'JUMPED') return '已跳转';
  return '已失效';
}

function resolveImportSessionTone(status: ImportDeviceSessionRecord['sessionStatus']) {
  if (status === '异常待处理') return 'critical';
  if (status === '导入中' || status === '扫描中') return 'warning';
  if (status === '可导入' || status === '部分完成') return 'info';
  return 'success';
}

function resolveReportTone(status: ImportReportSnapshot['status']) {
  if (status === '失败') return 'critical';
  if (status === '部分成功' || status === '运行中') return 'warning';
  if (status === '已排队') return 'info';
  return 'success';
}

function resolveIssueTone(status: IssueRecord['status'], nature: IssueRecord['nature']) {
  if (status === '已归档' || status === '已解决') return 'success';
  if (nature === 'BLOCKING') return 'critical';
  return 'info';
}

function renderGatewayStatus(status: string) {
  if (status === 'ONLINE') return '在线';
  if (status === 'ERROR') return '异常';
  if (status === 'DISABLED') return '已禁用';
  if (status === 'DEGRADED') return '降级';
  return '未检测';
}

function resolveGatewayTone(status: string) {
  if (status === 'ONLINE') return 'success' as const;
  if (status === 'DISABLED') return 'info' as const;
  if (status === 'DEGRADED') return 'warning' as const;
  if (status === 'ERROR') return 'critical' as const;
  return 'warning' as const;
}

function resolveDependencyIndicator({
  message,
  serviceName,
  status,
}: {
  message?: string;
  serviceName: string;
  status?: string;
}): { label: string; tone: DependencyIndicatorTone; tooltip: string } {
  const normalizedMessage = message?.trim();
  const tooltip =
    normalizedMessage && normalizedMessage.startsWith(`${serviceName} `)
      ? normalizedMessage.slice(serviceName.length + 1)
      : normalizedMessage || `${serviceName} 尚未检测`;
  if (status === 'ONLINE') {
    return { label: '在线', tone: 'success', tooltip };
  }
  if (status === 'ERROR') {
    return { label: '异常', tone: 'critical', tooltip };
  }
  if (status === 'DISABLED') {
    return { label: '已禁用', tone: 'warning', tooltip };
  }
  if (status === 'DEGRADED') {
    return { label: '降级', tone: 'warning', tooltip };
  }
  return { label: '未检测', tone: 'warning', tooltip };
}

function formatSettingsTimeLabel(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const targetStart = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.round((todayStart.getTime() - targetStart.getTime()) / (24 * 60 * 60 * 1000));
  const hh = String(date.getHours()).padStart(2, '0');
  const mm = String(date.getMinutes()).padStart(2, '0');

  if (diffDays === 0) {
    return `今天 ${hh}:${mm}`;
  }
  if (diffDays === 1) {
    return `昨天 ${hh}:${mm}`;
  }
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${hh}:${mm}`;
}
