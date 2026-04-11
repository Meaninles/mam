import type { NoticeKind, NoticeRecord, NoticeStatus } from '../data';

export type NoticeConsumptionState = {
  byId: Record<
    string,
    {
      status: 'READ' | 'JUMPED';
      consumedAt: string;
    }
  >;
};

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

export function createEmptyNoticeConsumptionState(): NoticeConsumptionState {
  return { byId: {} };
}

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

export function applyNoticeConsumptions(
  records: NoticeRecord[],
  state: NoticeConsumptionState,
  options?: { activeIssueIds?: string[] },
) {
  const activeIssueIds = options?.activeIssueIds ? new Set(options.activeIssueIds) : null;

  return records.map((record) => {
    const local = state.byId[record.id];
    const serverUpdatedAt = Date.parse(record.updatedAt);
    const localConsumedAt = local ? Date.parse(local.consumedAt) : Number.NaN;
    const shouldPreserveIssueConsumption = record.sourceType === 'ISSUE' && local !== undefined;
    const sourceIssueStale =
      record.sourceType === 'ISSUE' &&
      record.issueId &&
      activeIssueIds !== null &&
      !activeIssueIds.has(record.issueId);

    if (record.status === 'STALE' || sourceIssueStale) {
      return {
        ...record,
        status: 'STALE' as const,
      };
    }

    if (
      !local ||
      Number.isNaN(localConsumedAt) ||
      (!shouldPreserveIssueConsumption && Number.isFinite(serverUpdatedAt) && localConsumedAt < serverUpdatedAt)
    ) {
      return {
        ...record,
        status: 'UNREAD' as const,
        readAt: undefined,
        jumpedAt: undefined,
      };
    }

    if (local.status === 'JUMPED') {
      return {
        ...record,
        status: 'JUMPED' as const,
        jumpedAt: local.consumedAt,
        readAt: undefined,
      };
    }

    return {
      ...record,
      status: 'READ' as const,
      readAt: local.consumedAt,
      jumpedAt: undefined,
    };
  });
}

export function markNoticeConsumed(
  state: NoticeConsumptionState,
  noticeId: string,
  status: 'READ' | 'JUMPED',
  consumedAt: string,
) {
  return {
    byId: {
      ...state.byId,
      [noticeId]: {
        status,
        consumedAt,
      },
    },
  };
}

export function markNoticeAsRead(state: NoticeConsumptionState, noticeId: string, consumedAt: string) {
  return markNoticeConsumed(state, noticeId, 'READ', consumedAt);
}

export function markNoticeAfterJump(state: NoticeConsumptionState, notice: NoticeRecord, consumedAt: string) {
  if (notice.kind === 'ACTION_REQUIRED') {
    return markNoticeConsumed(state, notice.id, 'JUMPED', consumedAt);
  }
  return markNoticeConsumed(state, notice.id, 'READ', consumedAt);
}

export function consumeReminderNoticeIds(state: NoticeConsumptionState, records: NoticeRecord[], consumedAt: string) {
  return records.reduce((current, record) => {
    if (record.kind !== 'REMINDER' || record.status === 'STALE') {
      return current;
    }
    return markNoticeConsumed(current, record.id, 'READ', consumedAt);
  }, state);
}

export function pruneNoticeConsumptions(state: NoticeConsumptionState, activeNoticeIds: string[]) {
  const activeIds = new Set(activeNoticeIds);
  const nextById = Object.fromEntries(Object.entries(state.byId).filter(([id]) => activeIds.has(id)));
  return { byId: nextById };
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
