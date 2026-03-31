import type { IssueRecord } from '../data';
import { SelectPill, TonePill } from '../components/Shared';

export function IssuesPage({
  issueTypeFilter,
  items,
  setIssueTypeFilter,
}: {
  issueTypeFilter: string;
  items: IssueRecord[];
  setIssueTypeFilter: (value: string) => void;
}) {
  return (
    <section className="page-stack">
      <div className="toolbar-card">
        <div className="toolbar-group">
          <SelectPill
            ariaLabel="异常类型"
            options={['全部', '路径冲突', '校验失败', '鉴权提醒', '历史临时文件', '空间不足']}
            value={issueTypeFilter}
            onChange={setIssueTypeFilter}
          />
        </div>
      </div>

      <div className="workspace-card compact-list">
        {items.map((item) => (
          <div className="list-row" key={item.id}>
            <div className="row-main">
              <strong>{item.asset}</strong>
              <span>{item.type}</span>
            </div>
            <span>{item.status}</span>
            <span>{item.action}</span>
            <TonePill tone={item.severity}>{item.severity === 'critical' ? '高优先级' : item.severity === 'warning' ? '需处理' : '信息'}</TonePill>
          </div>
        ))}
      </div>
    </section>
  );
}
