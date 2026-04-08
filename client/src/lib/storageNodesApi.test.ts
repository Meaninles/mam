// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { storageNodesApi } from './storageNodesApi';

describe('storageNodesApi', () => {
  beforeEach(() => {
    window.localStorage.clear();
    (window as Window & { __MARE_RUNTIME_CONFIG__?: { centerBaseUrl?: string } }).__MARE_RUNTIME_CONFIG__ = {
      centerBaseUrl: 'http://127.0.0.1:18080',
    };
  });

  afterEach(() => {
    delete (window as Window & { __MARE_RUNTIME_CONFIG__?: { centerBaseUrl?: string } }).__MARE_RUNTIME_CONFIG__;
    vi.unstubAllGlobals();
  });

  it('uses center local folders and keeps NAS/cloud mock data unchanged', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: [
            {
              id: 'local-folder-1',
              name: '真实本地目录',
              libraryId: 'photo',
              libraryName: '商业摄影资产库',
              folderType: '本地',
              address: 'D:\\Real\\PhotoRaw',
              mountMode: '可写',
              enabled: true,
              scanStatus: '最近扫描成功',
              scanTone: 'success',
              lastScanAt: '今天 10:00',
              heartbeatPolicy: '从不',
              nextHeartbeatAt: '—',
              capacitySummary: '待检测',
              freeSpaceSummary: '待检测',
              capacityPercent: 0,
              riskTags: [],
              badges: ['本地', '可写'],
              authStatus: '无需鉴权',
              authTone: 'info',
              notes: '真实后端数据',
            },
          ],
        }),
      }),
    );

    const result = await storageNodesApi.loadDashboard();

    expect(result.mountFolders.some((item) => item.id === 'local-folder-1')).toBe(true);
    expect(result.mountFolders.some((item) => item.name === '商业摄影原片库')).toBe(false);
    expect(result.nasNodes.length).toBeGreaterThan(0);
    expect(result.cloudNodes.length).toBeGreaterThan(0);
  });

  it('sends local folder save requests to center service', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '挂载文件夹已保存',
          record: {
            id: 'local-folder-1',
            name: '真实本地目录',
          },
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await storageNodesApi.saveMountFolder({
      name: '真实本地目录',
      libraryId: 'photo',
      folderType: '本地',
      mountMode: '可写',
      heartbeatPolicy: '从不',
      localPath: 'D:\\Real\\PhotoRaw',
      nasId: '',
      cloudId: '',
      targetFolder: '',
      notes: '',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:18080/api/storage/local-folders',
      expect.objectContaining({
        method: 'POST',
      }),
    );
    expect(result.message).toBe('挂载文件夹已保存');
  });
});
