import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import App from './App';
import { resetFileCenterMock } from './lib/fileCenterApi';

describe('MARE 客户端', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    cleanup();
    await resetFileCenterMock();
  });

  it('支持进入目录并查看文件详情', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    expect(await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).toBeInTheDocument();

    await user.dblClick(screen.getByText('2026-03-29_上海发布会_A-cam_001.RAW'));
    const detailSheet = await screen.findByRole('region', { name: '2026-03-29_上海发布会_A-cam_001.RAW' });
    expect(within(detailSheet).getAllByText('Sony A7R V').length).toBeGreaterThan(0);
  });

  it('支持提交导入批次并生成任务', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '导入中心' }));
    await user.selectOptions(screen.getByLabelText('导入批次'), '录音卡 / 访谈音频');
    await user.click(screen.getByRole('button', { name: '提交导入任务' }));

    expect(screen.getByText('已提交导入批次，任务已加入队列')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    expect(screen.getByText('录音卡 / 访谈音频 入库')).toBeInTheDocument();
  });

  it('支持按建议处理异常并生成修复任务', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '异常中心' }));
    await user.click(screen.getByRole('button', { name: '按建议处理 客户访谈_第一机位_精编版.mov' }));

    expect(screen.getByText('已创建异常处理任务')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    expect(screen.getByText('修复：客户访谈_第一机位_精编版.mov')).toBeInTheDocument();
  });

  it('支持删除资产并进入等待清理状态', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.dblClick(await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW'));
    await user.click(screen.getByRole('button', { name: '更多操作 2026-03-29_上海发布会_A-cam_001.RAW' }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '删除资产' }));
    expect(await screen.findByRole('dialog', { name: '确认删除资产' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认删除' }));

    expect(await screen.findByText('删除请求已提交，资产进入等待清理')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    expect(screen.getAllByText('删除资产：2026-03-29_上海发布会_A-cam_001.RAW').length).toBeGreaterThan(0);
  });

  it('支持保存外观设置并持久化主题', async () => {
    const user = userEvent.setup();
    const { container, unmount } = render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(screen.getByRole('button', { name: '外观' }));
    await user.click(screen.getByRole('button', { name: '浅色主题' }));
    await user.click(screen.getByRole('button', { name: '保存设置' }));

    expect(container.firstChild).toHaveClass('theme-light');

    unmount();
    render(<App />);

    expect(document.querySelector('.app-shell')).toHaveClass('theme-light');
  });

  it('支持从指定端点删除且保留资产', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    const row = (await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row!).getByRole('button', { name: '更多操作 2026-03-29_上海发布会_A-cam_001.RAW' }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '影像NAS' }));
    expect(await screen.findByRole('dialog', { name: '确认删除副本' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认删除' }));
    expect(await screen.findByText('已提交端点删除请求')).toBeInTheDocument();
  });

  it('支持点击端点状态发起同步确认', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    const row = (await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row!).getByRole('button', { name: '115 未同步' }));
    expect(await screen.findByRole('dialog', { name: '确认同步' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认同步' }));

    expect(await screen.findByText('已创建同步任务到 115')).toBeInTheDocument();
  });

  it('支持进入存储节点页并执行连接测试', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '存储节点' }));

    expect(await screen.findByRole('button', { name: '挂载文件夹管理' })).toBeInTheDocument();

    const row = await screen.findByRole('row', { name: /视频工作流 NAS 挂载/ });
    await user.click(within(row).getByRole('button', { name: '连接测试 视频工作流 NAS 挂载' }));

    expect(await screen.findByRole('dialog', { name: '连接测试结果' })).toBeInTheDocument();
    expect(screen.getByText('挂载目录可达且当前配置可继续使用。')).toBeInTheDocument();
  });

  it('支持通知标记已读并清除红点', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(document.querySelector('.notification-dot')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: '通知' }));
    expect(await screen.findByText('检测到移动硬盘 SanDisk Extreme 2TB')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '标记已读' }));

    expect(screen.getByText('已读')).toBeInTheDocument();
    expect(screen.getByText('检测到移动硬盘 SanDisk Extreme 2TB')).toBeInTheDocument();
    expect(document.querySelector('.notification-dot')).toBeFalsy();
  });
});
