import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ImportCenterPage } from './ImportCenterPage';
import {
  importFixtureBrowserState,
  importFixtureDevices,
  importFixtureDrafts,
  importFixtureLibraries,
  importFixtureReports,
  importFixtureTargets,
} from '../test/importFixtures';

const primaryDeviceLabel = importFixtureDevices[0].deviceLabel;

function renderImportCenter() {
  return render(
    <ImportCenterPage
      libraries={importFixtureLibraries}
      devices={importFixtureDevices}
      drafts={importFixtureDrafts}
      issues={[]}
      reports={importFixtureReports}
      targetEndpoints={importFixtureTargets}
      browserState={importFixtureBrowserState}
      browserLoading={false}
      onBrowseSession={vi.fn()}
      onOpenFolder={vi.fn()}
      onGoToParentFolder={vi.fn()}
      onOpenFileCenter={vi.fn()}
      onOpenIssueCenter={vi.fn()}
      onOpenStorageNodes={vi.fn()}
      onOpenTaskCenter={vi.fn()}
      onRefreshDevices={vi.fn()}
      onSelectLibrary={vi.fn()}
      onRefreshPrecheck={vi.fn()}
      onSaveSelectionTargets={vi.fn()}
      onSubmitImport={vi.fn()}
    />,
  );
}

describe('ImportCenterPage', () => {
  it('支持从待导入设备列表打开会话并显示来源摘要', async () => {
    renderImportCenter();

    const deviceRow = (await screen.findAllByText(primaryDeviceLabel))[0]?.closest('article');
    expect(deviceRow).not.toBeNull();
    expect(within(deviceRow!).getByRole('button', { name: `打开会话 ${primaryDeviceLabel}` })).toBeInTheDocument();
    expect(within(deviceRow!).getByRole('button', { name: `查看预检 ${primaryDeviceLabel}` })).toBeInTheDocument();
    expect(within(deviceRow!).getByRole('button', { name: `查看详情 ${primaryDeviceLabel}` })).toBeInTheDocument();

    await userEvent.setup().click(deviceRow!);

    expect(await screen.findByText('来源路径')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '/' })).toBeInTheDocument();
    expect((await screen.findAllByText('A001')).length).toBeGreaterThan(0);
  });

  it('为可导入会话点击提交导入时会先弹出确认弹窗', async () => {
    const user = userEvent.setup();
    renderImportCenter();

    const deviceRow = (await screen.findAllByText(primaryDeviceLabel))[0]?.closest('article');
    expect(deviceRow).not.toBeNull();
    await user.click(deviceRow!);

    await user.click(screen.getByRole('button', { name: '提交导入' }));

    expect(await screen.findByRole('dialog', { name: '确认提交导入' })).toBeInTheDocument();
    expect(screen.getByText(/提交后会在后台递归扫描所选文件夹和文件/)).toBeInTheDocument();
  });

  it('当前层级浏览支持为目录和文件分配本地或 NAS 目标端', async () => {
    const user = userEvent.setup();
    const onSaveSelectionTargets = vi.fn();

    render(
      <ImportCenterPage
        libraries={importFixtureLibraries}
        devices={importFixtureDevices}
        drafts={importFixtureDrafts}
        issues={[]}
        reports={importFixtureReports}
        targetEndpoints={importFixtureTargets}
        browserState={importFixtureBrowserState}
        browserLoading={false}
        onBrowseSession={vi.fn()}
        onOpenFolder={vi.fn()}
        onGoToParentFolder={vi.fn()}
        onOpenFileCenter={vi.fn()}
        onOpenIssueCenter={vi.fn()}
        onOpenStorageNodes={vi.fn()}
        onOpenTaskCenter={vi.fn()}
        onRefreshDevices={vi.fn()}
        onSelectLibrary={vi.fn()}
        onRefreshPrecheck={vi.fn()}
        onSaveSelectionTargets={onSaveSelectionTargets}
        onSubmitImport={vi.fn()}
      />,
    );

    const deviceRow = (await screen.findAllByText(primaryDeviceLabel))[0]?.closest('article');
    expect(deviceRow).not.toBeNull();
    await user.click(deviceRow!);

    const row = (await screen.findAllByText('cover.jpg'))[0]?.closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLElement).getByLabelText('影像 NAS 01'));
    expect(onSaveSelectionTargets).toHaveBeenCalled();
  });

  it('不展示导入到网盘的目标端，只保留本地与 NAS 目标', async () => {
    const user = userEvent.setup();
    renderImportCenter();

    const deviceRow = (await screen.findAllByText(primaryDeviceLabel))[0]?.closest('article');
    expect(deviceRow).not.toBeNull();
    await user.click(deviceRow!);

    const row = (await screen.findAllByText('cover.jpg'))[0]?.closest('tr');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).queryByLabelText('115 云归档')).toBeNull();
    expect(within(row as HTMLElement).getByLabelText('本地 NVMe 主盘')).toBeInTheDocument();
    expect(within(row as HTMLElement).getByLabelText('影像 NAS 01')).toBeInTheDocument();
  });
});
