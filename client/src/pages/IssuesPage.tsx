import type { IssueRecord } from '../data';
import { ActionButton, EmptyState, SelectPill, TonePill } from '../components/Shared';

export function IssuesPage({
  issueTypeFilter,
  items,
  onIgnoreIssue,
  onResolveIssue,
  setIssueTypeFilter,
}: {
  issueTypeFilter: string;
  items: IssueRecord[];
  onIgnoreIssue: (id: string) => void;
  onResolveIssue: (id: string) => void;
  setIssueTypeFilter: (value: string) => void;
}) {
  return (
    <section className="page-stack">
      <div className="toolbar-card action-toolbar">
        <SelectPill
          ariaLabel="异常类型"
          options={['全部', '路径冲突', '校验失败', '鉴权提醒', '历史临时文件', '空间不足']}
          value={issueTypeFilter}
          onChange={setIssueTypeFilter}
        />
      </div>

      <div className="workspace-card compact-list">
        {items.length === 0 ? (
          <EmptyState title="当前没有异常" description="所有异常都已处理完成，当前筛选结果为空。" />
        ) : (
          items.map((item) => (
            <article className="list-row issue-row" key={item.id}>
              <div className="row-main">
                <strong>{item.asset}</strong>
                <span>{item.type}</span>
                <p>{item.detail}</p>
              </div>
              <span>{item.status}</span>
              <span>{item.action}</span>
              <TonePill tone={item.severity}>
                {item.severity === 'critical' ? '高优先级' : item.severity === 'warning' ? '需尽快处理' : item.severity === 'success' ? '已完成' : '信息'}
              </TonePill>
              <div className="row-actions">
                <ActionButton ariaLabel={`按建议处理 ${item.asset}`} onClick={() => onResolveIssue(item.id)}>
                  按建议处理
                </ActionButton>
                <ActionButton onClick={() => onIgnoreIssue(item.id)}>忽略</ActionButton>
              </div>
            </article>
          ))
        )}
      </div>
    </section>
  );
}
