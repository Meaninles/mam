import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { StorageNodesPage } from './StorageNodesPage';
import { storageNodesApi } from '../lib/storageNodesApi';

vi.mock('../lib/storageNodesApi', () => ({
  storageNodesApi: {
    loadDashboard: vi.fn(),
    saveNode: vi.fn(),
    runScan: vi.fn(),
    runConnectionTest: vi.fn(),
    updateHeartbeat: vi.fn(),
    saveCredentials: vi.fn(),
    updateEnabled: vi.fn(),
    deleteNode: vi.fn(),
    loadScanHistory: vi.fn(),
  },
}));

const mockedApi = vi.mocked(storageNodesApi);

type MockNode = {
  id: string;
  name: string;
  nodeType: '本机磁盘' | 'NAS/SMB' | '网盘';
  address: string;
  mountMode: '只读' | '可写';
  enabled: boolean;
  scanStatus: string;
  scanTone: 'success' | 'warning' | 'critical' | 'info';
  lastScanAt: string;
  heartbeatPolicy: '从不' | '每周（深夜）' | '每日（深夜）' | '每小时';
  nextHeartbeatAt: string;
  lastHeartbeatResult: string;
  heartbeatFailures: number;
  capacitySummary: string;
  freeSpaceSummary: string;
  capacityPercent: number;
  libraryBindings: string[];
  badges: string[];
  riskTags: string[];
  authStatus: string;
  authTone: 'success' | 'warning' | 'critical' | 'info';
  notes: string;
  vendor?: string;
  protocol?: string;
  detail:
    | {
        kind: 'local';
        rootPath: string;
      }
    | {
        kind: 'nas';
        host: string;
        shareName: string;
        username: string;
        passwordHint: string;
        protocol: 'SMB';
      }
    | {
        kind: 'cloud';
        vendor: '115';
        accountAlias: string;
        mountDirectory: string;
        accessMethod: '填入 Token' | '扫码登录获取 Token';
        qrChannel?: '微信小程序' | '支付宝小程序' | '电视端';
        tokenStatus: string;
      };
};

type MockDashboard = {
  nodes: MockNode[];
};

function createMockDashboard(): MockDashboard {
  return {
    nodes: [
      {
        id: 'node-local',
        name: '本地 NVMe 主盘',
        nodeType: '本机磁盘',
        address: 'D:\\Mare\\Assets',
        mountMode: '可写',
        enabled: true,
        scanStatus: '最近扫描成功',
        scanTone: 'success',
        lastScanAt: '今天 09:12',
        heartbeatPolicy: '从不',
        nextHeartbeatAt: '—',
        lastHeartbeatResult: '无需心跳',
        heartbeatFailures: 0,
        capacitySummary: '已用 64% · 3.4 TB 可用',
        freeSpaceSummary: '3.4 TB 可用',
        capacityPercent: 64,
        libraryBindings: ['商业摄影资产库', '视频工作流资产库'],
        badges: ['可写', '已绑定 2 个资产库'],
        riskTags: [],
        authStatus: '无需鉴权',
        authTone: 'info',
        notes: '本地生产主盘',
        detail: {
          kind: 'local',
          rootPath: 'D:\\Mare\\Assets',
        },
      },
      {
        id: 'node-nas',
        name: '影像 NAS 01',
        nodeType: 'NAS/SMB',
        address: '\\\\192.168.10.20\\media',
        mountMode: '可写',
        enabled: true,
        scanStatus: '等待队列',
        scanTone: 'info',
        lastScanAt: '2 分钟前',
        heartbeatPolicy: '每日（深夜）',
        nextHeartbeatAt: '今晚 02:00',
        lastHeartbeatResult: '上次成功',
        heartbeatFailures: 0,
        capacitySummary: '已用 48% · 18.9 TB 可用',
        freeSpaceSummary: '18.9 TB 可用',
        capacityPercent: 48,
        libraryBindings: ['商业摄影资产库', '视频工作流资产库'],
        badges: ['可写', 'SMB'],
        riskTags: [],
        authStatus: '鉴权正常',
        authTone: 'success',
        notes: '影像主 NAS',
        protocol: 'SMB',
        detail: {
          kind: 'nas',
          host: '192.168.10.20',
          shareName: 'media',
          username: 'mare-sync',
          passwordHint: '已保存，最近更新于 3 天前',
          protocol: 'SMB',
        },
      },
      {
        id: 'node-cloud',
        name: '115 云归档',
        nodeType: '网盘',
        address: '/MareArchive',
        mountMode: '可写',
        enabled: true,
        scanStatus: '最近扫描失败',
        scanTone: 'critical',
        lastScanAt: '今天 07:40',
        heartbeatPolicy: '每周（深夜）',
        nextHeartbeatAt: '周六 02:00',
        lastHeartbeatResult: '连续 2 次失败',
        heartbeatFailures: 2,
        capacitySummary: '远端容量正常 · 约 37% 已使用',
        freeSpaceSummary: '远端容量正常',
        capacityPercent: 37,
        libraryBindings: ['商业摄影资产库'],
        badges: ['115', '可写'],
        riskTags: ['扫描失败', '鉴权异常'],
        authStatus: '令牌 48 小时内过期',
        authTone: 'warning',
        notes: '云归档节点',
        vendor: '115',
        detail: {
          kind: 'cloud',
          vendor: '115',
          accountAlias: 'mare-archive',
          mountDirectory: '/MareArchive',
          accessMethod: '填入 Token',
          tokenStatus: '48 小时内过期',
        },
      },
    ],
  };
}

