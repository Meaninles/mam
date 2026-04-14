import '@testing-library/jest-dom/vitest';
import { beforeEach, vi } from 'vitest';

beforeEach(() => {
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
      if (/\/api\/libraries\/[^/]+\/browse/.test(url)) {
        return {
          ok: true,
          json: async () => ({
            data: {
              breadcrumbs: [{ id: null, label: '商业摄影资产库' }],
              items: [
                {
                  id: 'dir-raw',
                  libraryId: 'photo',
                  parentId: null,
                  type: 'folder',
                  lifecycleState: 'ACTIVE',
                  name: '拍摄原片',
                  fileKind: '文件夹',
                  displayType: '文件夹',
                  modifiedAt: '今天 09:18',
                  createdAt: '2026-04-12 09:18',
                  size: '12 项',
                  path: '商业摄影资产库 / 拍摄原片',
                  sourceLabel: '统一目录树',
                  lastTaskText: '暂无任务',
                  lastTaskTone: 'info',
                  rating: 0,
                  colorLabel: '无',
                  badges: [],
                  riskTags: [],
                  tags: [],
                  endpoints: [
                    {
                      name: '本地NVMe',
                      state: '已同步',
                      tone: 'success',
                      lastSyncAt: '今天 09:18',
                      endpointType: 'local',
                    },
                    {
                      name: '影像NAS',
                      state: '部分同步',
                      tone: 'warning',
                      lastSyncAt: '今天 09:22',
                      endpointType: 'nas',
                    },
                    {
                      name: '115',
                      state: '未同步',
                      tone: 'critical',
                      lastSyncAt: '尚未开始',
                      endpointType: 'cloud',
                    },
                  ],
                },
                {
                  id: 'asset-cover',
                  libraryId: 'photo',
                  parentId: null,
                  type: 'file',
                  lifecycleState: 'ACTIVE',
                  name: 'cover.jpg',
                  fileKind: '图片',
                  displayType: 'JPG 图片',
                  modifiedAt: '今天 09:30',
                  createdAt: '2026-04-12 09:30',
                  size: '12 MB',
                  path: '商业摄影资产库 / cover.jpg',
                  sourceLabel: '统一资产',
                  lastTaskText: '等待同步到 115',
                  lastTaskTone: 'warning',
                  rating: 4,
                  colorLabel: '红标',
                  badges: ['RAW'],
                  riskTags: ['待同步'],
                  tags: ['封面图'],
                  endpoints: [
                    {
                      name: '本地NVMe',
                      state: '已同步',
                      tone: 'success',
                      lastSyncAt: '今天 09:18',
                      endpointType: 'local',
                    },
                    {
                      name: '影像NAS',
                      state: '同步中',
                      tone: 'warning',
                      lastSyncAt: '刚刚',
                      endpointType: 'nas',
                    },
                    {
                      name: '115',
                      state: '未同步',
                      tone: 'critical',
                      lastSyncAt: '尚未开始',
                      endpointType: 'cloud',
                    },
                  ],
                },
              ],
              total: 2,
              currentPathChildren: 2,
              endpointNames: ['本地NVMe', '影像NAS', '115'],
            },
          }),
        } as Response;
      }

      if (/\/api\/file-entries\/[^/]+$/.test(url)) {
        return {
          ok: true,
          json: async () => ({
            data: {
              id: 'asset-cover',
              libraryId: 'photo',
              parentId: null,
              type: 'file',
              lifecycleState: 'ACTIVE',
              name: 'cover.jpg',
              fileKind: '图片',
              displayType: 'JPG 图片',
              modifiedAt: '今天 09:30',
              createdAt: '2026-04-12 09:30',
              size: '12 MB',
              path: '商业摄影资产库 / cover.jpg',
              sourceLabel: '统一资产',
              lastTaskText: '等待同步到 115',
              lastTaskTone: 'warning',
              rating: 4,
              colorLabel: '红标',
              badges: ['RAW'],
              riskTags: ['待同步'],
              tags: ['封面图'],
              endpoints: [
                {
                  name: '本地NVMe',
                  state: '已同步',
                  tone: 'success',
                  lastSyncAt: '今天 09:18',
                  endpointType: 'local',
                },
                {
                  name: '影像NAS',
                  state: '同步中',
                  tone: 'warning',
                  lastSyncAt: '刚刚',
                  endpointType: 'nas',
                },
                {
                  name: '115',
                  state: '未同步',
                  tone: 'critical',
                  lastSyncAt: '尚未开始',
                  endpointType: 'cloud',
                },
              ],
            },
          }),
        } as Response;
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

      if (url.includes('/api/notifications')) {
        return {
          ok: true,
          json: async () => ({
            data: {
              items: [],
              total: 0,
              page: 1,
              pageSize: 50,
            },
          }),
        } as Response;
      }

      if (url.includes('/api/issues/by-jobs')) {
        return {
          ok: true,
          json: async () => ({
            data: {
              items: [],
            },
          }),
        } as Response;
      }

      if (url.includes('/api/issues')) {
        return {
          ok: true,
          json: async () => ({
            data: {
              items: [],
              total: 0,
              page: 1,
              pageSize: 20,
            },
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
