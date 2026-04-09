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
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'nas-node-1',
                name: '影像 NAS 01',
                address: '\\\\192.168.10.20\\media',
                accessMode: 'SMB',
                username: 'mare-sync',
                passwordHint: '已保存',
                lastTestAt: '今天 10:20',
                status: '鉴权正常',
                tone: 'success',
                mountCount: 2,
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
    expect(result.nasNodes).toHaveLength(1);
    expect(result.nasNodes[0]?.accessMode).toBe('SMB');
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
      libraryName: '商业摄影资产库',
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
    expect(result.record.id).toBe('mount-1');
  });

  it('saveNasNode 改为调用中心服务接口', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: 'NAS 已保存',
          record: {
            id: 'nas-node-1',
            name: '影像 NAS 01',
          },
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await storageNodesApi.saveNasNode({
      name: '影像 NAS 01',
      address: '\\\\192.168.10.20\\media',
      username: 'mare-sync',
      password: 'secret',
      notes: '',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:18080/api/storage/nas-nodes',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          name: '影像 NAS 01',
          address: '\\\\192.168.10.20\\media',
          username: 'mare-sync',
          password: 'secret',
          notes: '',
        }),
      }),
    );
    expect(result.message).toBe('NAS 已保存');
  });

  it('loadDashboard 在单个接口失败时仍返回其余已成功的数据', async () => {
    vi.spyOn(window, 'setTimeout').mockImplementation(((handler: TimerHandler) => {
      if (typeof handler === 'function') {
        handler();
      }
      return 0 as unknown as number;
    }) as typeof window.setTimeout);

    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/storage/local-nodes')) {
        return {
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
        };
      }
      if (url.endsWith('/api/storage/local-folders')) {
        return {
          ok: true,
          json: async () => ({
            data: [],
          }),
        };
      }
      throw new Error('nas endpoint unavailable');
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await storageNodesApi.loadDashboard();

    expect(result.localNodes).toHaveLength(1);
    expect(result.nasNodes).toHaveLength(0);
    expect(result.mounts).toHaveLength(0);
  });
});

describe('storageNodesApi cloud mapping', () => {
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

  it('maps cloud mountPath into mountDirectory', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/storage/local-nodes')) {
        return { ok: true, json: async () => ({ data: [] }) };
      }
      if (url.endsWith('/api/storage/local-folders')) {
        return { ok: true, json: async () => ({ data: [] }) };
      }
      if (url.endsWith('/api/storage/nas-nodes')) {
        return { ok: true, json: async () => ({ data: [] }) };
      }
      if (url.endsWith('/api/storage/cloud-nodes')) {
        return {
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'cloud-node-1',
                name: '115 云归档',
                vendor: '115',
                accessMethod: '扫码登录获取 Token',
                qrChannel: 'tv',
                mountPath: '/MareArchive/Projects/Shanghai',
                tokenStatus: '已配置',
                status: '鉴权正常',
                tone: 'success',
                mountCount: 1,
                notes: '',
              },
            ],
          }),
        };
      }
      throw new Error(`unexpected url: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await storageNodesApi.loadDashboard();

    expect(result.cloudNodes).toHaveLength(1);
    expect(result.cloudNodes[0]?.mountDirectory).toBe('/MareArchive/Projects/Shanghai');
  });
});
