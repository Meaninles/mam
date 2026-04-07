import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { createInitialState } from '../lib/clientState';
import { ImportCenterPage } from './ImportCenterPage';

function renderImportCenter() {
  const state = createInitialState();

  return render(
    <ImportCenterPage
      devices={state.importDeviceSessions}
      drafts={state.importDrafts}
      issues={state.issueRecords}
      reports={state.importReports}
      sourceNodes={state.importSourceNodes}
      targetEndpoints={state.importTargetEndpoints}
      onApplyTargetToAll={vi.fn()}
      onRemoveTargetFromAll={vi.fn()}
      onOpenFileCenter={vi.fn()}
      onOpenIssueCenter={vi.fn()}
      onOpenStorageNodes={vi.fn()}
      onOpenTaskCenter={vi.fn()}
      onRefreshDevices={vi.fn()}
      onRefreshPrecheck={vi.fn()}
      onRescanDevice={vi.fn()}
      onSaveDraft={vi.fn()}
      onSetSourceTargets={vi.fn()}
      onSubmitImport={vi.fn(() => 'new-report-id')}
    />,
  );
}

describe('ImportCenterPage', () => {
  it('支持从待导入端列表打开设备会话并显示来源摘要', async () => {
    renderImportCenter();

    const deviceRow = (await screen.findByText('CFexpress A 卡（A 机位）')).closest('article');
    expect(deviceRow).not.toBeNull();
    expect(within(deviceRow!).getByRole('button', { name: '打开会话 CFexpress A 卡（A 机位）' })).toBeInTheDocument();
    expect(within(deviceRow!).getByRole('button', { name: '重新扫描 CFexpress A 卡（A 机位）' })).toBeInTheDocument();
    expect(within(deviceRow!).getByRole('button', { name: '查看预检 CFexpress A 卡（A 机位）' })).toBeInTheDocument();
    expect(within(deviceRow!).getByRole('button', { name: '查看详情 CFexpress A 卡（A 机位）' })).toBeInTheDocument();

    await userEvent.setup().click(deviceRow!);

    expect(await screen.findByText('来源路径')).toBeInTheDocument();
    expect(screen.getByText('目标编排')).toBeInTheDocument();
    expect(screen.getByText('文件清单')).toBeInTheDocument();
  });

  it('为可导入会话点击提交导入时会先弹出确认弹窗', async () => {
    const user = userEvent.setup();
    renderImportCenter();

    const deviceRow = (await screen.findByText('CFexpress A 卡（A 机位）')).closest('article');
    expect(deviceRow).not.toBeNull();
    await user.click(deviceRow!);

    await user.click(screen.getByRole('button', { name: '提交导入' }));

    expect(await screen.findByRole('dialog', { name: '确认提交导入' })).toBeInTheDocument();
    expect(screen.getByText(/提交后会正式创建导入作业/)).toBeInTheDocument();
  });

  it('目标编排支持全部应用和全部取消，并展示导入覆盖状态', async () => {
    const user = userEvent.setup();
    const state = createInitialState();
    const onApplyTargetToAll = vi.fn();
    const onRemoveTargetFromAll = vi.fn();

    render(
      <ImportCenterPage
        devices={state.importDeviceSessions}
        drafts={state.importDrafts}
        issues={state.issueRecords}
        reports={state.importReports}
        sourceNodes={state.importSourceNodes}
        targetEndpoints={state.importTargetEndpoints}
        onApplyTargetToAll={onApplyTargetToAll}
        onRemoveTargetFromAll={onRemoveTargetFromAll}
        onOpenFileCenter={vi.fn()}
        onOpenIssueCenter={vi.fn()}
        onOpenStorageNodes={vi.fn()}
        onOpenTaskCenter={vi.fn()}
        onRefreshDevices={vi.fn()}
        onRefreshPrecheck={vi.fn()}
        onRescanDevice={vi.fn()}
        onSaveDraft={vi.fn()}
        onSetSourceTargets={vi.fn()}
        onSubmitImport={vi.fn(() => 'new-report-id')}
      />,
    );

    const deviceRow = (await screen.findByText('CFexpress A 卡（A 机位）')).closest('article');
    expect(deviceRow).not.toBeNull();
    await user.click(deviceRow!);

    const targetCard = screen.getAllByText('影像 NAS 01')[0]?.closest('.import-target-card');
    expect(targetCard).not.toBeNull();
    expect(targetCard).toHaveClass('partial');

    await user.click(within(targetCard as HTMLElement).getByRole('button', { name: '应用到全部文件' }));
    expect(onApplyTargetToAll).toHaveBeenCalled();

    await user.click(within(targetCard as HTMLElement).getByRole('button', { name: '全部取消' }));
    expect(onRemoveTargetFromAll).toHaveBeenCalled();
  });
});
