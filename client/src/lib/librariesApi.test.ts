// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createLibrary, loadLibraries } from './librariesApi';

vi.mock('./runtimeConfig', () => ({
  getRuntimeConfig: () => ({
    centerBaseUrl: 'http://127.0.0.1:8080',
  }),
}));

describe('librariesApi', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('从中心服务读取资产库列表', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: [
            {
              id: 'photo',
              name: '商业摄影资产库',
              rootLabel: '/',
              itemCount: '12',
              health: '100%',
              storagePolicy: '本地 + NAS',
            },
          ],
        }),
      }),
    );

    await expect(loadLibraries()).resolves.toEqual([
      {
        id: 'photo',
        name: '商业摄影资产库',
        rootLabel: '/',
        itemCount: '12',
        health: '100%',
        storagePolicy: '本地 + NAS',
      },
    ]);
  });

  it('通过中心服务创建资产库', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          message: '资产库已创建',
          library: {
            id: 'library-1',
            name: '全新资产库',
            rootLabel: '/',
            itemCount: '0',
            health: '100%',
            storagePolicy: '未绑定端点',
          },
        },
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(createLibrary('全新资产库')).resolves.toEqual({
      message: '资产库已创建',
      library: {
        id: 'library-1',
        name: '全新资产库',
        rootLabel: '/',
        itemCount: '0',
        health: '100%',
        storagePolicy: '未绑定端点',
      },
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/libraries',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: '全新资产库' }),
      }),
    );
  });
});
