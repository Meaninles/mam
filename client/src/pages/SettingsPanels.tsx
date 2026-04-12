import type {
  ImportDeviceSessionRecord,
  ImportReportSnapshot,
  IssueRecord,
  NoticeRecord,
  SettingSection,
  TaskRecord,
} from '../data';
import { DenseRow, InlineSettingControl, TonePill } from '../components/Shared';
import { ActionButton } from '../components/Shared';
import type { RuntimeComponentRecord } from '../lib/integrationsApi';

type SectionChangeHandler = (sectionId: string, rowId: string, value: string) => void;

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
              <p className="muted-paragraph">用于直观确认默认打开页、保活和跳转聚焦策略与现有客户端工作区风格一致。</p>
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
                <p>切换到其它一级页面后继续保留当前页面的筛选、分页、搜索与草稿上下文。</p>
              </article>
            ))}
          </div>
        </section>
      </div>
    </section>
  );
}

export function ImportArchiveSettingsPanel({
  cd2Gateway,
  cd2Runtime,
  deviceSessions,
  onChangeCD2Gateway,
  onSaveCD2Gateway,
  reports,
  sections,
  onTestCD2Gateway,
  onChangeSetting,
}: {
  cd2Gateway: {
    baseUrl: string;
    enabled: boolean;
    password: string;
    runtimeStatus: string;
    saving: boolean;
    testing: boolean;
    username: string;
  };
  cd2Runtime: RuntimeComponentRecord | null;
  deviceSessions: ImportDeviceSessionRecord[];
  onChangeCD2Gateway: (field: 'baseUrl' | 'username' | 'password' | 'enabled', value: string | boolean) => void;
  onSaveCD2Gateway: () => void;
  reports: ImportReportSnapshot[];
  sections: SettingSection[];
  onTestCD2Gateway: () => void;
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
              <p className="muted-paragraph">展示当前真实导入会话与导入报告，帮助确认默认策略对预检、目标编排和结果摘要的影响。</p>
            </div>
          </header>
          <article className="settings-preview-item">
            <div className="settings-preview-head">
              <strong>CloudDrive2 集成</strong>
              <TonePill tone={resolveGatewayTone(cd2Runtime?.status ?? cd2Gateway.runtimeStatus)}>
                {renderGatewayStatus(cd2Runtime?.status ?? cd2Gateway.runtimeStatus)}
              </TonePill>
            </div>
            <div className="setting-list">
              <div className="setting-row editable">
                <div className="setting-copy">
                  <span>服务地址</span>
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
                </div>
                <input
                  aria-label="CloudDrive2 密码"
                  className="setting-input"
                  type="password"
                  value={cd2Gateway.password}
                  onChange={(event) => onChangeCD2Gateway('password', event.target.value)}
                />
              </div>
              <div className="setting-row editable">
                <div className="setting-copy">
                  <span>启用集成</span>
                </div>
                <button
                  aria-label="CloudDrive2 启用状态"
                  className={`toggle-button${cd2Gateway.enabled ? ' active' : ''}`}
                  type="button"
                  onClick={() => onChangeCD2Gateway('enabled', !cd2Gateway.enabled)}
                >
                  {cd2Gateway.enabled ? '开启' : '关闭'}
                </button>
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
            <div className="page-stack" style={{ marginTop: 12 }}>
              <DenseRow label="运行状态" value={cd2Runtime?.message ?? '尚未检测'} />
              {cd2Runtime?.lastCheckedAt ? <DenseRow label="最近检测" value={cd2Runtime.lastCheckedAt} /> : null}
            </div>
          </article>
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
  return 'critical' as const;
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
              <p className="muted-paragraph">直接复用当前通知中心中的 mock 通知，便于检查不同提醒口径下的表现是否符合预期。</p>
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
              <p className="muted-paragraph">复用异常中心现有状态组合，帮助检查保留策略与历史清理口径。</p>
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
              <p className="muted-paragraph">复用任务中心已有 mock 数据，覆盖扫描、解析、校验与删除清理几类后台任务组合。</p>
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