describe('StorageNodesPage', () => {
  let dashboard: MockDashboard;

  beforeEach(() => {
    dashboard = createMockDashboard();

    mockedApi.loadDashboard.mockImplementation(async () => structuredClone(dashboard));

    mockedApi.runScan.mockImplementation(async ({ ids }: { ids: string[] }) => {
      dashboard.nodes = dashboard.nodes.map((node) =>
        ids.includes(node.id)
          ? {
              ...node,
              scanStatus: '扫描中',
              scanTone: 'warning',
              lastScanAt: '正在执行',
            }
          : node,
      );

      return {
        message: `已为 ${ids.length} 个节点创建扫描任务`,
      };
    });

    mockedApi.runConnectionTest.mockImplementation(async ({ ids }: { ids: string[] }) => ({
      message: ids.length > 1 ? `已完成 ${ids.length} 个节点的连接测试` : '连接测试已完成',
      results: ids.map((id) => {
        const node = dashboard.nodes.find((item) => item.id === id)!;

        return {
          nodeId: node.id,
          nodeName: node.name,
          overallTone: node.id === 'node-cloud' ? 'warning' : 'success',
          summary:
            node.id === 'node-cloud' ? '令牌即将过期，建议尽快刷新后再继续归档。' : '节点可达且当前配置可继续使用。',
          checks: [
            { label: '可达性', status: 'success', detail: '目标节点可达。' },
            {
              label: '鉴权状态',
              status: node.id === 'node-cloud' ? 'warning' : 'success',
              detail: node.id === 'node-cloud' ? '令牌仍可用，但 48 小时内过期。' : '当前鉴权有效。',
            },
            { label: '读权限', status: 'success', detail: '可读取目标目录。' },
            { label: '写权限', status: 'success', detail: '可写入目标目录。' },
            { label: '目标目录可访问', status: 'success', detail: `已验证 ${node.address}。` },
          ],
          suggestion: node.id === 'node-cloud' ? '重新鉴权' : '可立即执行扫描',
          testedAt: '刚刚',
        };
      }),
    }));

    mockedApi.updateHeartbeat.mockImplementation(
      async ({ ids, heartbeatPolicy }: { ids: string[]; heartbeatPolicy: MockNode['heartbeatPolicy'] }) => {
        dashboard.nodes = dashboard.nodes.map((node) =>
          ids.includes(node.id)
            ? {
                ...node,
                heartbeatPolicy,
                nextHeartbeatAt: heartbeatPolicy === '每小时' ? '1 小时后' : node.nextHeartbeatAt,
              }
            : node,
        );

        return {
          message: ids.length > 1 ? `已更新 ${ids.length} 个节点的心跳策略` : '心跳策略已更新',
        };
      },
    );

    mockedApi.saveCredentials.mockImplementation(async ({ id }: { id: string }) => {
      dashboard.nodes = dashboard.nodes.map((node) =>
        node.id === id
          ? {
              ...node,
              authStatus: '鉴权正常',
              authTone: 'success',
              riskTags: node.riskTags.filter((tag) => tag !== '鉴权异常'),
            }
          : node,
      );

      return {
        message: '鉴权信息已保存',
      };
    });

    mockedApi.updateEnabled.mockImplementation(async ({ ids, enabled }: { ids: string[]; enabled: boolean }) => {
      dashboard.nodes = dashboard.nodes.map((node) =>
        ids.includes(node.id)
          ? {
              ...node,
              enabled,
            }
          : node,
      );

      return {
        message: enabled ? '已启用所选节点' : '已停用所选节点',
      };
    });

    mockedApi.deleteNode.mockImplementation(async ({ id }: { id: string }) => {
      dashboard.nodes = dashboard.nodes.filter((node) => node.id !== id);

      return {
        message: '节点已删除',
      };
    });

    mockedApi.loadScanHistory.mockImplementation(async ({ id }: { id: string }) => ({
      nodeId: id,
      items: [
        {
          id: 'history-1',
          startedAt: '2026-03-31 02:00',
          finishedAt: '2026-03-31 02:18',
          status: '成功',
          summary: '新增 218 项，变更 12 项，未发现异常。',
          trigger: '计划扫描',
        },
        {
          id: 'history-2',
          startedAt: '2026-03-30 02:00',
          finishedAt: '2026-03-30 02:06',
          status: '失败',
          summary: '远端目录读取超时，已写入异常中心。',
          trigger: '计划扫描',
        },
      ],
    }));

    mockedApi.saveNode.mockImplementation(async ({ draft }) => {
      const address =
        draft.detail.kind === 'nas'
          ? `\\\\${draft.detail.host}\\${draft.detail.shareName}`
          : draft.detail.kind === 'local'
            ? draft.detail.rootPath
            : draft.detail.mountDirectory;

      dashboard.nodes = [
        {
          id: 'node-new',
          name: draft.name,
          nodeType: draft.nodeType as MockNode['nodeType'],
          address,
          mountMode: draft.mountMode as MockNode['mountMode'],
          enabled: true,
          scanStatus: '未扫描',
          scanTone: 'info',
          lastScanAt: '未扫描',
          heartbeatPolicy: draft.heartbeatPolicy as MockNode['heartbeatPolicy'],
          nextHeartbeatAt: draft.heartbeatPolicy === '从不' ? '—' : '待首次执行',
          lastHeartbeatResult: '尚未执行',
          heartbeatFailures: 0,
          capacitySummary: '待首次检测',
          freeSpaceSummary: '待首次检测',
          capacityPercent: 0,
          libraryBindings: [],
          badges: [draft.mountMode],
          riskTags: [],
          authStatus: draft.nodeType === 'NAS/SMB' ? '待鉴权' : '无需鉴权',
          authTone: draft.nodeType === 'NAS/SMB' ? 'warning' : 'info',
          notes: draft.notes,
          detail:
            draft.detail.kind === 'nas'
              ? {
                  kind: 'nas',
                  host: draft.detail.host,
                  shareName: draft.detail.shareName,
                  username: draft.detail.username,
                  passwordHint: '刚刚更新',
                  protocol: 'SMB',
                }
              : draft.detail.kind === 'cloud'
                ? {
                    kind: 'cloud',
                    vendor: draft.detail.vendor,
                    accountAlias: draft.detail.accountAlias,
                    mountDirectory: draft.detail.mountDirectory,
                    accessMethod: draft.detail.accessMethod,
                    qrChannel: draft.detail.accessMethod === '扫码登录获取 Token' ? draft.detail.qrChannel : undefined,
                    tokenStatus: draft.detail.token ? '已配置' : '未配置',
                  }
                : {
                  kind: 'local',
                  rootPath: address,
                },
        },
        ...dashboard.nodes,
      ];

      return {
        message: '存储节点已保存',
      };
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('支持节点筛选、多选和批量扫描', async () => {
    const user = userEvent.setup();

    render(<StorageNodesPage />);

    expect(await screen.findByText('3 个节点')).toBeInTheDocument();
    expect(screen.getByText('本地 NVMe 主盘')).toBeInTheDocument();

    await user.type(screen.getByLabelText('搜索节点'), '影像');
    expect(screen.getByText('影像 NAS 01')).toBeInTheDocument();
    expect(screen.queryByText('115 云归档')).not.toBeInTheDocument();

    await user.clear(screen.getByLabelText('搜索节点'));
    await user.click(screen.getByLabelText('选择节点 本地 NVMe 主盘'));
    await user.click(screen.getByLabelText('选择节点 影像 NAS 01'));

    expect(screen.getByText('已选择 2 个节点')).toBeInTheDocument();

    await user.click(screen.getAllByRole('button', { name: '批量扫描' }).at(-1)!);

    expect(await screen.findByText('已为 2 个节点创建扫描任务')).toBeInTheDocument();
    expect(screen.getAllByText('扫描中').length).toBeGreaterThan(0);
  });

  it('支持单节点连接测试并展示分项结果', async () => {
    const user = userEvent.setup();

    render(<StorageNodesPage />);

    const row = (await screen.findByRole('row', { name: /115 云归档/ })) ?? screen.getByText('115 云归档').closest('tr');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByRole('button', { name: '连接测试 115 云归档' })).toHaveAttribute('title', '连接测试');

    await user.click(within(row as HTMLElement).getByRole('button', { name: '连接测试 115 云归档' }));

    expect(await screen.findByRole('dialog', { name: '连接测试结果' })).toBeInTheDocument();
    expect(screen.getByText('令牌即将过期，建议尽快刷新后再继续归档。')).toBeInTheDocument();
    expect(screen.getByText('可达性')).toBeInTheDocument();
    expect(screen.getByText('鉴权状态')).toBeInTheDocument();
    expect(screen.getByText('重新鉴权')).toBeInTheDocument();
  });

  it('支持新增 NAS 节点并根据类型切换表单字段', async () => {
    const user = userEvent.setup();

    render(<StorageNodesPage />);

    await user.click(await screen.findByRole('button', { name: '新增节点' }));

    expect(await screen.findByRole('region', { name: '新增存储节点' })).toBeInTheDocument();
    const sheet = screen.getByRole('region', { name: '新增存储节点' });

    await user.type(within(sheet).getByLabelText('节点名称'), '后期 NAS 02');
    expect(screen.queryByRole('combobox', { name: '节点类型' })).not.toBeInTheDocument();
    expect(within(sheet).queryByText('允许扫描')).not.toBeInTheDocument();
    await user.click(within(sheet).getByRole('button', { name: 'NAS/SMB' }));

    expect(within(sheet).getByLabelText('主机/IP')).toBeInTheDocument();
    expect(within(sheet).getByLabelText('共享目录')).toBeInTheDocument();
    expect(within(sheet).getByLabelText('主机/IP')).toHaveAttribute('placeholder', '例如：192.168.10.20');
    expect(within(sheet).getByLabelText('共享目录')).toHaveAttribute('placeholder', '例如：media');

    await user.type(within(sheet).getByLabelText('主机/IP'), '192.168.10.88');
    await user.type(within(sheet).getByLabelText('共享目录'), 'post');
    await user.type(within(sheet).getByLabelText('用户名'), 'mare-post');
    await user.type(within(sheet).getByLabelText('密码'), 'mock-password');
    await user.selectOptions(within(sheet).getByLabelText('心跳周期'), '每日（深夜）');
    await user.click(within(sheet).getByRole('button', { name: '保存节点' }));

    expect(await screen.findByText('存储节点已保存')).toBeInTheDocument();
    expect(screen.getByText('后期 NAS 02')).toBeInTheDocument();
  });

  it('支持设置心跳和保存鉴权信息', async () => {
    const user = userEvent.setup();

    render(<StorageNodesPage />);

    const cloudRow = (await screen.findByRole('row', { name: /115 云归档/ })) ?? screen.getByText('115 云归档').closest('tr');
    expect(cloudRow).not.toBeNull();

    await user.click(within(cloudRow as HTMLElement).getByRole('button', { name: '更多操作 115 云归档' }));
    await user.click(screen.getByRole('button', { name: '设置心跳' }));

    expect(await screen.findByRole('dialog', { name: '设置心跳' })).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('心跳周期设置'), '每小时');
    await user.click(screen.getByRole('button', { name: '保存心跳' }));

    expect(await screen.findByText('心跳策略已更新')).toBeInTheDocument();
    expect(screen.getByText('每小时')).toBeInTheDocument();

    await user.click(within(cloudRow as HTMLElement).getByRole('button', { name: '更多操作 115 云归档' }));
    await user.click(screen.getByRole('button', { name: '鉴权设置' }));

    expect(await screen.findByRole('region', { name: '鉴权设置' })).toBeInTheDocument();
    await user.clear(screen.getByLabelText('Token'));
    await user.type(screen.getByLabelText('Token'), 'mock-token-updated');
    await user.click(screen.getByRole('button', { name: '保存鉴权' }));

    expect(await screen.findByText('鉴权信息已保存')).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText('鉴权正常').length).toBeGreaterThan(0));
  });

  it('支持查看扫描历史并删除节点', async () => {
    const user = userEvent.setup();

    render(<StorageNodesPage />);

    const cloudRow = (await screen.findByRole('row', { name: /115 云归档/ })) ?? screen.getByText('115 云归档').closest('tr');
    expect(cloudRow).not.toBeNull();

    await user.click(within(cloudRow as HTMLElement).getByRole('button', { name: '更多操作 115 云归档' }));
    await user.click(screen.getByRole('button', { name: '查看扫描历史' }));

    expect(await screen.findByRole('dialog', { name: '扫描历史' })).toBeInTheDocument();
    expect(screen.getByText('新增 218 项，变更 12 项，未发现异常。')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '关闭' }));

    await user.click(within(cloudRow as HTMLElement).getByRole('button', { name: '更多操作 115 云归档' }));
    await user.click(screen.getByRole('button', { name: '删除节点' }));

    expect(await screen.findByText('节点已删除')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('115 云归档')).not.toBeInTheDocument());
  });
});
