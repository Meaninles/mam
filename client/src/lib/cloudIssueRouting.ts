import type { IssueRecord, NoticeJumpTargetKind, NoticeRecord } from '../data';

export type CloudIssueKind =
  | '115 鉴权问题'
  | 'CloudDrive2 网关问题'
  | 'aria2 下载器问题'
  | '云端路径问题'
  | '传输执行问题';

function normalizeText(parts: Array<string | undefined>) {
  return parts
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
}

function includesAny(text: string, candidates: string[]) {
  return candidates.some((candidate) => text.includes(candidate));
}

export function resolveIssueCloudKind(issue: Pick<IssueRecord, 'category' | 'title' | 'summary' | 'type' | 'detail' | 'suggestion' | 'action' | 'objectLabel'>): CloudIssueKind | null {
  const haystack = normalizeText([
    issue.title,
    issue.summary,
    issue.type,
    issue.detail,
    issue.suggestion,
    issue.action,
    issue.objectLabel,
  ]);

  if (includesAny(haystack, ['aria2'])) {
    return 'aria2 下载器问题';
  }
  if (includesAny(haystack, ['clouddrive2', 'cd2'])) {
    return 'CloudDrive2 网关问题';
  }
  if (includesAny(haystack, ['115']) && includesAny(haystack, ['token', 'cookie', '鉴权', '登录'])) {
    return '115 鉴权问题';
  }
  if (includesAny(haystack, ['115']) && includesAny(haystack, ['目录', '路径', 'mount', 'root', '挂载'])) {
    return '云端路径问题';
  }
  if (issue.category === '传输') {
    return '传输执行问题';
  }
  return null;
}

export function resolveNoticeCloudKind(notice: Pick<NoticeRecord, 'title' | 'summary' | 'objectLabel' | 'jumpParams' | 'source'>): CloudIssueKind | null {
  const haystack = normalizeText([
    notice.title,
    notice.summary,
    notice.objectLabel,
    notice.source.issueCategory,
    notice.source.issueSourceDomain,
    notice.source.sourceLabel,
    notice.source.routeLabel,
    notice.jumpParams.label,
    notice.jumpParams.path,
  ]);

  if (includesAny(haystack, ['aria2'])) {
    return 'aria2 下载器问题';
  }
  if (includesAny(haystack, ['clouddrive2', 'cd2'])) {
    return 'CloudDrive2 网关问题';
  }
  if (includesAny(haystack, ['115']) && includesAny(haystack, ['token', 'cookie', '鉴权', '登录'])) {
    return '115 鉴权问题';
  }
  if (includesAny(haystack, ['115']) && includesAny(haystack, ['目录', '路径', 'mount', 'root', '挂载'])) {
    return '云端路径问题';
  }
  if (notice.source.issueCategory === '传输') {
    return '传输执行问题';
  }
  return null;
}

export function resolveCloudIssueTargetKind(kind: CloudIssueKind | null, fallback: NoticeJumpTargetKind): NoticeJumpTargetKind {
  if (kind === 'CloudDrive2 网关问题' || kind === 'aria2 下载器问题') {
    return 'settings';
  }
  if (kind === '115 鉴权问题' || kind === '云端路径问题') {
    return 'storage-nodes';
  }
  return fallback;
}
