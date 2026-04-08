// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchSystemRuntimeStatus } from './systemRuntimeApi';

describe('systemRuntimeApi', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('maps ready center and online agent to success summary', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            status: 'ready',
            service: { name: 'mare-center', version: 'dev', status: 'up', startedAt: '2026-04-08T14:22:33Z' },
            database: { status: 'up', message: '数据库连接正常' },
            migration: { status: 'ready', currentVersion: 1, latestVersion: 1 },
            agents: [
              {
                agentId: 'agent-dev-1',
                version: 'dev',
                hostname: 'DESKTOP-A',
                platform: 'windows/amd64',
                mode: 'attached',
                processId: 1001,
                status: 'online',
                registeredAt: '2026-04-08T14:22:37Z',
                lastHeartbeatAt: '2026-04-08T14:22:41Z',
              },
            ],
            timestamp: '2026-04-08T14:22:42Z',
          },
          timestamp: '2026-04-08T14:22:42Z',
        }),
      }),
    );

    await expect(fetchSystemRuntimeStatus('http://127.0.0.1:8080')).resolves.toEqual({
      agentStatus: 'online',
      centerStatus: 'ready',
      lightTone: 'success',
    });
  });

  it('maps ready center and no online agent to warning summary', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            status: 'ready',
            agents: [
              {
                agentId: 'agent-dev-1',
                status: 'heartbeat_timeout',
              },
            ],
          },
        }),
      }),
    );

    await expect(fetchSystemRuntimeStatus('http://127.0.0.1:8080')).resolves.toEqual({
      agentStatus: 'heartbeat_timeout',
      centerStatus: 'ready',
      lightTone: 'warning',
    });
  });

  it('maps fetch failures to critical summary', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network failed')));

    await expect(fetchSystemRuntimeStatus('http://127.0.0.1:8080')).resolves.toEqual({
      agentStatus: 'unknown',
      centerStatus: 'unreachable',
      lightTone: 'critical',
    });
  });
});
