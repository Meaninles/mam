import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { resetFileCenterMock } from './lib/fileCenterApi';
import { integrationsApi } from './lib/integrationsApi';
import { jobsApi } from './lib/jobsApi';

describe('设置页依赖服务', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    vi.restoreAllMocks();
    cleanup();
    await resetFileCenterMock();
  });

  it('在独立的依赖服务子页中展示 CloudDrive2 和 aria2 状态灯，并从导入与归档中移除 CloudDrive2 配置', async () => {
    const user = userEvent.setup();
    vi.spyOn(jobsApi, 'list').mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 100 });
    vi.spyOn(jobsApi, 'subscribe').mockReturnValue(() => {});
    vi.spyOn(integrationsApi, 'loadGateways').mockResolvedValue([
      {
        id: 'integration-gateway-cd2',
        gatewayType: 'CD2',
        displayName: 'CloudDrive2',
        baseUrl: 'http://localhost:29798',
        enabled: true,
        runtimeStatus: 'ONLINE',
        hasPassword: true,
        username: 'mare',
        lastTestAt: '2026-04-12T09:10:00Z',
      },
    ]);
    vi.spyOn(integrationsApi, 'loadRuntime').mockResolvedValue([
      {
        name: 'CloudDrive2',
        status: 'ONLINE',
        message: 'CloudDrive2 连接正常',
        lastCheckedAt: '2026-04-12T09:10:00+08:00',
        lastErrorCode: '',
      },
      {
        name: 'aria2',
        status: 'ERROR',
        message: 'aria2 启动失败',
        lastCheckedAt: '2026-04-12T09:12:00+08:00',
        lastErrorCode: 'rpc_healthcheck_failed',
        lastErrorMessage: 'RPC 健康检查未通过',
      },
    ]);

    render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(await screen.findByRole('button', { name: '依赖服务' }));

    expect(await screen.findByText('依赖服务状态')).toBeInTheDocument();
    expect(screen.getByTestId('dependency-service-indicator-CloudDrive2')).toHaveClass('success');
    expect(screen.getByTestId('dependency-service-indicator-aria2')).toHaveClass('critical');
    expect(screen.getByTestId('dependency-service-indicator-CloudDrive2')).toHaveAttribute(
      'data-tooltip',
      'CloudDrive2：连接正常',
    );
    expect(screen.getByTestId('dependency-service-indicator-aria2')).toHaveAttribute(
      'data-tooltip',
      'aria2：启动失败',
    );
    expect(screen.getByText('已保存凭据')).toBeInTheDocument();
    expect(screen.getByText('RPC 健康检查未通过')).toBeInTheDocument();
    expect(screen.getByText('rpc_healthcheck_failed')).toBeInTheDocument();
    expect(screen.getAllByText('最近检测').length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByText('在线服务数')).not.toBeInTheDocument();

    const cd2Card = screen.getAllByText('CloudDrive2')[0]?.closest('section');
    expect(cd2Card).not.toBeNull();
    expect(within(cd2Card as HTMLElement).queryByText('最近错误代码')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '导入与归档' }));
    expect(await screen.findByText('导入会话示例')).toBeInTheDocument();
    expect(screen.queryByText('CloudDrive2 集成')).not.toBeInTheDocument();
  });

  it('支持在依赖服务子页保存 CloudDrive2 配置并刷新状态', async () => {
    const user = userEvent.setup();
    vi.spyOn(jobsApi, 'list').mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 100 });
    vi.spyOn(jobsApi, 'subscribe').mockReturnValue(() => {});
    vi.spyOn(integrationsApi, 'loadGateways').mockResolvedValue([
      {
        id: 'integration-gateway-cd2',
        gatewayType: 'CD2',
        displayName: 'CloudDrive2',
        baseUrl: 'http://localhost:29798',
        enabled: true,
        runtimeStatus: 'UNKNOWN',
        hasPassword: false,
        username: '',
      },
    ]);
    vi.spyOn(integrationsApi, 'loadRuntime').mockResolvedValue([
      {
        name: 'CloudDrive2',
        status: 'UNKNOWN',
        message: 'CloudDrive2 尚未检测',
      },
      {
        name: 'aria2',
        status: 'ONLINE',
        message: 'aria2 运行正常',
      },
    ]);
    vi.spyOn(integrationsApi, 'saveCD2Gateway').mockResolvedValue({
      message: 'CloudDrive2 配置已保存',
      record: {
        id: 'integration-gateway-cd2',
        gatewayType: 'CD2',
        displayName: 'CloudDrive2',
        baseUrl: 'http://localhost:29798',
        enabled: true,
        runtimeStatus: 'ONLINE',
        hasPassword: true,
        username: 'mare',
        lastTestAt: '2026-04-12T09:10:00Z',
      },
    });
    vi.spyOn(integrationsApi, 'testCD2Gateway').mockResolvedValue({
      message: 'CloudDrive2 连接测试通过',
      record: {
        id: 'integration-gateway-cd2',
        gatewayType: 'CD2',
        displayName: 'CloudDrive2',
        baseUrl: 'http://localhost:29798',
        enabled: true,
        runtimeStatus: 'ONLINE',
        hasPassword: true,
        username: 'mare',
        lastTestAt: '2026-04-12T09:12:00Z',
      },
    });

    render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(await screen.findByRole('button', { name: '依赖服务' }));

    await user.clear(await screen.findByLabelText('CloudDrive2 账号'));
    await user.type(screen.getByLabelText('CloudDrive2 账号'), 'mare');
    await user.type(screen.getByLabelText('CloudDrive2 密码'), 'secret');
    await user.click(screen.getByRole('button', { name: '保存 CloudDrive2' }));

    await waitFor(() => {
      expect(integrationsApi.saveCD2Gateway).toHaveBeenCalledWith({
        baseUrl: 'http://localhost:29798',
        username: 'mare',
        password: 'secret',
        enabled: true,
      });
    });
    await waitFor(() => {
      expect(integrationsApi.testCD2Gateway).toHaveBeenCalledWith({
        baseUrl: 'http://localhost:29798',
        username: 'mare',
        password: 'secret',
        enabled: true,
      });
    });
    expect(await screen.findByText('CloudDrive2 配置已保存，并已确认连接正常')).toBeInTheDocument();
    expect(screen.getByTestId('dependency-service-indicator-CloudDrive2')).toHaveClass('success');
    expect(screen.getByText('已保存凭据')).toBeInTheDocument();
  });
});
