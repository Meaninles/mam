import { describe, expect, it } from 'vitest';
import { resolveCloudIssueTargetKind, resolveIssueCloudKind, resolveNoticeCloudKind } from './cloudIssueRouting';

describe('cloudIssueRouting', () => {
  it('识别 CloudDrive2 和 aria2 运行类问题', () => {
    expect(
      resolveIssueCloudKind({
        category: '节点与权限',
        title: 'CloudDrive2 网关离线',
        summary: 'CD2 健康检查失败',
        type: 'GATEWAY_FAILED',
        detail: 'CloudDrive2 reconnect failed',
        suggestion: '前往依赖服务页处理',
        action: '检查 CD2',
        objectLabel: 'CloudDrive2',
      }),
    ).toBe('CloudDrive2 网关问题');

    expect(
      resolveIssueCloudKind({
        category: '传输',
        title: 'aria2 下载失败',
        summary: 'aria2 rpc timeout',
        type: 'ARIA2_TIMEOUT',
        detail: 'aria2 downloader unavailable',
        suggestion: '检查 aria2',
        action: '重试下载',
        objectLabel: 'aria2',
      }),
    ).toBe('aria2 下载器问题');
  });

  it('识别 115 鉴权和云端路径问题，并映射到正确跳转目标', () => {
    expect(
      resolveIssueCloudKind({
        category: '节点与权限',
        title: '115 鉴权失效',
        summary: 'Token 已失效',
        type: 'TOKEN_INVALID',
        detail: 'cookie rejected by 115',
        suggestion: '重新扫码登录',
        action: '更新 115 凭据',
        objectLabel: '115 云归档',
      }),
    ).toBe('115 鉴权问题');

    expect(
      resolveNoticeCloudKind({
        title: '云端目录校验失败',
        summary: '115 挂载目录不存在',
        objectLabel: '115 云归档',
        jumpParams: { kind: 'issues', path: '/MareArchive/missing' },
        source: {
          sourceDomain: '异常中心',
          issueCategory: '节点与权限',
          issueSourceDomain: '存储节点',
          sourceLabel: '115 云归档',
          routeLabel: '存储节点 / 网盘管理',
        },
      }),
    ).toBe('云端路径问题');

    expect(resolveCloudIssueTargetKind('CloudDrive2 网关问题', 'issues')).toBe('settings');
    expect(resolveCloudIssueTargetKind('aria2 下载器问题', 'issues')).toBe('settings');
    expect(resolveCloudIssueTargetKind('115 鉴权问题', 'issues')).toBe('storage-nodes');
    expect(resolveCloudIssueTargetKind('云端路径问题', 'issues')).toBe('storage-nodes');
  });
});
