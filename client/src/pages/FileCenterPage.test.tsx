import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { FileCenterEntry } from '../lib/fileCenterApi';
import { FileCenterPage } from './FileCenterPage';

const sampleEntry: FileCenterEntry = {
  id: 'photo-file-raw-002',
  libraryId: 'photo',
  parentId: 'photo-root-raw',
  type: 'file',
  lifecycleState: 'ACTIVE',
  name: '2026-03-29_上海发布会_B-cam_018.RAW',
  fileKind: '图片',
  displayType: 'RAW 图像',
  modifiedAt: '2026-03-29 18:49',
  createdAt: '2026-03-29 18:49',
  size: '47.8 MB',
  path: '商业摄影资产库 / 拍摄原片 / 2026-03-29_上海发布会_B-cam_018.RAW',
  sourceLabel: 'Sony A7 IV',
  notes: '',
  lastTaskText: 'NAS 补齐进行中',
  lastTaskTone: 'warning',
  rating: 0,
  colorLabel: '无',
  badges: ['RAW'],
  riskTags: ['待同步'],
  tags: ['发布会'],
  endpoints: [
    { name: '本地NVMe', state: '已同步', tone: 'success', lastSyncAt: '今天 09:18', endpointType: 'local' },
    { name: '影像NAS', state: '同步中', tone: 'warning', lastSyncAt: '刚刚', endpointType: 'nas' },
    { name: '115', state: '未同步', tone: 'critical', lastSyncAt: '尚未开始', endpointType: 'cloud' },
  ],
  metadata: [{ label: '设备', value: 'Sony A7 IV' }],
};

