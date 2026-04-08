import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { StorageNodesPage } from './StorageNodesPage';
import { storageNodesApi } from '../lib/storageNodesApi';

vi.mock('../lib/storageNodesApi', () => ({
  storageNodesApi: {
    browseLocalFolder: vi.fn(),
    deleteCloudNode: vi.fn(),
    deleteMountFolder: vi.fn(),
    deleteNasNode: vi.fn(),
    loadDashboard: vi.fn(),
    loadMountScanHistory: vi.fn(),
    runCloudConnectionTest: vi.fn(),
    runMountConnectionTest: vi.fn(),
    runMountScan: vi.fn(),
    runNasConnectionTest: vi.fn(),
    saveCloudNode: vi.fn(),
    saveMountFolder: vi.fn(),
    saveNasNode: vi.fn(),
    updateMountHeartbeat: vi.fn(),
  },
}));

const mockedApi = vi.mocked(storageNodesApi);

const libraries = [
  { id: 'photo', name: '商业摄影资产库', rootLabel: '', itemCount: '', health: '', storagePolicy: '' },
  { id: 'video', name: '视频工作流资产库', rootLabel: '', itemCount: '', health: '', storagePolicy: '' },
];

describe('StorageNodesPage', () => {
  beforeEach(() => {
    mockedApi.loadDashboard.mockResolvedValue({
      mountFolders: [
        {
          id: 'mount-1',
          name: '商业摄影原片库',
          libraryId: 'photo',
          libraryName: '商业摄影资产库',
          folderType: '本地',
          address: 'D:\\Mare\\Assets\\PhotoRaw',
          mountMode: '可写',
          enabled: true,
          scanStatus: '最近扫描成功',
          scanTone: 'success',
          lastScanAt: '今天 09:12',
          heartbeatPolicy: '从不',
          nextHeartbeatAt: '—',
          capacitySummary: '已用 64% · 3.4 TB 可用',
          freeSpaceSummary: '3.4 TB 可用',
          capacityPercent: 64,
          riskTags: [],
          badges: ['本地', '可写'],
          authStatus: '无需鉴权',
          authTone: 'info',
          notes: '',
        },
      ],
      nasNodes: [
        {
          id: 'nas-1',
          name: '影像 NAS 01',
          address: '\\\\192.168.10.20\\media',
          username: 'mare-sync',
          passwordHint: '已保存',
          status: '鉴权正常',
          tone: 'success',
          notes: '',
        },
      ],
      cloudNodes: [
        {
          id: 'cloud-1',
          name: '115 云归档',
          vendor: '115',
          accessMethod: '填入 Token',
          accountAlias: 'mare-archive',
          mountDirectory: '/MareArchive',
          tokenStatus: '已配置',
          status: '鉴权正常',
          tone: 'success',
          notes: '',
        },
      ],
    });
    mockedApi.browseLocalFolder.mockResolvedValue({ path: 'D:\\Mare\\NewMount' });
    mockedApi.saveMountFolder.mockResolvedValue({ message: '挂载文件夹已保存' });
    mockedApi.saveNasNode.mockResolvedValue({ message: 'NAS 已保存' });
    mockedApi.saveCloudNode.mockResolvedValue({ message: '网盘已保存' });
    mockedApi.runMountScan.mockResolvedValue({ message: '已为 1 个挂载文件夹创建扫描任务' });
    mockedApi.runMountConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [
        {
          id: 'mount-1',
          name: '商业摄影原片库',
          overallTone: 'success',
          summary: '目录可访问。',
          checks: [{ label: '可达性', status: 'success', detail: '目录可访问。' }],
          testedAt: '刚刚',
        },
      ],
    });
    mockedApi.runNasConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [
        {
          id: 'nas-1',
          name: '影像 NAS 01',
          overallTone: 'success',
          summary: 'NAS 连接测试通过，可继续使用当前配置。',
          checks: [{ label: '可达性', status: 'success', detail: 'NAS 可达。' }],
          testedAt: '刚刚',
        },
      ],
    });
    mockedApi.runCloudConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [
        {
          id: 'cloud-1',
          name: '115 云归档',
          overallTone: 'warning',
          summary: '网盘可达，但 Token 需要重新确认。',
          checks: [{ label: '鉴权状态', status: 'warning', detail: 'Token 需要重新确认。' }],
          testedAt: '刚刚',
        },
      ],
    });
    mockedApi.updateMountHeartbeat.mockResolvedValue({ message: '心跳策略已更新' });
    mockedApi.loadMountScanHistory.mockResolvedValue({
      id: 'mount-1',
      items: [{ id: 'history-1', startedAt: '2026-03-31 02:00', finishedAt: '2026-03-31 02:18', status: '成功', summary: '新增 218 项。', trigger: '计划扫描' }],
    });
    mockedApi.deleteMountFolder.mockResolvedValue({ message: '挂载文件夹已删除' });
    mockedApi.deleteNasNode.mockResolvedValue({ message: 'NAS 已删除' });
    mockedApi.deleteCloudNode.mockResolvedValue({ message: '网盘已删除' });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('第一页已收敛为本地文件夹管理，不再显示混合类型筛选', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    expect(await screen.findByText('本地文件夹管理')).toBeInTheDocument();
    expect(screen.getByText('商业摄影原片库')).toBeInTheDocument();
    expect(screen.queryByLabelText('挂载类型筛选')).not.toBeInTheDocument();

    await user.click(screen.getByLabelText('选择本地文件夹 商业摄影原片库'));
    expect(screen.getByText('已选择 1 个本地文件夹')).toBeInTheDocument();
  });

  it('支持切换到 NAS 管理和网盘管理子页', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('商业摄影原片库');
    await user.click(screen.getByRole('button', { name: 'NAS 管理' }));
    expect(await screen.findByText('影像 NAS 01')).toBeInTheDocument();
    expect(screen.getByText('mare-sync')).toBeInTheDocument();
    expect(screen.getByText('已保存')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '网盘管理' }));
    expect(await screen.findByText('115 云归档')).toBeInTheDocument();
    expect(screen.getByText('/MareArchive')).toBeInTheDocument();
  });

  it('支持点击空白关闭更多操作，并在 NAS/网盘管理中进行连接测试', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('商业摄影原片库');
    await user.click(screen.getByRole('button', { name: 'NAS 管理' }));
    const nasMenuButton = await screen.findByRole('button', { name: '更多操作 影像 NAS 01' });
    await user.click(nasMenuButton);
    expect(screen.getByRole('button', { name: '删除' })).toBeInTheDocument();
    await user.click(document.body);
    expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '连接测试 影像 NAS 01' }));
    expect(mockedApi.runNasConnectionTest).toHaveBeenCalledWith(['nas-1']);
    expect(await screen.findByText('NAS 连接测试通过，可继续使用当前配置。')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '网盘管理' }));
    await user.click(screen.getByRole('button', { name: '连接测试 115 云归档' }));
    expect(mockedApi.runCloudConnectionTest).toHaveBeenCalledWith(['cloud-1']);
    expect(await screen.findByText('网盘可达，但 Token 需要重新确认。')).toBeInTheDocument();
  });

  it('支持新增本地文件夹', async () => {
    const user = userEvent.setup();
    const handleFeedback = vi.fn();
    render(<StorageNodesPage libraries={libraries as any} onFeedback={handleFeedback} />);

    await screen.findByText('商业摄影原片库');
    await user.click(screen.getByRole('button', { name: '新增本地文件夹' }));

    const sheet = await screen.findByRole('region', { name: '新增本地文件夹' });
    await user.type(within(sheet).getByLabelText('文件夹名称'), '新本地文件夹');
    await user.selectOptions(within(sheet).getByLabelText('所属资产库'), 'photo');
    await user.click(within(sheet).getByRole('button', { name: '浏览目录' }));
    expect(mockedApi.browseLocalFolder).toHaveBeenCalled();
    expect(within(sheet).getByLabelText('本地目录')).toHaveValue('D:\\Mare\\NewMount');

    await user.click(within(sheet).getByRole('button', { name: '保存本地文件夹' }));
    expect(mockedApi.saveMountFolder).toHaveBeenCalled();
    expect(handleFeedback).toHaveBeenCalledWith(
      expect.objectContaining({
        message: '挂载文件夹已保存',
        tone: 'success',
      }),
    );
  });
});
