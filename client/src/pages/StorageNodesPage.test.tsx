import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QRCode from 'qrcode';
import { StorageNodesPage } from './StorageNodesPage';
import { storageNodesApi } from '../lib/storageNodesApi';

vi.mock('../lib/storageNodesApi', () => ({
  storageNodesApi: {
    browseLocalFolder: vi.fn(),
    createCloudQrSession: vi.fn(),
    deleteCloudNode: vi.fn(),
    deleteLocalNode: vi.fn(),
    deleteMountFolder: vi.fn(),
    deleteNasNode: vi.fn(),
    getCloudQrImageUrl: vi.fn(),
    getCloudQrSessionStatus: vi.fn(),
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

vi.mock('qrcode', () => ({
  default: {
    toDataURL: vi.fn(),
  },
}));

const mockedApi = vi.mocked(storageNodesApi);
const mockedQRCode = vi.mocked(QRCode);

const libraries = [
  { id: 'photo', name: '商业摄影资产库', rootLabel: '', itemCount: '', health: '', storagePolicy: '' },
  { id: 'video', name: '视频工作流资产库', rootLabel: '', itemCount: '', health: '', storagePolicy: '' },
];

describe('StorageNodesPage', () => {
  beforeEach(() => {
    mockedQRCode.toDataURL.mockImplementation(async () => 'data:image/png;base64,qr-preview');
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
          capacitySummary: '已用 64% / 可用 3.4 TB',
          freeSpaceSummary: '3.4 TB 可用',
          capacityPercent: 64,
          mountCount: 1,
          notes: '',
        },
      ],
      nasNodes: [],
      cloudNodes: [],
      mounts: [],
      mountFolders: [],
    });
    mockedApi.createCloudQrSession.mockResolvedValue({
      uid: 'uid-1',
      time: 123,
      sign: 'sign-1',
      qrcode: 'https://115.com/qr/mock',
      channel: '微信小程序',
    });
    mockedApi.getCloudQrSessionStatus.mockResolvedValue({
      status: 'WAITING',
      message: '等待扫码',
    });
    mockedApi.saveCloudNode.mockResolvedValue({ message: '网盘已保存' });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('默认打开本地文件夹管理', async () => {
    render(<StorageNodesPage libraries={libraries as any} />);

    expect(await screen.findByText('本地文件夹管理')).toBeInTheDocument();
    expect(screen.getByText('本地素材根目录')).toBeInTheDocument();
  });

  it('扫码登录二维码会在前端本地渲染', async () => {
    const user = userEvent.setup();
    render(<StorageNodesPage libraries={libraries as any} />);

    await screen.findByText('本地素材根目录');
    await user.click(screen.getByRole('button', { name: '网盘管理' }));
    await user.click(screen.getByRole('button', { name: '新增网盘' }));

    const sheet = await screen.findByRole('region', { name: '新增网盘' });
    const qrImage = await within(sheet).findByRole('img', { name: /115/ });

    expect(mockedApi.createCloudQrSession).toHaveBeenCalledWith('微信小程序');
    expect(mockedQRCode.toDataURL).toHaveBeenCalledWith(
      'https://115.com/qr/mock',
      expect.objectContaining({ width: 168, margin: 1 }),
    );
    expect(qrImage).toHaveAttribute('src', 'data:image/png;base64,qr-preview');
  });
});
