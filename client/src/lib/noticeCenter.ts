import type { NoticeKind, NoticeRecord, NoticeStatus } from '../data';

const MUTATION_TIMESTAMP = '刚刚';

const STATUS_ORDER: Record<NoticeStatus, number> = {
  UNREAD: 0,
  READ: 1,
  JUMPED: 2,
  STALE: 3,
};

const KIND_ORDER: Record<NoticeKind, number> = {
  ACTION_REQUIRED: 0,
  REMINDER: 1,
};

export function getUnconsumedNoticeCount(records: NoticeRecord[]) {
  return records.filter((record) => record.status === 'UNREAD').length;
}

export function getVisibleNoticeRecords(records: NoticeRecord[]) {
  return [...records]
    .filter((record) => record.status !== 'STALE')
    .sort((left, right) => {
      const statusDelta = STATUS_ORDER[left.status] - STATUS_ORDER[right.status];
      if (statusDelta !== 0) {
        return statusDelta;
      }

      const kindDelta = KIND_ORDER[left.kind] - KIND_ORDER[right.kind];
      if (kindDelta !== 0) {
        return kindDelta;
      }

      return right.sortKey - left.sortKey;
    });
}

export function consumeReminderNotices(records: NoticeRecord[]) {
  return records.map((record) =>
    record.kind === 'REMINDER' && record.status === 'UNREAD'
      ? {
          ...record,
          status: 'READ' as const,
          readAt: MUTATION_TIMESTAMP,
          updatedAt: MUTATION_TIMESTAMP,
        }
      : record,
  );
}

export function markNoticeAsRead(records: NoticeRecord[], noticeId: string) {
  return records.map((record) =>
    record.id === noticeId && record.status === 'UNREAD'
      ? {
          ...record,
          status: 'READ' as const,
          readAt: MUTATION_TIMESTAMP,
          updatedAt: MUTATION_TIMESTAMP,
        }
      : record,
  );
}

export function markNoticeAfterJump(records: NoticeRecord[], noticeId: string) {
  return records.map((record) => {
    if (record.id !== noticeId || record.status === 'STALE') {
      return record;
    }

    if (record.kind === 'ACTION_REQUIRED') {
      return {
        ...record,
        status: 'JUMPED' as const,
        jumpedAt: MUTATION_TIMESTAMP,
        updatedAt: MUTATION_TIMESTAMP,
      };
    }

    if (record.status === 'UNREAD') {
      return {
        ...record,
        status: 'READ' as const,
        readAt: MUTATION_TIMESTAMP,
        updatedAt: MUTATION_TIMESTAMP,
      };
    }

    return record;
  });
}

export function markIssueLinkedNoticesStale(records: NoticeRecord[], issueIds: string[]) {
  if (issueIds.length === 0) {
    return records;
  }

  const targetIds = new Set(issueIds);
  return records.map((record) =>
    record.issueId && targetIds.has(record.issueId)
      ? {
          ...record,
          status: 'STALE' as const,
          updatedAt: MUTATION_TIMESTAMP,
        }
      : record,
  );
}

export function removeIssueLinkedNotices(records: NoticeRecord[], issueIds: string[]) {
  if (issueIds.length === 0) {
    return records;
  }

  const targetIds = new Set(issueIds);
  return records.filter((record) => !record.issueId || !targetIds.has(record.issueId));
}

export function getNoticeKindLabel(kind: NoticeKind) {
  return kind === 'ACTION_REQUIRED' ? '处置类' : '提醒类';
}

export function getNoticeStatusLabel(record: NoticeRecord) {
  if (record.status === 'UNREAD') {
    return '未消费';
  }
  if (record.status === 'JUMPED') {
    return '已跳转';
  }
  return '已读';
}

export function getNoticePrimaryActionLabel(record: NoticeRecord) {
  return record.kind === 'ACTION_REQUIRED' ? '去处理' : '去查看';
}
