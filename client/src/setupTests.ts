import '@testing-library/jest-dom/vitest';
import { beforeEach, vi } from 'vitest';

beforeEach(() => {
  (window as Window & { __MARE_ENABLE_FILE_CENTER_MOCK_SYNC__?: boolean }).__MARE_ENABLE_FILE_CENTER_MOCK_SYNC__ = true;
  class MockEventSource {
    onmessage: ((event: MessageEvent<string>) => void) | null = null;

    addEventListener() {
      return undefined;
    }

    close() {
      return undefined;
    }
  }

  vi.stubGlobal('EventSource', MockEventSource);
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('sql-wasm.wasm')) {
        const { readFile } = await import('node:fs/promises');
        const { resolve } = await import('node:path');
        const wasmBinary = await readFile(resolve(process.cwd(), 'node_modules/sql.js/dist/sql-wasm.wasm'));
        return new Response(wasmBinary, { status: 200 });
      }
      if (url.includes('/api/libraries')) {
        return {
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'photo',
                name: '商业摄影资产库',
                rootLabel: '/',
                itemCount: '0',
                health: '100%',
                storagePolicy: '本地 + NAS',
              },
              {
                id: 'video',
                name: '视频工作流资产库',
                rootLabel: '/',
                itemCount: '0',
                health: '100%',
                storagePolicy: '本地 + NAS',
              },
              {
                id: 'family',
                name: '家庭照片资产库',
                rootLabel: '/',
                itemCount: '0',
                health: '100%',
                storagePolicy: '本地 + NAS',
              },
            ],
          }),
        } as Response;
      }

      if (url.includes('/api/storage/local-nodes')) {
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
                lastCheckAt: '今天 09:12',
                capacitySummary: '待检测',
                freeSpaceSummary: '待检测',
                capacityPercent: 0,
                mountCount: 1,
                notes: '',
              },
            ],
          }),
        } as Response;
      }

      if (url.includes('/api/storage/local-folders')) {
        return {
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'mount-1',
                name: '商业摄影原片库',
                libraryId: 'photo',
                libraryName: '商业摄影资产库',
                folderType: '本地',
                nodeId: 'local-node-1',
                nodeName: '本地素材根目录',
                nodeRootPath: 'D:\\Assets',
                relativePath: 'RAW',
                address: 'D:\\Assets\\RAW',
                mountMode: '可写',
                enabled: true,
                scanStatus: '最近扫描成功',
                scanTone: 'success',
                lastScanAt: '今天 09:12',
                heartbeatPolicy: '从不',
                nextHeartbeatAt: '—',
                capacitySummary: '待检测',
                freeSpaceSummary: '待检测',
                capacityPercent: 0,
                riskTags: [],
                badges: ['本地', '可写'],
                authStatus: '无需鉴权',
                authTone: 'info',
                notes: '',
              },
            ],
          }),
        } as Response;
      }

      if (url.includes('/api/storage/nas-nodes')) {
        if (url.includes('/connection-test')) {
          return {
            ok: true,
            json: async () => ({
              data: {
                message: '连接测试已完成',
                results: [
                  {
                    id: 'nas-node-1',
                    name: '影像 NAS 01',
                    overallTone: 'success',
                    summary: 'SMB 鉴权正常',
                    checks: [],
                    testedAt: '刚刚',
                  },
                ],
              },
            }),
          } as Response;
        }
        return {
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
                mountCount: 1,
                notes: '',
              },
            ],
          }),
        } as Response;
      }

      if (url.includes('/api/storage/cloud-nodes')) {
        if (url.includes('/connection-test')) {
          return {
            ok: true,
            json: async () => ({
              data: {
                message: '连接测试已完成',
                results: [
                  {
                    id: 'cloud-node-1',
                    name: '115 云归档',
                    overallTone: 'success',
                    summary: '鉴权正常',
                    checks: [],
                    testedAt: '刚刚',
                  },
                ],
              },
            }),
          } as Response;
        }
        return {
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'cloud-node-1',
                name: '115 云归档',
                vendor: '115',
                accessMethod: '填入 Token',
                mountPath: '/MareArchive',
                tokenStatus: '已配置',
                lastTestAt: '今天 10:20',
                status: '鉴权正常',
                tone: 'success',
                mountCount: 1,
                notes: '',
              },
            ],
          }),
        } as Response;
      }

      if (url.includes('/api/storage/local-folders/connection-test')) {
        return {
          ok: true,
          json: async () => ({
            data: {
              message: '连接测试已完成',
              results: [
                {
                  id: 'mount-1',
                  name: '商业摄影原片库',
                  overallTone: 'success',
                  summary: '目录可访问',
                  checks: [],
                  testedAt: '刚刚',
                },
              ],
            },
          }),
        } as Response;
      }

      if (url.includes('/api/file-entries/') && url.endsWith('/annotations')) {
        return {
          ok: true,
          json: async () => ({
            data: {
              message: '资产标记已更新',
            },
          }),
        } as Response;
      }

      if (url.includes('/api/tags/suggestions')) {
        return {
          ok: true,
          json: async () => ({
            data: [
              {
                id: 'tag-publish',
                name: '发布会',
                count: 8,
                groupName: '未分组',
                isPinned: true,
                libraryIds: ['photo'],
              },
              {
                id: 'tag-live',
                name: '直播切片',
                count: 5,
                groupName: '未分组',
                isPinned: false,
                libraryIds: ['photo'],
              },
            ],
          }),
        } as Response;
      }

      return {
        ok: false,
        status: 503,
        json: async () => ({
          error: {
            message: 'test fetch not implemented',
          },
        }),
      } as Response;
    }),
  );
});
