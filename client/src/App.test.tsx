import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import App from './App';

describe('MARE 客户端', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('支持进入目录并查看文件详情', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(screen.getByText('拍摄原片'));
    expect(screen.getByText('2026-03-29_上海发布会_A-cam_001.RAW')).toBeInTheDocument();

    await user.dblClick(screen.getByText('2026-03-29_上海发布会_A-cam_001.RAW'));
    expect(screen.getByRole('region', { name: '2026-03-29_上海发布会_A-cam_001.RAW' })).toBeInTheDocument();
    expect(screen.getByText('Sony A7R V')).toBeInTheDocument();
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

    await user.dblClick(screen.getByText('拍摄原片'));
    await user.dblClick(screen.getByText('2026-03-29_上海发布会_A-cam_001.RAW'));
    await user.click(screen.getAllByRole('button', { name: '删除资产' }).at(-1)!);

    expect(screen.getByText('删除请求已提交，资产进入等待清理')).toBeInTheDocument();

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

    await user.dblClick(screen.getByText('拍摄原片'));
    await user.dblClick(screen.getByText('2026-03-29_上海发布会_A-cam_001.RAW'));
    await user.click(screen.getByRole('button', { name: '从 115 删除' }));

    expect(screen.getByText('已提交端点删除请求')).toBeInTheDocument();
    expect(screen.getByText('缺失')).toBeInTheDocument();
    expect(screen.getAllByText('2026-03-29_上海发布会_A-cam_001.RAW').length).toBeGreaterThan(0);
  });

  it('支持进入存储节点页并执行连接测试', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '存储节点' }));

    expect(await screen.findByLabelText('按类型筛选')).toBeInTheDocument();

    const row = await screen.findByRole('row', { name: /115 云归档/ });
    await user.click(within(row).getByRole('button', { name: '连接测试 115 云归档' }));

    expect(await screen.findByRole('dialog', { name: '连接测试结果' })).toBeInTheDocument();
    expect(screen.getByText('当前连接可达，但仍建议先处理风险提示。')).toBeInTheDocument();
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
