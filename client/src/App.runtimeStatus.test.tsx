// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { resetFileCenterMock } from './lib/fileCenterApi';

declare global {
  interface Window {
    __MARE_RUNTIME_CONFIG__?: {
      centerBaseUrl?: string;
      runtimeStatusEnabled?: boolean;
      runtimeStatusPollMs?: number;
    };
  }
}

describe('App runtime indicator', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    window.__MARE_RUNTIME_CONFIG__ = {
      centerBaseUrl: 'http://127.0.0.1:18080',
      runtimeStatusEnabled: true,
      runtimeStatusPollMs: 60000,
    };
    await resetFileCenterMock();
  });

  afterEach(async () => {
    cleanup();
    delete window.__MARE_RUNTIME_CONFIG__;
    vi.unstubAllGlobals();
    await resetFileCenterMock();
  });

  it('renders a compact green light before the import-entry button when center and agent are healthy', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            status: 'ready',
            agents: [{ agentId: 'agent-dev-1', status: 'online' }],
          },
        }),
      }),
    );

    const { container } = render(<App />);

    const indicator = await screen.findByTestId('system-runtime-indicator');
    expect(indicator).toHaveClass('success');
    expect(indicator).toHaveTextContent('');
    expect(indicator).toHaveAttribute('aria-label', '系统状态：中心服务可用，本地执行器在线');
    expect(indicator).toHaveAttribute('data-tooltip', '中心服务：正常 · 本地执行器：在线');

    const actions = container.querySelector('.page-header-actions');
    const importChip = container.querySelector('.import-entry-chip');
    expect(actions).not.toBeNull();
    expect(importChip).not.toBeNull();
    expect(actions?.children[0]).toBe(indicator);
    expect(actions?.children[1]).toBe(importChip);
  });

  it('renders a yellow light when center is ready but agent heartbeat timed out', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          data: {
            status: 'ready',
            agents: [{ agentId: 'agent-dev-1', status: 'heartbeat_timeout' }],
          },
        }),
      }),
    );

    render(<App />);

    await waitFor(() => {
      expect(screen.getByTestId('system-runtime-indicator')).toHaveClass('warning');
    });
    expect(screen.getByTestId('system-runtime-indicator')).toHaveAttribute(
      'data-tooltip',
      '中心服务：正常 · 本地执行器：异常',
    );
  });
});
