import { useEffect, useMemo, useState } from 'react';
import { CircleEllipsis, Search, Settings2 } from 'lucide-react';
import type { NoticeJumpTargetKind, NoticeRecord } from '../data';
import { ActionButton, EmptyState, IconButton, SelectPill } from '../components/Shared';
import { resolveCloudIssueTargetKind, resolveNoticeCloudKind } from '../lib/cloudIssueRouting';
import { getNoticeKindLabel, getNoticePrimaryActionLabel, getNoticeStatusLabel, getVisibleNoticeRecords } from '../lib/noticeCenter';

type NoticeFilterValue = 'ALL' | 'UNCONSUMED' | 'ACTION_REQUIRED' | 'REMINDER';

const FILTER_OPTIONS: Array<{ value: NoticeFilterValue; label: string }> = [
  { value: 'ALL', label: '全部通知' },
  { value: 'UNCONSUMED', label: '未消费' },
  { value: 'ACTION_REQUIRED', label: '处置类' },
  { value: 'REMINDER', label: '提醒类' },
];

export function NotificationCenterSheet({
  noticeRecords,
  onMarkRead,
  onOpenTarget,
}: {
  noticeRecords: NoticeRecord[];
  onMarkRead: (noticeId: string) => void;
  onOpenTarget: (notice: NoticeRecord, targetKind?: NoticeJumpTargetKind) => void;
}) {
  const [filterValue, setFilterValue] = useState<NoticeFilterValue>('ALL');
  const [searchText, setSearchText] = useState('');
  const [menuNoticeId, setMenuNoticeId] = useState<string | null>(null);

  useEffect(() => {
    if (!menuNoticeId) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      if (target.closest('.notification-center-menu')) return;
      if (target.closest('.notification-center-row-actions')) return;
      setMenuNoticeId(null);
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, [menuNoticeId]);

  const visibleRecords = useMemo(() => getVisibleNoticeRecords(noticeRecords), [noticeRecords]);

  const filteredRecords = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();

    return visibleRecords.filter((record) => {
      const matchesFilter =
        filterValue === 'ALL'
          ? true
          : filterValue === 'UNCONSUMED'
            ? record.status === 'UNREAD'
            : filterValue === 'ACTION_REQUIRED'
              ? record.kind === 'ACTION_REQUIRED'
              : record.kind === 'REMINDER';

      const matchesSearch = keyword
        ? [record.title, record.summary, record.objectLabel, record.source.sourceLabel, record.source.routeLabel]
            .filter(Boolean)
            .join(' ')
            .toLowerCase()
            .includes(keyword)
        : true;

      return matchesFilter && matchesSearch;
    });
  }, [filterValue, searchText, visibleRecords]);

  return (
    <div className="notification-center-sheet">
      <div className="toolbar-card notification-center-toolbar">
        <div className="notification-center-toolbar-row">
          <label className="search-field notification-search-field" htmlFor="notification-search">
            <Search size={14} />
            <input
              id="notification-search"
              aria-label="通知搜索"
              placeholder="搜索标题、对象、来源"
              type="search"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
          <div className="notification-filter-pill">
            <SelectPill
              ariaLabel="通知筛选"
              options={FILTER_OPTIONS.map((option) => option.label)}
              value={FILTER_OPTIONS.find((option) => option.value === filterValue)?.label ?? FILTER_OPTIONS[0].label}
              onChange={(label) => setFilterValue(FILTER_OPTIONS.find((option) => option.label === label)?.value ?? 'ALL')}
            />
          </div>
        </div>
      </div>

      <div className="workspace-card compact-list inner-list notification-center-list">
        {filteredRecords.length === 0 ? (
          <EmptyState title="当前没有可展示的通知" description="可以调整筛选条件，或等待新的提醒与处置通知进入通知中心。" />
        ) : (
          filteredRecords.map((record) => {
            const primaryActionLabel = getNoticePrimaryActionLabel(record);
            const isMenuOpen = menuNoticeId === record.id;
            const preferredTarget = resolveNoticePreferredTarget(record);

            return (
              <article className={`notification-center-card${record.status === 'UNREAD' ? ' unread' : ''}`} key={record.id}>
                <div className="notification-center-card-main">
                  <strong className="notification-center-title" title={record.title}>
                    {record.title}
                  </strong>
                  <div className="visually-hidden" aria-hidden="true">
                    <span>{getNoticeKindLabel(record.kind)}</span>
                    <span>{getNoticeStatusLabel(record)}</span>
                  </div>

                  <p className="notification-center-summary" title={record.summary}>
                    {record.summary}
                  </p>

                  <div className="notification-center-meta">
                    <span className="notification-center-object" title={record.objectLabel}>
                      {record.objectLabel}
                    </span>
                    {record.source.sourceLabel ? (
                      <span className="notification-center-route" title={record.source.sourceLabel}>
                        {record.source.sourceLabel}
                      </span>
                    ) : null}
                  </div>
                </div>

                <div className="notification-center-footer">
                  <span className="notification-time">{record.updatedAt}</span>

                  <div className="notification-center-row-actions">
                    <ActionButton
                      ariaLabel={`${primaryActionLabel} ${record.title}`}
                      tone={record.kind === 'ACTION_REQUIRED' ? 'primary' : 'default'}
                      onClick={() => onOpenTarget(record, preferredTarget)}
                    >
                      {primaryActionLabel}
                    </ActionButton>

                    <div className="notification-center-menu-anchor">
                      <IconButton
                        ariaLabel={`更多操作 ${record.title}`}
                        tooltip="更多操作"
                        onClick={() => setMenuNoticeId((current) => (current === record.id ? null : record.id))}
                      >
                        <CircleEllipsis size={16} />
                      </IconButton>

                      {isMenuOpen ? (
                        <div className="context-menu notification-center-menu">
                          {record.capabilities.canMarkRead && record.status === 'UNREAD' ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onMarkRead(record.id);
                              }}
                            >
                              标记已读
                            </button>
                          ) : null}
                          {shouldOpenSettingsForNotice(record) ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onOpenTarget(record, 'settings');
                              }}
                            >
                              打开设置页
                            </button>
                          ) : null}
                          {record.capabilities.canOpenIssueCenter ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onOpenTarget(record, 'issues');
                              }}
                            >
                              打开异常中心
                            </button>
                          ) : null}
                          {record.capabilities.canOpenTaskCenter ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onOpenTarget(record, 'task-center');
                              }}
                            >
                              打开任务中心
                            </button>
                          ) : null}
                          {record.capabilities.canOpenFileCenter ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onOpenTarget(record, 'file-center');
                              }}
                            >
                              打开文件中心
                            </button>
                          ) : null}
                          {record.capabilities.canOpenStorageNodes || preferredTarget === 'storage-nodes' ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onOpenTarget(record, 'storage-nodes');
                              }}
                            >
                              打开存储节点
                            </button>
                          ) : null}
                          {record.capabilities.canOpenImportCenter ? (
                            <button
                              type="button"
                              onClick={() => {
                                setMenuNoticeId(null);
                                onOpenTarget(record, 'import-center');
                              }}
                            >
                              打开导入中心
                            </button>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  </div>
                </div>
              </article>
            );
          })
        )}
      </div>
    </div>
  );
}

function resolveNoticePreferredTarget(record: NoticeRecord): NoticeJumpTargetKind {
  const kind = resolveNoticeCloudKind(record);
  return resolveCloudIssueTargetKind(kind, record.jumpParams.kind);
}

function shouldOpenSettingsForNotice(record: NoticeRecord) {
  return resolveNoticePreferredTarget(record) === 'settings';
}
