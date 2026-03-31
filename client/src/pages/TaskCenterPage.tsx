import type { TaskItemRecord, TaskRecord, TaskTab } from '../data';
import type { ContextMenuTarget } from '../App';
import { DenseRow, ProgressBar, SelectPill, Sheet, TabSwitch, TonePill } from '../components/Shared';

export function TaskCenterPage(props: {
  activeTab: TaskTab;
  setActiveTab: (value: TaskTab) => void;
  setContextMenu: (value: ContextMenuTarget) => void;
  setTaskDetail: (value: TaskRecord | null) => void;
  setTaskStatusFilter: (value: string) => void;
  statusFilter: string;
  tasks: TaskRecord[];
}) {
  const { activeTab, setActiveTab, setContextMenu, setTaskDetail, setTaskStatusFilter, statusFilter, tasks } = props;

  return (
    <section className="page-stack">
      <div className="toolbar-card">
        <div className="toolbar-group">
          <TabSwitch
            items={[
              { id: 'transfer', label: '传输任务' },
              { id: 'other', label: '其它任务' },
            ]}
            value={activeTab}
            onChange={(value) => setActiveTab(value as TaskTab)}
          />
          <SelectPill
            ariaLabel="任务状态"
            options={['全部', '运行中', '等待确认', '同步中', '暂停中', '失败', '已完成']}
            value={statusFilter}
            onChange={setTaskStatusFilter}
          />
        </div>
      </div>

      <div className="workspace-card compact-list">
        {tasks.map((task) => (
          <div
            className={`list-row${task.multiFile ? ' enterable' : ''}`}
            key={task.id}
            onContextMenu={(event) => {
              if (!task.multiFile) return;
              event.preventDefault();
              setContextMenu({ type: 'task', item: task, x: event.clientX, y: event.clientY });
            }}
            onDoubleClick={() => {
              if (task.multiFile) setTaskDetail(task);
            }}
          >
            <div className="row-main">
              <strong>{task.title}</strong>
              <span>
                {task.type} · {task.source ? `${task.source} → ${task.target}` : task.type}
              </span>
            </div>
            <span>{task.fileCount} 项</span>
            <span>{task.speed}</span>
            <span>{task.eta}</span>
            <TonePill tone={task.statusTone}>{task.status}</TonePill>
            <div className="row-progress">
              <ProgressBar value={task.progress} />
            </div>
          </div>
        ))}
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
      </div>
      <div className="workspace-card compact-list inner-list">
        {items.map((taskItem) => (
          <div className="list-row" key={taskItem.id}>
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
        ))}
      </div>
    </Sheet>
  );
}
