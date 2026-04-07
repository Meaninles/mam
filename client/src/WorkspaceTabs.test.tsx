import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import App from './App';
import { resetFileCenterMock } from './lib/fileCenterApi';

describe('客户端顶层多标签工作区', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    cleanup();
    await resetFileCenterMock();
  });

  it('点击左侧导航会打开或聚焦唯一的一级页面标签，不会重复新开', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByRole('tab', { name: '文件中心' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    expect(screen.getByRole('tab', { name: '任务中心' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    expect(screen.getAllByRole('tab', { name: '任务中心' })).toHaveLength(1);
  });

  it('切换标签后保留页面内部状态', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '存储节点' }));
    const storageSearch = await screen.findByRole('textbox', { name: '搜索存储项' });
    await user.type(storageSearch, 'NAS');
    expect(storageSearch).toHaveValue('NAS');

    await user.click(screen.getByRole('button', { name: '文件中心' }));
    await user.click(screen.getByRole('tab', { name: '存储节点' }));

    expect(await screen.findByRole('textbox', { name: '搜索存储项' })).toHaveValue('NAS');
  });

  it('支持标签右键关闭其它标签与最近关闭恢复', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '异常中心' }));

    const issuesTab = screen.getByRole('tab', { name: '异常中心' });
    await user.pointer([{ target: issuesTab, keys: '[MouseRight]' }]);

    const contextMenu = await screen.findByRole('menu', { name: '标签页操作' });
    await user.click(within(contextMenu).getByRole('menuitem', { name: '关闭其他标签' }));

    await waitFor(() => {
      expect(screen.getAllByRole('tab')).toHaveLength(1);
    });
    expect(screen.getByRole('tab', { name: '异常中心' })).toBeInTheDocument();

    const activeTab = screen.getByRole('tab', { name: '异常中心' });
    await user.pointer([{ target: activeTab, keys: '[MouseRight]' }]);
    const reopenMenu = await screen.findByRole('menu', { name: '标签页操作' });
    await user.click(within(reopenMenu).getByRole('menuitem', { name: '重新打开刚关闭的标签' }));

    expect(await screen.findByRole('tab', { name: '任务中心' })).toBeInTheDocument();
  });

  it('支持直接拖拽整个标签框体进行排序', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '异常中心' }));

    const fileTab = screen.getByRole('tab', { name: '文件中心' });
    const taskTab = screen.getByRole('tab', { name: '任务中心' });

    const dataTransfer = {
      dropEffect: 'move',
      effectAllowed: 'move',
      files: [],
      items: [],
      types: [],
      clearData: () => {},
      getData: () => '',
      setData: () => {},
      setDragImage: () => {},
    };

    fireEvent.dragStart(taskTab, { dataTransfer });
    fireEvent.dragOver(fileTab, { dataTransfer });
    fireEvent.drop(fileTab, { dataTransfer });
    fireEvent.dragEnd(taskTab, { dataTransfer });

    expect(screen.getAllByRole('tab').map((item) => item.textContent?.trim())).toEqual([
      '任务中心',
      '文件中心',
      '异常中心',
    ]);
  });
});
