import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import type { Library } from './data';
import { TaskCenterPage } from './pages/TaskCenterPage';
import { createInitialState } from './lib/clientState';
import { fileCenterApi, resetFileCenterMock } from './lib/fileCenterApi';

const TEST_LIBRARIES: Library[] = [
  { id: 'photo', name: '商业摄影资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地 + NAS' },
  { id: 'video', name: '视频工作流资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地 + NAS' },
  { id: 'family', name: '家庭照片资产库', rootLabel: '/', itemCount: '0', health: '100%', storagePolicy: '本地 + NAS' },
];

describe('MARE 客户端', () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await resetFileCenterMock();
  });

  afterEach(async () => {
    vi.restoreAllMocks();
    cleanup();
    await resetFileCenterMock();
  });

  it.skip('支持进入目录并查看文件详情', async () => {
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

    await user.click(screen.getByRole('button', { name: '已插入 5 个设备' }));
    const deviceRow = (await screen.findByText('CFexpress A 卡（A 机位）')).closest('article');
    expect(deviceRow).not.toBeNull();
    await user.click(deviceRow!);
    await user.click(screen.getByRole('button', { name: '提交导入' }));
    await user.click(await screen.findByRole('button', { name: '确认提交' }));

    expect(await screen.findByText('已提交导入作业，任务已加入队列')).toBeInTheDocument();
    expect(await screen.findByText('结果摘要')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '查看导入报告' })).toBeInTheDocument();
  });

  it('任务状态筛选默认使用活跃中', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    expect(screen.getByLabelText('任务状态')).toHaveValue('活跃中');
    expect(screen.queryByText('片头配乐_v4_master.wav')).not.toBeInTheDocument();
    expect(screen.queryByText('客户访谈_第一机位_精编版.mov')).not.toBeInTheDocument();
  });

  it('传输任务默认只显示一级任务，可在列表展开二级任务并在详情中查看三级任务', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    expect(screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/)).toBeInTheDocument();
    expect(screen.queryByText('2026-03-29_上海发布会_A-cam_001.RAW')).not.toBeInTheDocument();
    expect(screen.queryByText('2026-03-29_上海发布会_A-cam_001.RAW')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /展开 2026-03-29_上海发布会_A-cam_001\.RAW、2026-03-29_上海发布会_B-cam_018\.RAW/ }));
    expect(await screen.findAllByText('2026-03-29_上海发布会_A-cam_001.RAW')).toHaveLength(1);
    expect(screen.getByText('2026-03-29_上海发布会_B-cam_018.RAW')).toBeInTheDocument();

    const taskRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/).closest('article') as HTMLElement | null;
    expect(taskRow).not.toBeNull();
    await user.click(within(taskRow!).getAllByRole('button', { name: '详情' })[0]);

    const detailSheet = await screen.findByRole('region', { name: /2026-03-29_上海发布会_A-cam_001\.RAW、2026-03-29_上海发布会_B-cam_018\.RAW/ });
    expect(within(detailSheet).getByText('本地NVMe')).toBeInTheDocument();
    expect(within(detailSheet).getByText('影像NAS')).toBeInTheDocument();
    expect(within(detailSheet).getByText('124')).toBeInTheDocument();
  });

  it('一级任务显示总体大小，二级任务使用文件样式并支持暂停、继续、取消和详情查看', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    expect(screen.getByText('124 个文件 · 0 个文件夹')).toBeInTheDocument();
    expect(await screen.findByText('124 项')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /展开 2026-03-29_上海发布会_A-cam_001\.RAW、2026-03-29_上海发布会_B-cam_018\.RAW/ }));
    const childRow = screen.getByText('2026-03-29_上海发布会_B-cam_018.RAW').closest('.transfer-child-task-row') as HTMLElement | null;
    expect(childRow).not.toBeNull();
    expect(within(childRow!).getByText('商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片')).toBeInTheDocument();

    await user.click(within(childRow!).getByRole('button', { name: '暂停' }));
    expect((await within(childRow!).findAllByText('已暂停')).length).toBeGreaterThan(0);

    await user.click(within(childRow!).getByRole('button', { name: '继续' }));
    expect((await within(childRow!).findAllByText('传输中')).length).toBeGreaterThan(0);

    await user.click(within(childRow!).getByRole('button', { name: '取消' }));
    expect((await within(childRow!).findAllByText('已取消')).length).toBeGreaterThan(0);

    await user.click(within(childRow!).getByRole('button', { name: '详情' }));
    const itemSheet = await screen.findByRole('region', { name: '2026-03-29_上海发布会_B-cam_018.RAW' });
    expect(within(itemSheet).getByText('47.8 MB')).toBeInTheDocument();
  });

  it('一级任务详情中的查看文件中心会跳转并自动勾选对应文件或文件夹', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    const taskRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/).closest('article') as HTMLElement | null;
    expect(taskRow).not.toBeNull();
    await user.click(within(taskRow!).getAllByRole('button', { name: '详情' })[0]);

    const detailSheet = await screen.findByRole('region', { name: /2026-03-29_上海发布会_A-cam_001\.RAW、2026-03-29_上海发布会_B-cam_018\.RAW/ });
    await user.click(within(detailSheet).getByRole('button', { name: '查看文件中心' }));

    expect(await screen.findByRole('button', { name: '文件中心' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByLabelText('选择 2026-03-29_上海发布会_A-cam_001.RAW')).toBeChecked());
    await waitFor(() => expect(screen.getByLabelText('选择 2026-03-29_上海发布会_B-cam_018.RAW')).toBeChecked());
  });

  it('主任务详情中暂停和继续会即时切换按钮', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    const taskRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/).closest('article') as HTMLElement | null;
    expect(taskRow).not.toBeNull();
    await user.click(within(taskRow!).getAllByRole('button', { name: '详情' })[0]);

    const detailSheet = await screen.findByRole('region', { name: /2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/ });
    await user.click(within(detailSheet).getByRole('button', { name: '暂停' }));
    expect(await within(detailSheet).findByRole('button', { name: '继续' })).toBeInTheDocument();

    await user.click(within(detailSheet).getByRole('button', { name: '继续' }));
    expect(await within(detailSheet).findByRole('button', { name: '暂停' })).toBeInTheDocument();
  });

  it('一级任务仅在存在唯一对应路径时才在名称下方显示路径', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    const uniquePathRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/).closest('article') as HTMLElement | null;
    expect(uniquePathRow).not.toBeNull();
    expect(within(uniquePathRow!).getByText('商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片')).toBeInTheDocument();

    const singleTaskRow = screen.getByText('客户返签确认单_2026Q2.pdf').closest('article') as HTMLElement | null;
    expect(singleTaskRow).not.toBeNull();
    const inlinePath = within(singleTaskRow!).getByText('C:\\Mare\\Exports\\客户返签确认单_2026Q2.pdf');
    expect(inlinePath).toHaveAttribute('title', 'C:\\Mare\\Exports\\客户返签确认单_2026Q2.pdf');
  });

  it('含二级任务的一级任务显示二级任务数量，其余任务显示大小', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    const multiChildRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/).closest('article') as HTMLElement | null;
    expect(multiChildRow).not.toBeNull();
    expect(within(multiChildRow!).getByText('124 项')).toBeInTheDocument();

    const singleTaskRow = screen.getByText('客户返签确认单_2026Q2.pdf').closest('article') as HTMLElement | null;
    expect(singleTaskRow).not.toBeNull();
    expect(within(singleTaskRow!).getByText('6.2 MB')).toBeInTheDocument();
  });

  it('新增的跨目录任务展开后会显示多个不同路径的二级任务', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    const mixedTaskRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*上海发布会_精选组图\.zip/).closest('article') as HTMLElement | null;
    expect(mixedTaskRow).not.toBeNull();
    await user.click(within(mixedTaskRow!).getByRole('button', { name: /展开 2026-03-29_上海发布会_A-cam_001\.RAW.*上海发布会_精选组图\.zip/ }));

    const mixedChildRows = within(mixedTaskRow!).getAllByText(/2026-03-29_上海发布会_A-cam_001\.RAW|上海发布会_精选组图\.zip/);
    expect(mixedChildRows.length).toBeGreaterThanOrEqual(2);
    expect(within(mixedTaskRow!).getAllByText('商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片').length).toBeGreaterThanOrEqual(1);
    expect(within(mixedTaskRow!).getByTitle('商业摄影资产库 / 2026 / Shanghai Launch / 精选交付 / 发布会资料包')).toBeInTheDocument();
  });

  it('文件夹类型一级任务展开后只显示递归提取出的文件，不显示文件夹节点', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    const folderTaskRow = screen.getByText('精选交付').closest('article') as HTMLElement | null;
    expect(folderTaskRow).not.toBeNull();
    await user.click(within(folderTaskRow!).getByRole('button', { name: /展开 精选交付/ }));

    expect(await within(folderTaskRow!).findByText('上海发布会_精选封面.jpg')).toBeInTheDocument();
    expect(within(folderTaskRow!).getByText('上海发布会_精选组图.zip')).toBeInTheDocument();
    expect(within(folderTaskRow!).queryByText('发布会资料包')).not.toBeInTheDocument();
  });

  it('二级任务关联文件被删除后显示为整行不可点击的失效状态', async () => {
    const seed = createInitialState();
    const brokenItems = seed.taskItemRecords.map((item) =>
      item.id === 'task-item-2' ? { ...item, fileNodeId: 'missing-node' } : item,
    );

    render(
      <TaskCenterPage
        activeTab="transfer"
        fileNodes={seed.fileNodes}
        issues={seed.issueRecords}
        libraries={TEST_LIBRARIES}
        preselectedTaskIds={null}
        statusFilter="全部"
        taskItems={brokenItems}
        tasks={seed.taskRecords}
        onChangeTaskPriority={() => {}}
        onChangeTaskItemStatus={() => {}}
        onChangeTaskStatus={() => {}}
        onConsumePreselectedTaskIds={() => {}}
        onOpenIssueCenterForIssue={() => {}}
        onOpenIssueCenterForTask={() => {}}
        onOpenTaskDetail={() => {}}
        onSetActiveTab={() => {}}
        onSetTaskStatusFilter={() => {}}
      />,
    );

    const expandButton = screen.getByRole('button', { name: /展开 2026-03-29_上海发布会_A-cam_001\.RAW、文件已删除/ });
    await userEvent.setup().click(expandButton);

    const missingRow = screen.getByText('文件已删除').closest('.transfer-child-task-row') as HTMLElement | null;
    expect(missingRow).not.toBeNull();
    expect(missingRow).toHaveClass('missing');
    expect(within(missingRow!).getByRole('button', { name: '详情' })).toBeDisabled();
    expect(within(missingRow!).getByRole('button', { name: '取消' })).toBeDisabled();
    expect(within(missingRow!).getByRole('checkbox')).toBeDisabled();
  });

  it('传输任务支持异常浮窗，并可按任务跳转到异常中心', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));
    await user.click(await screen.findByRole('button', { name: '查看异常 精选交付' }));

    expect(await screen.findByText('令牌将在 12 小时内过期')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '处置所有异常' }));

    expect(await screen.findByText(/按任务查看异常：精选交付/)).toBeInTheDocument();
    expect(screen.getByText('115 云归档鉴权即将过期')).toBeInTheDocument();
    expect(screen.queryByText('片头配乐_v4_master.wav')).not.toBeInTheDocument();
  });

  it('传输任务支持批量暂停和批量调整优先级', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    await user.click(screen.getByLabelText(/选择 2026-03-29_上海发布会_A-cam_001\.RAW、2026-03-29_上海发布会_B-cam_018\.RAW/));
    await user.click(screen.getAllByLabelText('选择 精选交付')[0]);

    expect(screen.getByText('已选择 4 个任务项')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '批量暂停' }));
    expect((await screen.findAllByText('已暂停')).length).toBeGreaterThanOrEqual(2);

    await user.click(screen.getByRole('button', { name: '批量设为高优先级' }));
    const priorityPills = screen.getAllByText('高优先级');
    expect(priorityPills.length).toBeGreaterThanOrEqual(2);
  });

  it('已完成任务显示完成按钮且保持三个操作按钮', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    const completedRow = screen.getByText('客户访谈_第一机位_精编版.mov').closest('article') as HTMLElement | null;
    expect(completedRow).not.toBeNull();
    expect(within(completedRow!).getByRole('button', { name: '详情' })).toBeInTheDocument();
    expect(within(completedRow!).getByRole('button', { name: '完成' })).toBeInTheDocument();
    expect(within(completedRow!).getByRole('button', { name: '取消' })).toBeInTheDocument();
    expect(within(completedRow!).queryByRole('button', { name: '暂停' })).not.toBeInTheDocument();
    expect(within(completedRow!).queryByRole('button', { name: '继续' })).not.toBeInTheDocument();
    expect(within(completedRow!).queryByRole('button', { name: '重试' })).not.toBeInTheDocument();
  });

  it('已完成的二级任务显示完成按钮', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));
    await user.click(screen.getByRole('button', { name: /展开 2026-03-29_上海发布会_A-cam_001\.RAW、2026-03-29_上海发布会_B-cam_018\.RAW/ }));

    const completedChildRow = screen.getByText('2026-03-29_上海发布会_A-cam_001.RAW').closest('.transfer-child-task-row') as HTMLElement | null;
    expect(completedChildRow).not.toBeNull();
    expect(within(completedChildRow!).getByRole('button', { name: '详情' })).toBeInTheDocument();
    expect(within(completedChildRow!).getByRole('button', { name: '完成' })).toBeInTheDocument();
    expect(within(completedChildRow!).getByRole('button', { name: '取消' })).toBeInTheDocument();
  });

  it('失败任务只显示重试，不显示暂停或继续', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');

    const failedRow = screen.getByText('片头配乐_v4_master.wav').closest('article') as HTMLElement | null;
    expect(failedRow).not.toBeNull();
    expect(within(failedRow!).getByRole('button', { name: '重试' })).toBeInTheDocument();
    expect(within(failedRow!).queryByRole('button', { name: '暂停' })).not.toBeInTheDocument();
    expect(within(failedRow!).queryByRole('button', { name: '继续' })).not.toBeInTheDocument();
  });

  it('支持在异常中心执行轻处理动作并更新异常状态', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '异常中心' }));
    const issueRow = screen.getByText('115 云归档鉴权即将过期').closest('article') as HTMLElement | null;
    expect(issueRow).not.toBeNull();
    await user.click(within(issueRow!).getByRole('button', { name: '标记确认' }));

    expect(await screen.findByText('已标记 当前异常 为已确认')).toBeInTheDocument();
    expect(within(issueRow!).getByText('已解决')).toBeInTheDocument();
  });

  it('支持从异常中心跳转到具体任务并自动展开对应异常', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '异常中心' }));
    const issueRow = screen.getByText('片头配乐_v4_master.wav 上传后校验失败').closest('article') as HTMLElement | null;
    expect(issueRow).not.toBeNull();

    await user.click(within(issueRow!).getByRole('button', { name: '更多' }));
    await user.click(screen.getByRole('button', { name: '打开任务中心' }));

    expect(await screen.findByRole('button', { name: '任务中心' })).toBeInTheDocument();
    expect((await screen.findAllByText('片头配乐_v4_master.wav')).length).toBeGreaterThanOrEqual(1);
    expect(await screen.findByText('当前任务异常')).toBeInTheDocument();
    expect(screen.getByText('远端校验摘要与本地源文件不一致。')).toBeInTheDocument();
  });

  it.skip('支持删除资产并进入等待清理状态', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.dblClick(await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW'));
    await user.click(screen.getByRole('button', { name: '更多操作 2026-03-29_上海发布会_A-cam_001.RAW' }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '删除资产' }));
    expect(await screen.findByRole('dialog', { name: '存在运行中任务' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '取消任务并删除' }));

    expect(await screen.findByText('删除请求已提交，资产进入等待清理')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '其它任务' }));
    await user.selectOptions(screen.getByLabelText('任务状态'), '全部');
    expect(screen.getAllByText('删除资产：2026-03-29_上海发布会_A-cam_001.RAW').length).toBeGreaterThan(0);
  });

  it.skip('支持多选后批量设置星级和色标', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.click(screen.getByLabelText('选择 2026-03-29_上海发布会_A-cam_001.RAW'));
    await user.click(screen.getByLabelText('选择 2026-03-29_上海发布会_B-cam_018.RAW'));

    await user.click(screen.getByRole('button', { name: '批量标记' }));
    expect(await screen.findByRole('dialog', { name: '批量标记' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '5 星' }));
    await user.click(screen.getByRole('button', { name: '蓝标' }));
    await user.click(screen.getByRole('button', { name: '保存标记' }));

    expect(await screen.findByText('已更新 2 项资产的标记')).toBeInTheDocument();

    const row = (await screen.findByText('2026-03-29_上海发布会_B-cam_018.RAW')).closest('tr');
    expect(row).not.toBeNull();
    expect(within(row!).getByLabelText('5 星')).toBeInTheDocument();
    expect(within(row!).getByLabelText('蓝标')).toBeInTheDocument();
  });

  it.skip('支持通过上传菜单选择文件并开始上传', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.click(screen.getByRole('button', { name: '上传' }));
    expect(await screen.findByRole('button', { name: '上传文件' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '上传文件夹' })).toBeInTheDocument();

    const input = screen.getByLabelText('上传文件选择');
    const file = new File(['image-binary'], '客户logo.png', { type: 'image/png' });
    await user.upload(input, file);

    expect(await screen.findByText('已开始上传 1 个文件')).toBeInTheDocument();
    const row = (await screen.findByText('客户logo.png')).closest('tr');
    expect(row).not.toBeNull();
    expect(within(row!).getByText(/^今天 \d{2}:\d{2}$/)).toBeInTheDocument();
    expect(within(row!).getByRole('button', { name: '本地NVMe 已同步' })).toBeInTheDocument();
    expect(within(row!).getByRole('button', { name: '影像NAS 未同步' })).toBeInTheDocument();
    expect(within(row!).getByRole('button', { name: '115 未同步' })).toBeInTheDocument();
  });

  it.skip('支持多选后批量同步到指定端点', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.click(screen.getByLabelText('选择 2026-03-29_上海发布会_A-cam_001.RAW'));
    await user.click(screen.getByLabelText('选择 2026-03-29_上海发布会_B-cam_018.RAW'));

    await user.click(screen.getByRole('button', { name: '同步' }));
    await user.click(screen.getByRole('button', { name: '115' }));
    expect(await screen.findByRole('dialog', { name: '确认同步' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认同步' }));

    expect(await screen.findByText('已为 2 项资产创建同步任务到 115')).toBeInTheDocument();
  });

  it.skip('支持点击文件夹存储状态递归同步文件夹内容', async () => {
    const user = userEvent.setup();
    render(<App />);

    const folderRow = (await screen.findByText('拍摄原片')).closest('tr');
    expect(folderRow).not.toBeNull();
    await user.click(within(folderRow!).getByRole('button', { name: '115 部分同步' }));
    expect(await screen.findByRole('dialog', { name: '确认同步' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认同步' }));

    await user.dblClick(screen.getByText('拍摄原片'));
    const childRow = (await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).closest('tr');
    expect(childRow).not.toBeNull();
    expect(within(childRow!).getByRole('button', { name: '115 同步中' })).toBeInTheDocument();
  });

  it.skip('支持多选后按端批量删除副本', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.click(screen.getByLabelText('选择 2026-03-29_上海发布会_A-cam_047.RAW'));
    await user.click(screen.getByLabelText('选择 2026-03-29_上海发布会_A-cam_046.RAW'));

    await user.click(screen.getByRole('button', { name: '删除副本' }));
    await user.click(screen.getByRole('button', { name: '115' }));
    expect(await screen.findByRole('dialog', { name: '确认删除副本' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认删除' }));

    expect(await screen.findByText('已提交 2 项资产的端点删除请求')).toBeInTheDocument();

    const row047 = (await screen.findByText('2026-03-29_上海发布会_A-cam_047.RAW')).closest('tr');
    const row046 = (await screen.findByText('2026-03-29_上海发布会_A-cam_046.RAW')).closest('tr');
    expect(row047).not.toBeNull();
    expect(row046).not.toBeNull();
    expect(within(row047!).getByRole('button', { name: '115 未同步' })).toBeInTheDocument();
    expect(within(row046!).getByRole('button', { name: '115 未同步' })).toBeInTheDocument();
  });

  it.skip('支持多选文件夹后批量同步文件夹内容', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByLabelText('选择 拍摄原片'));
    await user.click(screen.getByRole('button', { name: '同步' }));
    await user.click(screen.getByRole('button', { name: '115' }));
    expect(await screen.findByRole('dialog', { name: '确认同步' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认同步' }));

    await user.dblClick(screen.getByText('拍摄原片'));
    const childRow = (await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).closest('tr');
    expect(childRow).not.toBeNull();
    expect(within(childRow!).getByRole('button', { name: '115 同步中' })).toBeInTheDocument();
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

  it('设置页展示新的策略页签并提供与现有工作台一致的预览信息', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));

    expect(await screen.findByRole('button', { name: '工作区' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '导入与归档' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '通知与提醒' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '异常治理' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '后台任务与性能' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '通知与提醒' }));
    expect(await screen.findByText('通知示例')).toBeInTheDocument();
    expect(screen.getByText('影像 NAS 01 共享目录写入权限异常')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '异常治理' }));
    expect(await screen.findByText('治理快照')).toBeInTheDocument();
    expect(screen.getAllByText('待处理').length).toBeGreaterThan(0);
  });

  it('支持保存工作区默认打开页，并在重新打开客户端后直接进入对应页面', async () => {
    const user = userEvent.setup();
    const { unmount } = render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(await screen.findByRole('button', { name: '工作区' }));
    await user.selectOptions(screen.getByLabelText('默认打开页面'), '任务中心');
    await user.click(screen.getByRole('button', { name: '保存设置' }));

    expect(await screen.findByText('设置已保存')).toBeInTheDocument();

    unmount();
    render(<App />);

    expect(await screen.findByLabelText('任务状态')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '传输任务' })).toBeInTheDocument();
  });

  it('重新加载后仍保留上次选择的资产库', async () => {
    const user = userEvent.setup();
    const { unmount } = render(<App />);

    await waitFor(() =>
      expect(document.querySelector('.library-trigger strong')?.textContent).toBe('商业摄影资产库'),
    );

    const trigger = document.querySelector('.library-trigger') as HTMLButtonElement | null;
    expect(trigger).not.toBeNull();
    await user.click(trigger!);
    await user.click(screen.getAllByRole('button', { name: /视频工作流资产库/ })[0]!);

    await waitFor(() =>
      expect(document.querySelector('.library-trigger strong')?.textContent).toBe('视频工作流资产库'),
    );

    unmount();
    render(<App />);

    await waitFor(() =>
      expect(document.querySelector('.library-trigger strong')?.textContent).toBe('视频工作流资产库'),
    );
  });

  it.skip('支持从指定端点删除且保留资产', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    const row = (await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row!).getByRole('button', { name: '更多操作 2026-03-29_上海发布会_A-cam_001.RAW' }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '影像NAS' }));
    expect(await screen.findByRole('dialog', { name: '存在运行中任务' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '取消任务并删除' }));
    expect(await screen.findByText('已提交端点删除请求')).toBeInTheDocument();
  });

  it.skip('支持通过文件夹更多操作递归删除文件夹内容的端点副本', async () => {
    const user = userEvent.setup();
    render(<App />);

    const folderRow = (await screen.findByText('精选交付')).closest('tr');
    expect(folderRow).not.toBeNull();
    await user.click(within(folderRow!).getByRole('button', { name: '更多操作 精选交付' }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '115' }));
    expect(await screen.findByRole('dialog', { name: '存在运行中任务' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '取消任务并删除' }));

    await user.dblClick(screen.getByText('精选交付'));
    const childRow = (await screen.findByText('上海发布会_精选封面.jpg')).closest('tr');
    expect(childRow).not.toBeNull();
    expect(within(childRow!).getByRole('button', { name: '115 未同步' })).toBeInTheDocument();
  });

  it.skip('当各端都没有副本后会自动从文件中心移除资产', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.dblClick(await screen.findByText('拍摄原片'));
    const fileName = '2026-03-29_上海发布会_A-cam_001.RAW';
    let row = (await screen.findByText(fileName)).closest('tr');
    expect(row).not.toBeNull();

    await user.click(within(row!).getByRole('button', { name: `更多操作 ${fileName}` }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '本地NVMe' }));
    expect(await screen.findByRole('dialog', { name: '存在运行中任务' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '取消任务并删除' }));
    expect(await screen.findByText('已提交端点删除请求')).toBeInTheDocument();

    row = (await screen.findByText(fileName)).closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row!).getByRole('button', { name: `更多操作 ${fileName}` }));
    await user.hover(screen.getByRole('button', { name: '删除' }));
    await user.click(screen.getByRole('button', { name: '影像NAS' }));
    await user.click(await screen.findByRole('button', { name: '确认删除' }));

    expect(await screen.findByText('资产已因无剩余副本自动删除')).toBeInTheDocument();
    expect(screen.queryByText(fileName)).toBeNull();
  });

  it.skip('支持点击端点状态发起同步确认', async () => {
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

  it.skip(
    '文件同步后会在 5 秒后自动刷新为已同步',
    async () => {
      const user = userEvent.setup();
      render(<App />);

      await user.dblClick(await screen.findByText('拍摄原片'));
      const row = (await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).closest('tr');
      expect(row).not.toBeNull();

      await user.click(within(row!).getByRole('button', { name: '115 未同步' }));
      expect(await screen.findByRole('dialog', { name: '确认同步' })).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: '确认同步' }));

      expect(await screen.findByText('已创建同步任务到 115')).toBeInTheDocument();
      expect(await within(row!).findByRole('button', { name: '115 已同步' }, { timeout: 7000 })).toBeInTheDocument();
    },
    12000,
  );

  it('支持进入存储节点页并执行连接测试', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '存储节点' }));

    expect(await screen.findByRole('button', { name: '本地文件夹管理' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'NAS 管理' }));

    const nameCell = await screen.findByText('影像 NAS 01');
    const row = nameCell.closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row!).getByRole('button', { name: '连接测试 影像 NAS 01' }));

    expect(await screen.findByRole('dialog', { name: '连接测试结果' })).toBeInTheDocument();
  });

  it('页头展示动态导入入口并可跳转到导入中心', async () => {
    const user = userEvent.setup();
    render(<App />);

    const signalButton = screen.getByRole('button', { name: '已插入 5 个设备' });
    expect(signalButton).toBeInTheDocument();

    await user.click(signalButton);

    expect(await screen.findByRole('tab', { name: '导入中心' })).toBeInTheDocument();
    expect(await screen.findByText('来源路径')).toBeInTheDocument();
  });

  it('打开通知中心会自动消费提醒类通知但保留处置类角标', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByLabelText('未消费通知 8 条')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '通知' }));

    const reminderRow = (await screen.findByText('115 云归档鉴权即将过期')).closest('article');
    const actionRow = screen.getByText('影像 NAS 01 共享目录写入权限异常').closest('article');

    expect(reminderRow).not.toBeNull();
    expect(actionRow).not.toBeNull();

    await waitFor(() => expect(screen.getByLabelText('未消费通知 5 条')).toBeInTheDocument());

    expect(within(reminderRow!).getByText('提醒类')).toBeInTheDocument();
    expect(within(reminderRow!).getByText('已读')).toBeInTheDocument();
    expect(within(actionRow!).getByText('处置类')).toBeInTheDocument();
    expect(within(actionRow!).getByText('未消费')).toBeInTheDocument();
  });

  it('处置类通知支持手动已读和点击跳转后消角标', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '通知' }));
    await waitFor(() => expect(screen.getByLabelText('未消费通知 5 条')).toBeInTheDocument());

    const conflictRow = (await screen.findByText('客户访谈_第一机位_精编版.mov 路径冲突')).closest('article');
    expect(conflictRow).not.toBeNull();

    await user.click(within(conflictRow!).getByRole('button', { name: '更多操作 客户访谈_第一机位_精编版.mov 路径冲突' }));
    await user.click(screen.getByRole('button', { name: '标记已读' }));
    await waitFor(() => expect(screen.getByLabelText('未消费通知 4 条')).toBeInTheDocument());
    expect(within(conflictRow!).getByText('已读')).toBeInTheDocument();

    const storageRow = screen.getByText('影像 NAS 01 共享目录写入权限异常').closest('article');
    expect(storageRow).not.toBeNull();

    await user.click(within(storageRow!).getByRole('button', { name: '去处理 影像 NAS 01 共享目录写入权限异常' }));

    expect(await screen.findByRole('button', { name: '异常中心' })).toBeInTheDocument();
    expect(await screen.findByText('影像 NAS 01 共享目录写入权限异常')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByLabelText('未消费通知 3 条')).toBeInTheDocument());
  });

  it.skip('支持在设置中进入标签管理并让新标签出现在文件中心标签选择器中', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(screen.getByRole('button', { name: '标签管理' }));
    expect(await screen.findByText('标签总数')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '新增标签' }));
    const createDialog = await screen.findByRole('dialog', { name: '新增标签' });
    await user.type(within(createDialog).getByLabelText('标签名称'), '直播切片');
    await user.selectOptions(within(createDialog).getByLabelText('所属分组'), 'tag-group-project');
    await user.click(within(createDialog).getByLabelText('商业摄影资产库'));
    await user.click(within(createDialog).getByRole('button', { name: '创建标签' }));
    expect(await screen.findByText('标签已创建')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '文件中心' }));
    await user.dblClick(await screen.findByText('拍摄原片'));
    await user.click(screen.getByRole('button', { name: '更多操作 2026-03-29_上海发布会_A-cam_001.RAW' }));
    await user.click(screen.getByRole('button', { name: '标签' }));

    const editor = await screen.findByRole('dialog', { name: '标签编辑' });
    expect(within(editor).getByRole('button', { name: '直播切片 0 次使用' })).toBeInTheDocument();
  });
  it('recalculates transfer size after cancel and shows primary path meta for single task', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '任务中心' }));
    await user.click(screen.getByRole('button', { name: '同步' }));

    expect(screen.queryByText('大小计算中')).not.toBeInTheDocument();

    const taskRow = screen.getByText(/2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/).closest('article') as HTMLElement | null;
    expect(taskRow).not.toBeNull();
    expect(within(taskRow!).getByText('本地NVMe → 影像NAS')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /展开 2026-03-29_上海发布会_A-cam_001\.RAW.*2026-03-29_上海发布会_B-cam_018\.RAW/ }));
    const childRow = screen.getByText('2026-03-29_上海发布会_B-cam_018.RAW').closest('.transfer-child-task-row') as HTMLElement | null;
    expect(childRow).not.toBeNull();

    const childPath = within(childRow!).getByText('商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片');
    expect(childPath).toHaveAttribute('title', '商业摄影资产库 / 2026 / Shanghai Launch / 拍摄原片');

    await user.click(within(childRow!).getByRole('button', { name: '取消' }));
    expect(within(taskRow!).queryByText('大小计算中')).not.toBeInTheDocument();
    expect(await within(taskRow!).findByText('124 项')).toBeInTheDocument();

    const singleTaskRow = screen.getByText('客户返签确认单_2026Q2.pdf').closest('article') as HTMLElement | null;
    expect(singleTaskRow).not.toBeNull();
    const routeMeta = within(singleTaskRow!).getByText('本地NVMe → 115 云归档');
    expect(routeMeta).toHaveAttribute('title', '本地NVMe → 115 云归档');
    expect(routeMeta).toHaveClass('transfer-progress-meta');
  });

  it('进入目录时会后台扫描当前层且不阻塞已有目录内容', async () => {
    const user = userEvent.setup();
    const pendingScan = new Promise<{ message: string }>(() => undefined);
    const scanSpy = vi
      .spyOn(fileCenterApi, 'scanDirectory')
      .mockResolvedValueOnce({ message: '当前目录扫描已完成' })
      .mockImplementationOnce(() => pendingScan);

    render(<App />);

    await waitFor(() => {
      expect(scanSpy).toHaveBeenCalledWith({
        libraryId: 'photo',
        parentId: null,
      });
    });

    await user.dblClick(await screen.findByText('拍摄原片'));

    expect(await screen.findByText('2026-03-29_上海发布会_A-cam_001.RAW')).toBeInTheDocument();
    expect(scanSpy).toHaveBeenLastCalledWith({
      libraryId: 'photo',
      parentId: 'photo-root-raw',
    });
  });
});
