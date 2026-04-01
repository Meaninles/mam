import type { TaskItemRecord, TaskRecord, TaskTab } from '../data';
import {
  ActionButton,
  DenseRow,
  EmptyState,
  ProgressBar,
  SelectPill,
  Sheet,
  TabSwitch,
  TonePill,
} from '../components/Shared';

export function TaskCenterPage(props: {
  activeTab: TaskTab;
  statusFilter: string;
  tasks: TaskRecord[];
  onChangeTaskStatus: (id: string, action: 'pause' | 'resume' | 'retry' | 'complete') => void;
  onOpenTaskDetail: (value: TaskRecord | null) => void;
  onSetActiveTab: (value: TaskTab) => void;
  onSetTaskStatusFilter: (value: string) => void;
}) {
  const { activeTab, statusFilter, tasks, onChangeTaskStatus, onOpenTaskDetail, onSetActiveTab, onSetTaskStatusFilter } =
    props;

  return (
    <section className="page-stack">
      <div className="toolbar-card action-toolbar">
        <div className="toolbar-group wrap">
          <TabSwitch
            items={[
              { id: 'transfer', label: '传输任务' },
              { id: 'other', label: '其它任务' },
            ]}
            value={activeTab}
            onChange={(value) => onSetActiveTab(value as TaskTab)}
          />
          <SelectPill
            ariaLabel="任务状态"
            options={['全部', '运行中', '等待确认', '同步中', '暂停中', '失败', '已完成']}
            value={statusFilter}
            onChange={onSetTaskStatusFilter}
          />
        </div>
      </div>

      <div className="workspace-card compact-list">
        {tasks.length === 0 ? (
          <EmptyState title="当前没有任务" description="可以去导入中心提交批次，或在异常中心发起修复任务。" />
        ) : (
          tasks.map((task) => (
            <article className="list-row task-row" key={task.id}>
              <div className="row-main">
                <strong>{task.title}</strong>
                <span>
                  {task.type} · {task.source ? `${task.source} → ${task.target}` : task.type}
                </span>
                <p>最近更新：{task.updatedAt}</p>
              </div>
              <span>{task.fileCount} 项</span>
              <span>{task.speed}</span>
              <span>{task.eta}</span>
              <TonePill tone={task.statusTone}>{task.status}</TonePill>
              <div className="row-progress">
                <ProgressBar value={task.progress} />
              </div>
              <div className="row-actions">
                <ActionButton onClick={() => onOpenTaskDetail(task)}>详情</ActionButton>
                {task.status === '失败' ? (
                  <ActionButton onClick={() => onChangeTaskStatus(task.id, 'retry')}>重试</ActionButton>
                ) : null}
                {task.status === '暂停中' ? (
                  <ActionButton onClick={() => onChangeTaskStatus(task.id, 'resume')}>继续</ActionButton>
                ) : null}
                {['运行中', '同步中'].includes(task.status) ? (
                  <ActionButton onClick={() => onChangeTaskStatus(task.id, 'pause')}>暂停</ActionButton>
                ) : null}
                {task.progress < 100 ? (
                  <ActionButton onClick={() => onChangeTaskStatus(task.id, 'complete')}>标记完成</ActionButton>
                ) : null}
              </div>
            </article>
          ))
        )}
      </div>
    </section>
  );
}

export function TaskDetailSheet({
  item,
  items,
  onClose,
}: {
  item: TaskRecord;
  items: TaskItemRecord[];
  onClose: () => void;
}) {
  return (
    <Sheet onClose={onClose} title={item.title}>
      <div className="sheet-section">
        <DenseRow label="类型" value={item.type} />
        <DenseRow label="状态" tone={item.statusTone} value={item.status} />
        <DenseRow label="源端" value={item.source ?? '—'} />
        <DenseRow label="目标端" value={item.target ?? '—'} />
        <DenseRow label="总文件数" value={`${item.fileCount}`} />
        <DenseRow label="最近更新" value={item.updatedAt} />
      </div>
      <div className="workspace-card compact-list inner-list">
        {items.length === 0 ? (
          <EmptyState title="暂无子任务" description="当前任务没有拆分的文件级记录。" />
        ) : (
          items.map((taskItem) => (
            <div className="list-row task-detail-row" key={taskItem.id}>
              <div className="row-main">
                <strong>{taskItem.name}</strong>
                <span>{taskItem.size}</span>
              </div>
              <span>{taskItem.speed}</span>
              <TonePill tone={taskItem.statusTone}>{taskItem.status}</TonePill>
              <div className="row-progress">
                <ProgressBar value={taskItem.progress} />
              </div>
            </div>
          ))
        )}
      </div>
    </Sheet>
  );
}
