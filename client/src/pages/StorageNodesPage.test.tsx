import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { StorageNodesPage } from './StorageNodesPage';
import { storageNodesApi } from '../lib/storageNodesApi';

vi.mock('../lib/storageNodesApi', () => ({
  storageNodesApi: {
    browseLocalFolder: vi.fn(),
    deleteCloudNode: vi.fn(),
    deleteLocalNode: vi.fn(),
    deleteMountFolder: vi.fn(),
    deleteNasNode: vi.fn(),
    loadDashboard: vi.fn(),
    loadMountScanHistory: vi.fn(),
    runCloudConnectionTest: vi.fn(),
    runLocalNodeConnectionTest: vi.fn(),
    runMountConnectionTest: vi.fn(),
    runMountScan: vi.fn(),
    runNasConnectionTest: vi.fn(),
    saveCloudNode: vi.fn(),
    saveLocalNode: vi.fn(),
    saveMount: vi.fn(),
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
      localNodes: [
        {
          id: 'local-node-1',
          name: '本地素材根目录',
          rootPath: 'D:\\Assets',
          enabled: true,
          healthStatus: '可用',
          healthTone: 'success',
          lastCheckAt: '今天 09:12',
          capacitySummary: '已用 64% · 3.4 TB 可用',
          freeSpaceSummary: '3.4 TB 可用',
          capacityPercent: 64,
          mountCount: 1,
          notes: '',
        },
      ],
      nasNodes: [
        {
          id: 'nas-1',
          name: '影像 NAS 01',
          address: '\\\\192.168.10.20\\media',
          accessMode: 'SMB',
          username: 'mare-sync',
          passwordHint: '已保存',
          status: '鉴权正常',
          tone: 'success',
          mountCount: 2,
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
      mounts: [
        {
          id: 'mount-1',
          name: '上海发布会原片',
          libraryId: 'photo',
          libraryName: '商业摄影资产库',
          nodeId: 'local-node-1',
          nodeName: '本地素材根目录',
          nodeRootPath: 'D:\\Assets',
          relativePath: 'ShanghaiLaunch\\RAW',
          folderType: '本地',
          address: 'D:\\Assets\\ShanghaiLaunch\\RAW',
          mountMode: '可写',
          enabled: true,
          scanStatus: '最近扫描成功',
          scanTone: 'success',
          lastScanAt: '今天 09:20',
          heartbeatPolicy: '每小时',
          nextHeartbeatAt: '1小时后',
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
      mountFolders: [],
    });
    mockedApi.browseLocalFolder.mockResolvedValue({ path: 'D:\\Assets' });
    mockedApi.saveLocalNode.mockResolvedValue({ message: '本地文件夹已保存' });
    mockedApi.saveMount.mockResolvedValue({ message: '挂载已保存' });
    mockedApi.saveNasNode.mockResolvedValue({ message: 'NAS 已保存' });
    mockedApi.saveCloudNode.mockResolvedValue({ message: '网盘已保存' });
    mockedApi.runLocalNodeConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [{ id: 'local-node-1', name: '本地素材根目录', overallTone: 'success', summary: '节点根目录可访问。', checks: [], testedAt: '刚刚' }],
    });
    mockedApi.runMountScan.mockResolvedValue({ message: '已为 1 个挂载创建扫描任务' });
    mockedApi.runMountConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [{ id: 'mount-1', name: '上海发布会原片', overallTone: 'success', summary: '挂载目录可访问。', checks: [], testedAt: '刚刚' }],
    });
    mockedApi.runNasConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [{ id: 'nas-1', name: '影像 NAS 01', overallTone: 'success', summary: 'NAS 连接测试通过。', checks: [], testedAt: '刚刚' }],
    });
    mockedApi.runCloudConnectionTest.mockResolvedValue({
      message: '连接测试已完成',
      results: [{ id: 'cloud-1', name: '115 云归档', overallTone: 'success', summary: '网盘连接测试通过。', checks: [], testedAt: '刚刚' }],
    });
    mockedApi.updateMountHeartbeat.mockResolvedValue({ message: '心跳策略已更新' });
    mockedApi.loadMountScanHistory.mockResolvedValue({
      id: 'mount-1',
      items: [{ id: 'history-1', startedAt: '2026-03-31 02:00', finishedAt: '2026-03-31 02:18', status: '成功', summary: '完成扫描。', trigger: '计划扫描' }],
    });
    mockedApi.deleteLocalNode.mockResolvedValue({ message: '本地文件夹已删除' });
    mockedApi.deleteMountFolder.mockResolvedValue({ message: '挂载已删除' });
    mockedApi.deleteNasNode.mockResolvedValue({ message: 'NAS 已删除' });
    mockedApi.deleteCloudNode.mockResolvedValue({ message: '网盘已删除' });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('默认展示节点管理，本地节点不再直接绑定资产库', async () => {
    render(<StorageNodesPage libraries={libraries as any} />);

    expect(await screen.findByText('本地文件夹管理')).toBeInTheDocument();
    expect(screen.getByText('本地素材根目录')).toBeInTheDocument();
    expect(screen.queryByText('商业摄影资产库')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '挂载管理' })).toBeInTheDocument();
  });

  it('支持切换到挂载管理并显示资产库绑定', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: '挂载管理' }));

    expect(screen.getByRole('button', { name: '挂载管理' })).toHaveClass('active');
    expect(screen.getByRole('button', { name: '本地文件夹管理' })).not.toHaveClass('active');

    expect(await screen.findByText('上海发布会原片')).toBeInTheDocument();
    expect(screen.getAllByText('商业摄影资产库').length).toBeGreaterThan(0);
    expect(screen.getByText('D:\\Assets\\ShanghaiLaunch\\RAW')).toBeInTheDocument();
    expect(screen.getByLabelText('资产库筛选')).toBeInTheDocument();
  });

  it('挂载管理支持按资产库筛选', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: '挂载管理' }));
    await user.selectOptions(screen.getByLabelText('资产库筛选'), '视频工作流资产库');

    expect(screen.queryByText('上海发布会原片')).not.toBeInTheDocument();
    expect(screen.getByText('没有匹配的挂载')).toBeInTheDocument();
  });

  it('支持新增本地节点', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: '新增本地文件夹' }));

    const sheet = await screen.findByRole('region', { name: '新增本地文件夹节点' });
    await user.type(within(sheet).getByLabelText('节点名称'), '项目根目录');
    await user.click(within(sheet).getByRole('button', { name: '浏览目录' }));
    await user.click(within(sheet).getByRole('button', { name: '保存本地文件夹节点' }));

    expect(mockedApi.saveLocalNode).toHaveBeenCalled();
  });

  it('支持从挂载管理创建挂载，并要求选择节点和资产库', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: '挂载管理' }));
    await user.click(screen.getByRole('button', { name: '新增挂载' }));

    const sheet = await screen.findByRole('region', { name: '新增挂载' });
    await user.type(within(sheet).getByLabelText('挂载名称'), '精选交付');
    await user.selectOptions(within(sheet).getByLabelText('所属节点'), 'local-node-1');
    await user.selectOptions(within(sheet).getByLabelText('所属资产库'), 'photo');
    await user.type(within(sheet).getByLabelText('挂载子目录'), 'Delivery\\精选');
    await user.click(within(sheet).getByRole('button', { name: '保存挂载' }));

    expect(mockedApi.saveMount).toHaveBeenCalled();
  });

  it('挂载表单的所属节点下拉应包含 NAS 节点', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: '挂载管理' }));
    await user.click(screen.getByRole('button', { name: '新增挂载' }));

    const sheet = await screen.findByRole('region', { name: '新增挂载' });
    const nodeTypeSelect = within(sheet).getByLabelText('节点类型');
    const nodeSelect = within(sheet).getByLabelText('所属节点');

    expect(within(nodeTypeSelect).getByRole('option', { name: '本地' })).toBeInTheDocument();
    expect(within(nodeTypeSelect).getByRole('option', { name: 'NAS' })).toBeInTheDocument();
    expect(within(nodeSelect).getByRole('option', { name: '本地 · 本地素材根目录' })).toBeInTheDocument();

    await user.selectOptions(nodeTypeSelect, 'NAS');

    expect(within(nodeSelect).getByRole('option', { name: 'NAS · 影像 NAS 01' })).toBeInTheDocument();
    expect(within(nodeSelect).queryByRole('option', { name: '本地 · 本地素材根目录' })).not.toBeInTheDocument();
  });

  it('从 NAS 页切到挂载管理后，新增入口必须打开挂载表单而不是 NAS 表单', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: 'NAS 管理' }));
    await screen.findByText('影像 NAS 01');

    await user.click(screen.getByRole('button', { name: '挂载管理' }));
    await user.click(screen.getByRole('button', { name: '新增挂载' }));

    expect(await screen.findByRole('region', { name: '新增挂载' })).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: '新增 NAS' })).not.toBeInTheDocument();
  });
});