describe('FileCenterPage', () => {
  it('统一使用三种存储状态，并将不可同步端点灰化禁用', () => {
    render(
      <FileCenterPage
        breadcrumbs={[{ id: null, label: '商业摄影资产库' }]}
        batchDeleteEndpointActions={[]}
        batchSyncEndpointActions={[]}
        canGoBack={false}
        canGoForward={false}
        currentEntries={[sampleEntry]}
        currentPage={1}
        currentPathChildren={1}
        fileTypeFilter="全部"
        loading={false}
        pageCount={1}
        pageSize={10}
        partialSyncEndpointNames={[]}
        searchText=""
        selectedIds={[]}
        sortDirection="desc"
        sortValue="修改时间"
        statusFilterEndpointNames={['本地NVMe', '影像NAS', '115']}
        statusFilter="全部"
        theme="light"
        total={1}
        onChangeSort={vi.fn()}
        onClearSelection={vi.fn()}
        onCreateFolder={vi.fn()}
        onDeleteAssetDirect={vi.fn()}
        onDeleteSelected={vi.fn()}
        onGoBack={vi.fn()}
        onGoForward={vi.fn()}
        onNavigateBreadcrumb={vi.fn()}
        onOpenItem={vi.fn()}
        onOpenBatchAnnotationEditor={vi.fn()}
        onOpenItemDetail={vi.fn()}
        onOpenTagEditor={vi.fn()}
        onRefreshIndex={vi.fn()}
        onUploadFiles={vi.fn()}
        onUploadFolder={vi.fn()}
        onRequestBatchDeleteEndpoint={vi.fn()}
        onRequestBatchSyncEndpoint={vi.fn()}
        onRequestDeleteEndpoint={vi.fn()}
        onRequestSyncEndpoint={vi.fn()}
        onSetCurrentPage={vi.fn()}
        onSetFileTypeFilter={vi.fn()}
        onSetPageSize={vi.fn()}
        onSetSearchText={vi.fn()}
        onSetStatusFilter={vi.fn()}
        onClearPartialSyncEndpoints={vi.fn()}
        onTogglePartialSyncEndpoint={vi.fn()}
        onToggleSortDirection={vi.fn()}
        onToggleSelect={vi.fn()}
        onToggleSelectVisible={vi.fn()}
      />,
    );

    expect(screen.getByRole('button', { name: '本地NVMe 已同步' })).toHaveTextContent('已同步');
    expect(screen.getByRole('button', { name: '影像NAS 同步中' })).toHaveTextContent('同步中');
    expect(screen.getByRole('button', { name: '115 未同步' })).toHaveTextContent('未同步');

    expect(screen.getByRole('button', { name: '本地NVMe 已同步' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '影像NAS 同步中' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '115 未同步' })).not.toBeDisabled();
  });

  it('更多操作会根据当前端点状态决定同步和删除是否可点', async () => {
    const user = userEvent.setup();

    render(
      <FileCenterPage
        breadcrumbs={[{ id: null, label: '商业摄影资产库' }]}
        batchDeleteEndpointActions={[]}
        batchSyncEndpointActions={[]}
        canGoBack={false}
        canGoForward={false}
        currentEntries={[sampleEntry]}
        currentPage={1}
        currentPathChildren={1}
        fileTypeFilter="全部"
        loading={false}
        pageCount={1}
        pageSize={10}
        partialSyncEndpointNames={[]}
        searchText=""
        selectedIds={[]}
        sortDirection="desc"
        sortValue="修改时间"
        statusFilterEndpointNames={['本地NVMe', '影像NAS', '115']}
        statusFilter="全部"
        theme="light"
        total={1}
        onChangeSort={vi.fn()}
        onClearSelection={vi.fn()}
        onCreateFolder={vi.fn()}
        onDeleteAssetDirect={vi.fn()}
        onDeleteSelected={vi.fn()}
        onGoBack={vi.fn()}
        onGoForward={vi.fn()}
        onNavigateBreadcrumb={vi.fn()}
        onOpenItem={vi.fn()}
        onOpenBatchAnnotationEditor={vi.fn()}
        onOpenItemDetail={vi.fn()}
        onOpenTagEditor={vi.fn()}
        onRefreshIndex={vi.fn()}
        onUploadFiles={vi.fn()}
        onUploadFolder={vi.fn()}
        onRequestBatchDeleteEndpoint={vi.fn()}
        onRequestBatchSyncEndpoint={vi.fn()}
        onRequestDeleteEndpoint={vi.fn()}
        onRequestSyncEndpoint={vi.fn()}
        onSetCurrentPage={vi.fn()}
        onSetFileTypeFilter={vi.fn()}
        onSetPageSize={vi.fn()}
        onSetSearchText={vi.fn()}
        onSetStatusFilter={vi.fn()}
        onClearPartialSyncEndpoints={vi.fn()}
        onTogglePartialSyncEndpoint={vi.fn()}
        onToggleSortDirection={vi.fn()}
        onToggleSelect={vi.fn()}
        onToggleSelectVisible={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: `更多操作 ${sampleEntry.name}` }));
    await user.hover(screen.getByRole('button', { name: '同步' }));

    let submenu = document.querySelector('.submenu-menu');
    expect(submenu).not.toBeNull();
    expect(within(submenu as HTMLElement).getByRole('button', { name: '本地NVMe' })).toHaveClass('is-disabled');
    expect(within(submenu as HTMLElement).getByRole('button', { name: '本地NVMe' })).toBeDisabled();
    expect(within(submenu as HTMLElement).getByRole('button', { name: '影像NAS' })).toHaveClass('is-disabled');
    expect(within(submenu as HTMLElement).getByRole('button', { name: '影像NAS' })).toBeDisabled();
    expect(within(submenu as HTMLElement).getByRole('button', { name: '115' })).not.toBeDisabled();

    await user.hover(screen.getByRole('button', { name: '删除' }));

    submenu = document.querySelector('.submenu-menu');
    expect(submenu).not.toBeNull();
    expect(within(submenu as HTMLElement).getByRole('button', { name: '本地NVMe' })).not.toBeDisabled();
    expect(within(submenu as HTMLElement).getByRole('button', { name: '影像NAS' })).toHaveClass('is-disabled');
    expect(within(submenu as HTMLElement).getByRole('button', { name: '影像NAS' })).toBeDisabled();
    expect(within(submenu as HTMLElement).getByRole('button', { name: '115' })).toHaveClass('is-disabled');
    expect(within(submenu as HTMLElement).getByRole('button', { name: '115' })).toBeDisabled();
  });

  it('显示星级排序选项和逆序按钮状态', () => {
    const onToggleSortDirection = vi.fn();

    render(
      <FileCenterPage
        breadcrumbs={[{ id: null, label: '商业摄影资产库' }]}
        batchDeleteEndpointActions={[]}
        batchSyncEndpointActions={[]}
        canGoBack={false}
        canGoForward={false}
        currentEntries={[sampleEntry]}
        currentPage={1}
        currentPathChildren={1}
        fileTypeFilter="全部"
        loading={false}
        pageCount={1}
        pageSize={10}
        partialSyncEndpointNames={[]}
        searchText=""
        selectedIds={[]}
        sortDirection="desc"
        sortValue="星级"
        statusFilterEndpointNames={['本地NVMe', '影像NAS', '115']}
        statusFilter="全部"
        theme="light"
        total={1}
        onChangeSort={vi.fn()}
        onClearSelection={vi.fn()}
        onCreateFolder={vi.fn()}
        onDeleteAssetDirect={vi.fn()}
        onDeleteSelected={vi.fn()}
        onGoBack={vi.fn()}
        onGoForward={vi.fn()}
        onNavigateBreadcrumb={vi.fn()}
        onOpenItem={vi.fn()}
        onOpenBatchAnnotationEditor={vi.fn()}
        onOpenItemDetail={vi.fn()}
        onOpenTagEditor={vi.fn()}
        onRefreshIndex={vi.fn()}
        onUploadFiles={vi.fn()}
        onUploadFolder={vi.fn()}
        onRequestBatchDeleteEndpoint={vi.fn()}
        onRequestBatchSyncEndpoint={vi.fn()}
        onRequestDeleteEndpoint={vi.fn()}
        onRequestSyncEndpoint={vi.fn()}
        onSetCurrentPage={vi.fn()}
        onSetFileTypeFilter={vi.fn()}
        onSetPageSize={vi.fn()}
        onSetSearchText={vi.fn()}
        onSetStatusFilter={vi.fn()}
        onClearPartialSyncEndpoints={vi.fn()}
        onTogglePartialSyncEndpoint={vi.fn()}
        onToggleSortDirection={onToggleSortDirection}
        onToggleSelect={vi.fn()}
        onToggleSelectVisible={vi.fn()}
      />,
    );

    expect(screen.getByRole('combobox', { name: '排序方式' })).toHaveValue('星级');
    const button = screen.getByRole('button', { name: '当前逆序，切换排序方向' });
    expect(button).toHaveAttribute('data-tooltip', '逆序');
    expect(button.querySelectorAll('.sort-direction-half.active')).toHaveLength(1);
    expect(button.querySelector('.sort-direction-half.inactive')).toBeTruthy();
  });

  it('状态筛选支持部分同步二级端点勾选', async () => {
    const user = userEvent.setup();
    const onSetStatusFilter = vi.fn();
    const onClearPartialSyncEndpoints = vi.fn();
    const onTogglePartialSyncEndpoint = vi.fn();

    render(
      <FileCenterPage
        breadcrumbs={[{ id: null, label: '商业摄影资产库' }]}
        batchDeleteEndpointActions={[]}
        batchSyncEndpointActions={[]}
        canGoBack={false}
        canGoForward={false}
        currentEntries={[sampleEntry]}
        currentPage={1}
        currentPathChildren={1}
        fileTypeFilter="全部"
        loading={false}
        pageCount={1}
        pageSize={10}
        partialSyncEndpointNames={['本地NVMe']}
        searchText=""
        selectedIds={[]}
        sortDirection="desc"
        sortValue="修改时间"
        statusFilterEndpointNames={['本地NVMe', '影像NAS', '115']}
        statusFilter="部分同步"
        theme="light"
        total={1}
        onChangeSort={vi.fn()}
        onClearSelection={vi.fn()}
        onCreateFolder={vi.fn()}
        onDeleteAssetDirect={vi.fn()}
        onDeleteSelected={vi.fn()}
        onGoBack={vi.fn()}
        onGoForward={vi.fn()}
        onNavigateBreadcrumb={vi.fn()}
        onOpenItem={vi.fn()}
        onOpenBatchAnnotationEditor={vi.fn()}
        onOpenItemDetail={vi.fn()}
        onOpenTagEditor={vi.fn()}
        onRefreshIndex={vi.fn()}
        onUploadFiles={vi.fn()}
        onUploadFolder={vi.fn()}
        onRequestBatchDeleteEndpoint={vi.fn()}
        onRequestBatchSyncEndpoint={vi.fn()}
        onRequestDeleteEndpoint={vi.fn()}
        onRequestSyncEndpoint={vi.fn()}
        onSetCurrentPage={vi.fn()}
        onSetFileTypeFilter={vi.fn()}
        onSetPageSize={vi.fn()}
        onSetSearchText={vi.fn()}
        onSetStatusFilter={onSetStatusFilter}
        onClearPartialSyncEndpoints={onClearPartialSyncEndpoints}
        onTogglePartialSyncEndpoint={onTogglePartialSyncEndpoint}
        onToggleSortDirection={vi.fn()}
        onToggleSelect={vi.fn()}
        onToggleSelectVisible={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: '状态筛选' }));
    expect(screen.getByRole('button', { name: '全部' })).toBeInTheDocument();
    await user.hover(screen.getByRole('button', { name: '部分同步' }));
    expect(screen.getByLabelText('本地NVMe')).toBeChecked();
    await user.click(screen.getByLabelText('影像NAS'));
    expect(onSetStatusFilter).toHaveBeenCalledWith('部分同步');
    expect(onTogglePartialSyncEndpoint).toHaveBeenCalledWith('影像NAS');

    await user.click(screen.getByRole('button', { name: '全部' }));
    expect(onClearPartialSyncEndpoints).toHaveBeenCalled();
    expect(onSetStatusFilter).toHaveBeenCalledWith('全部');
  });

  it('靠近底部时更多操作浮窗会自动向上展开', async () => {
    const user = userEvent.setup();
    const originalInnerHeight = window.innerHeight;
    Object.defineProperty(window, 'innerHeight', {
      configurable: true,
      value: 360,
    });

    render(
      <FileCenterPage
        breadcrumbs={[{ id: null, label: '商业摄影资产库' }]}
        batchDeleteEndpointActions={[]}
        batchSyncEndpointActions={[]}
        canGoBack={false}
        canGoForward={false}
        currentEntries={[sampleEntry]}
        currentPage={1}
        currentPathChildren={1}
        fileTypeFilter="全部"
        loading={false}
        pageCount={1}
        pageSize={10}
        partialSyncEndpointNames={[]}
        searchText=""
        selectedIds={[]}
        sortDirection="desc"
        sortValue="修改时间"
        statusFilterEndpointNames={['本地NVMe', '影像NAS', '115']}
        statusFilter="全部"
        theme="light"
        total={1}
        onChangeSort={vi.fn()}
        onClearSelection={vi.fn()}
        onCreateFolder={vi.fn()}
        onDeleteAssetDirect={vi.fn()}
        onDeleteSelected={vi.fn()}
        onGoBack={vi.fn()}
        onGoForward={vi.fn()}
        onNavigateBreadcrumb={vi.fn()}
        onOpenItem={vi.fn()}
        onOpenBatchAnnotationEditor={vi.fn()}
        onOpenItemDetail={vi.fn()}
        onOpenTagEditor={vi.fn()}
        onRefreshIndex={vi.fn()}
        onUploadFiles={vi.fn()}
        onUploadFolder={vi.fn()}
        onRequestBatchDeleteEndpoint={vi.fn()}
        onRequestBatchSyncEndpoint={vi.fn()}
        onRequestDeleteEndpoint={vi.fn()}
        onRequestSyncEndpoint={vi.fn()}
        onSetCurrentPage={vi.fn()}
        onSetFileTypeFilter={vi.fn()}
        onSetPageSize={vi.fn()}
        onSetSearchText={vi.fn()}
        onSetStatusFilter={vi.fn()}
        onClearPartialSyncEndpoints={vi.fn()}
        onTogglePartialSyncEndpoint={vi.fn()}
        onToggleSortDirection={vi.fn()}
        onToggleSelect={vi.fn()}
        onToggleSelectVisible={vi.fn()}
      />,
    );

    const trigger = screen.getByRole('button', { name: `更多操作 ${sampleEntry.name}` });
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
      x: 900,
      y: 300,
      width: 38,
      height: 38,
      top: 300,
      right: 938,
      bottom: 338,
      left: 900,
      toJSON: () => ({}),
    } as DOMRect);

    await user.click(trigger);

    const menu = document.querySelector('.file-center-floating-menu') as HTMLElement | null;
    expect(menu).not.toBeNull();
    expect(menu?.style.top).toBe('72px');

    Object.defineProperty(window, 'innerHeight', {
      configurable: true,
      value: originalInnerHeight,
    });
  });
});
