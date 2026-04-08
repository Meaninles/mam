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

  it('loadDashboard 返回节点层与挂载层分离数据', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn()
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'local-node-1',
                name: '本地素材根目录',
                rootPath: 'D:\\Assets',
                enabled: true,
                healthStatus: '可用',
                healthTone: 'success',
                lastCheckAt: '今天 10:00',
                capacitySummary: '已用 20% · 800 GB 可用',
                freeSpaceSummary: '800 GB 可用',
                capacityPercent: 20,
                mountCount: 2,
                notes: '',
              },
            ],
          }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            data: [
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
                lastScanAt: '今天 10:10',
                heartbeatPolicy: '每小时',
                nextHeartbeatAt: '1小时后',
                capacitySummary: '已用 20% · 800 GB 可用',
                freeSpaceSummary: '800 GB 可用',
                capacityPercent: 20,
                riskTags: [],
                badges: ['本地', '可写'],
                authStatus: '无需鉴权',
                authTone: 'info',
                notes: '',
              },
            ],
          }),
        }),
    );

    const result = await storageNodesApi.loadDashboard();

    expect(result.localNodes).toHaveLength(1);
    expect(result.localNodes[0]).not.toHaveProperty('libraryId');
    expect(result.mounts).toHaveLength(1);
    expect(result.mounts[0].nodeId).toBe('local-node-1');
    expect(result.nasNodes.length).toBeGreaterThan(0);
    expect(result.cloudNodes.length).toBeGreaterThan(0);
  });

  it('saveLocalNode 只保存节点，不接受资产库绑定', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '本地文件夹已保存',
          record: {
            id: 'local-node-1',
            name: '本地素材根目录',
            rootPath: 'D:\\Assets',
          },
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await storageNodesApi.saveLocalNode({
      name: '本地素材根目录',
      rootPath: 'D:\\Assets',
      notes: '',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:18080/api/storage/local-nodes',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          name: '本地素材根目录',
          rootPath: 'D:\\Assets',
          notes: '',
        }),
      }),
    );
    expect(result.message).toBe('本地文件夹已保存');
  });

  it('saveMount 必须携带 nodeId 和相对子目录以及资产库绑定', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '挂载已保存',
          record: {
            id: 'mount-1',
            name: '上海发布会原片',
          },
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await storageNodesApi.saveMount({
      name: '上海发布会原片',
      nodeId: 'local-node-1',
      libraryId: 'photo',
      mountMode: '可写',
      heartbeatPolicy: '每小时',
      relativePath: 'ShanghaiLaunch\\RAW',
      notes: '',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:18080/api/storage/local-folders',
      expect.objectContaining({
        method: 'POST',
      }),
    );
    expect(fetchMock.mock.calls[0]?.[1]).toEqual(
      expect.objectContaining({
        body: JSON.stringify({
          name: '上海发布会原片',
          nodeId: 'local-node-1',
          libraryId: 'photo',
          libraryName: '商业摄影资产库',
          mountMode: '可写',
          heartbeatPolicy: '每小时',
          relativePath: 'ShanghaiLaunch\\RAW',
          notes: '',
        }),
      }),
    );
    expect(result.message).toBe('挂载已保存');
  });
});
