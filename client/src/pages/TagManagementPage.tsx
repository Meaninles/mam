import { useEffect, useMemo, useState } from 'react';
import {
  ArrowDown,
  ArrowUp,
  FolderPlus,
  Pin,
  PinOff,
  Plus,
  Save,
  Search,
  Shuffle,
  Trash2,
  X,
} from 'lucide-react';
import type { Library, Severity } from '../data';
import { ActionButton, EmptyState, IconButton, TonePill } from '../components/Shared';
import { fileCenterApi, type ManagedTag, type TagManagementSnapshot } from '../lib/fileCenterApi';

type FeedbackState = { message: string; tone: Severity } | null;

type CreateGroupState = {
  name: string;
  saving: boolean;
} | null;

type CreateTagState = {
  name: string;
  groupId: string;
  libraryIds: string[];
  isPinned: boolean;
  saving: boolean;
} | null;

type MergeState = {
  source: ManagedTag;
  targetId: string;
  saving: boolean;
} | null;

type DeleteState = {
  tag: ManagedTag;
  saving: boolean;
} | null;

type TagDraft = {
  name: string;
  groupId: string;
  libraryIds: string[];
  isPinned: boolean;
  mergeTargetId: string;
};

const ALL_GROUP_ID = 'all';

export function TagManagementPage({
  libraries,
  onFeedback,
}: {
  libraries: Library[];
  onFeedback?: (value: FeedbackState) => void;
}) {
  const [snapshot, setSnapshot] = useState<TagManagementSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchText, setSearchText] = useState('');
  const [selectedGroupId, setSelectedGroupId] = useState<string>(ALL_GROUP_ID);
  const [selectedTagId, setSelectedTagId] = useState<string | null>(null);
  const [tagDraft, setTagDraft] = useState<TagDraft | null>(null);
  const [groupNameDraft, setGroupNameDraft] = useState('');
  const [createGroupState, setCreateGroupState] = useState<CreateGroupState>(null);
  const [createTagState, setCreateTagState] = useState<CreateTagState>(null);
  const [mergeState, setMergeState] = useState<MergeState>(null);
  const [deleteState, setDeleteState] = useState<DeleteState>(null);
  const [feedback, setFeedback] = useState<FeedbackState>(null);

  useEffect(() => {
    void refreshSnapshot();
  }, []);

  useEffect(() => {
    onFeedback?.(feedback);
  }, [feedback, onFeedback]);

  useEffect(() => {
    const unsubscribe = fileCenterApi.subscribe(() => {
      void refreshSnapshot();
    });
    return unsubscribe;
  }, []);

  const visibleTags = useMemo(() => {
    if (!snapshot) {
      return [];
    }
    const keyword = searchText.trim().toLocaleLowerCase('zh-CN');
    return snapshot.tags.filter((tag) => {
      const matchesGroup = selectedGroupId === ALL_GROUP_ID ? true : tag.groupId === selectedGroupId;
      const matchesKeyword = keyword ? tag.name.toLocaleLowerCase('zh-CN').includes(keyword) : true;
      return matchesGroup && matchesKeyword;
    });
  }, [searchText, selectedGroupId, snapshot]);

  const selectedTag = useMemo(
    () => visibleTags.find((tag) => tag.id === selectedTagId) ?? snapshot?.tags.find((tag) => tag.id === selectedTagId) ?? null,
    [selectedTagId, snapshot, visibleTags],
  );

  useEffect(() => {
    if (!visibleTags.length) {
      setSelectedTagId(null);
      return;
    }
    if (!selectedTagId || !visibleTags.some((tag) => tag.id === selectedTagId)) {
      setSelectedTagId(visibleTags[0]?.id ?? null);
    }
  }, [selectedTagId, visibleTags]);

  useEffect(() => {
    if (!selectedTag) {
      setTagDraft(null);
      return;
    }
    setTagDraft({
      name: selectedTag.name,
      groupId: selectedTag.groupId,
      libraryIds: [...selectedTag.libraryIds],
      isPinned: selectedTag.isPinned,
      mergeTargetId: '',
    });
  }, [selectedTag]);

  useEffect(() => {
    const selectedGroup =
      snapshot?.groups.find((group) => group.id === selectedGroupId) ?? snapshot?.groups[0] ?? null;
    setGroupNameDraft(selectedGroup && selectedGroupId !== ALL_GROUP_ID ? selectedGroup.name : '');
  }, [selectedGroupId, snapshot]);

  async function refreshSnapshot(preferredTagId?: string | null) {
    setLoading(true);
    try {
      const next = await fileCenterApi.loadTagManagementSnapshot();
      setSnapshot(next);
      if (preferredTagId) {
        setSelectedTagId(preferredTagId);
      }
    } catch (error) {
      setFeedback({
        message: error instanceof Error ? error.message : '加载标签管理失败，请稍后重试',
        tone: 'critical',
      });
    } finally {
      setLoading(false);
    }
  }

  async function handleSaveGroup() {
    if (selectedGroupId === ALL_GROUP_ID) {
      return;
    }
    try {
      const result = await fileCenterApi.updateTagGroup(selectedGroupId, groupNameDraft);
      setFeedback({ message: result.message, tone: 'success' });
      await refreshSnapshot();
    } catch (error) {
      setFeedback({ message: error instanceof Error ? error.message : '分组更新失败', tone: 'critical' });
    }
  }

  async function handleCreateGroup() {
    if (!createGroupState) return;
    setCreateGroupState((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await fileCenterApi.createTagGroup(createGroupState.name);
      setFeedback({ message: result.message, tone: 'success' });
      setCreateGroupState(null);
      setSelectedGroupId(result.groupId);
      await refreshSnapshot(result.groupId);
    } catch (error) {
      setCreateGroupState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: error instanceof Error ? error.message : '分组创建失败', tone: 'critical' });
    }
  }

  async function handleMoveGroup(direction: 'up' | 'down') {
    if (selectedGroupId === ALL_GROUP_ID) {
      return;
    }
    try {
      const result = await fileCenterApi.moveTagGroup(selectedGroupId, direction);
      setFeedback({ message: result.message, tone: 'success' });
      await refreshSnapshot();
    } catch (error) {
      setFeedback({ message: error instanceof Error ? error.message : '分组排序失败', tone: 'critical' });
    }
  }

  async function handleCreateTag() {
    if (!createTagState) return;
    setCreateTagState((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await fileCenterApi.createManagedTag({
        name: createTagState.name,
        groupId: createTagState.groupId,
        libraryIds: createTagState.libraryIds,
        isPinned: createTagState.isPinned,
      });
      setFeedback({ message: result.message, tone: 'success' });
      setCreateTagState(null);
      setSelectedGroupId(createTagState.groupId);
      setSelectedTagId(result.tagId);
      await refreshSnapshot(result.tagId);
    } catch (error) {
      setCreateTagState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: error instanceof Error ? error.message : '标签创建失败', tone: 'critical' });
    }
  }

  async function handleSaveTag() {
    if (!selectedTag || !tagDraft) {
      return;
    }
    try {
      const result = await fileCenterApi.updateManagedTag(selectedTag.id, {
        name: tagDraft.name,
        groupId: tagDraft.groupId,
        libraryIds: tagDraft.libraryIds,
        isPinned: tagDraft.isPinned,
      });
      setFeedback({ message: result.message, tone: 'success' });
      await refreshSnapshot(selectedTag.id);
    } catch (error) {
      setFeedback({ message: error instanceof Error ? error.message : '标签更新失败', tone: 'critical' });
    }
  }

  async function handleConfirmMerge() {
    if (!mergeState || !mergeState.targetId) {
      return;
    }
    setMergeState((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await fileCenterApi.mergeManagedTag(mergeState.source.id, mergeState.targetId);
      setFeedback({ message: result.message, tone: 'success' });
      setMergeState(null);
      await refreshSnapshot();
    } catch (error) {
      setMergeState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: error instanceof Error ? error.message : '标签合并失败', tone: 'critical' });
    }
  }

  async function handleConfirmDelete() {
    if (!deleteState) {
      return;
    }
    setDeleteState((current) => (current ? { ...current, saving: true } : current));
    try {
      const result = await fileCenterApi.deleteManagedTag(deleteState.tag.id);
      setFeedback({ message: result.message, tone: 'warning' });
      setDeleteState(null);
      await refreshSnapshot();
    } catch (error) {
      setDeleteState((current) => (current ? { ...current, saving: false } : current));
      setFeedback({ message: error instanceof Error ? error.message : '标签删除失败', tone: 'critical' });
    }
  }

  const selectedGroup =
    snapshot?.groups.find((group) => group.id === selectedGroupId) ?? null;

  return (
    <section className="page-stack tag-management-page">
      <div className="tag-management-stats">
        <TagSummaryBadge label="标签总数" value={String(snapshot?.overview.totalTags ?? 0)} />
        <TagSummaryBadge label="使用中标签" tone="success" value={String(snapshot?.overview.usedTagCount ?? 0)} />
        <TagSummaryBadge label="未分组标签" tone="warning" value={String(snapshot?.overview.ungroupedTagCount ?? 0)} />
        <TagSummaryBadge label="跨库标签" tone="info" value={String(snapshot?.overview.crossLibraryTagCount ?? 0)} />
      </div>

      <div className="toolbar-card action-toolbar tag-management-toolbar">
        <div className="toolbar-group wrap storage-toolbar-main">
          <label className="search-field">
            <Search size={14} />
            <input
              aria-label="搜索标签"
              placeholder="搜索标签名称"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
          </label>
        </div>
        <div className="toolbar-group wrap">
          <ActionButton onClick={() => setCreateGroupState({ name: '', saving: false })}>
            <FolderPlus size={14} />
            新增分组
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={() =>
              setCreateTagState({
                name: '',
                groupId: selectedGroupId === ALL_GROUP_ID ? 'tag-group-project' : selectedGroupId,
                libraryIds: [],
                isPinned: false,
                saving: false,
              })
            }
          >
            <Plus size={14} />
            新增标签
          </ActionButton>
        </div>
      </div>

      <div className="tag-management-layout">
        <section className="workspace-card tag-management-browser">
          <div className="tag-group-pane">
            <button
              className={`tag-group-item${selectedGroupId === ALL_GROUP_ID ? ' active' : ''}`}
              type="button"
              onClick={() => setSelectedGroupId(ALL_GROUP_ID)}
            >
              <div>
                <strong>全部标签</strong>
                <span>查看当前标签库全部条目</span>
              </div>
              <TonePill tone="info">{snapshot?.overview.totalTags ?? 0}</TonePill>
            </button>

            {snapshot?.groups.map((group) => (
              <button
                key={group.id}
                className={`tag-group-item${group.id === selectedGroupId ? ' active' : ''}`}
                type="button"
                onClick={() => setSelectedGroupId(group.id)}
              >
                <div>
                  <strong>{group.name}</strong>
                  <span>{group.usedTagCount}/{group.tagCount} 使用中</span>
                </div>
                <TonePill tone={group.id === 'tag-group-ungrouped' ? 'warning' : 'info'}>{group.tagCount}</TonePill>
              </button>
            ))}

            {selectedGroup && selectedGroupId !== ALL_GROUP_ID ? (
              <div className="tag-group-editor">
                <label className="form-field">
                  <span>分组名称</span>
                  <input
                    aria-label="分组名称"
                    value={groupNameDraft}
                    onChange={(event) => setGroupNameDraft(event.target.value)}
                  />
                </label>
                <div className="toolbar-group wrap">
                  <IconButton ariaLabel="分组上移" onClick={() => void handleMoveGroup('up')}>
                    <ArrowUp size={15} />
                  </IconButton>
                  <IconButton ariaLabel="分组下移" onClick={() => void handleMoveGroup('down')}>
                    <ArrowDown size={15} />
                  </IconButton>
                  <ActionButton onClick={() => void handleSaveGroup()}>
                    <Save size={14} />
                    保存分组
                  </ActionButton>
                </div>
              </div>
            ) : null}
          </div>

          <div className="tag-list-pane">
            <div className="section-header">
              <div className="tag-list-title">
                <strong>{selectedGroupId === ALL_GROUP_ID ? '标签列表' : selectedGroup?.name ?? '标签列表'}</strong>
                <span className="selection-caption">共 {visibleTags.length} 项</span>
              </div>
            </div>

            {loading ? (
              <div className="empty-state">
                <strong>正在加载标签管理</strong>
                <p>正在准备标签分组、作用域和使用统计。</p>
              </div>
            ) : visibleTags.length === 0 ? (
              <EmptyState title="没有匹配的标签" description="可以调整搜索条件，或直接创建一个新的标签。" />
            ) : (
              <div className="tag-list">
                {visibleTags.map((tag) => (
                  <article
                    key={tag.id}
                    className={`tag-list-item${tag.id === selectedTagId ? ' active' : ''}`}
                  >
                    <button
                      aria-label={tag.name}
                      className="tag-list-item-main"
                      type="button"
                      onClick={() => setSelectedTagId(tag.id)}
                    >
                      <div className="tag-list-item-head">
                        <strong>{tag.name}</strong>
                        {tag.isPinned ? <TonePill tone="info">置顶</TonePill> : null}
                      </div>
                      <div className="endpoint-row">
                        <TonePill tone="success">{tag.groupName}</TonePill>
                        <span className="selection-caption">{tag.usageCount} 次使用</span>
                        <span className="selection-caption">
                          {tag.libraryIds.length === 0 ? '当前无可用资产库' : `${tag.libraryIds.length} 个作用资产库`}
                        </span>
                      </div>
                    </button>
                    <div className="row-actions">
                      <IconButton
                        ariaLabel={tag.isPinned ? `取消置顶 ${tag.name}` : `置顶 ${tag.name}`}
                        onClick={() => {
                          setSelectedTagId(tag.id);
                          setTagDraft({
                            name: tag.name,
                            groupId: tag.groupId,
                            libraryIds: [...tag.libraryIds],
                            isPinned: !tag.isPinned,
                            mergeTargetId: '',
                          });
                          void fileCenterApi
                            .updateManagedTag(tag.id, {
                              name: tag.name,
                              groupId: tag.groupId,
                              libraryIds: tag.libraryIds,
                              isPinned: !tag.isPinned,
                            })
                            .then((result) => {
                              setFeedback({ message: result.message, tone: 'success' });
                              return refreshSnapshot(tag.id);
                            })
                            .catch((error) => {
                              setFeedback({ message: error instanceof Error ? error.message : '标签更新失败', tone: 'critical' });
                            });
                        }}
                      >
                        {tag.isPinned ? <PinOff size={15} /> : <Pin size={15} />}
                      </IconButton>
                      <IconButton
                        ariaLabel={`标签上移 ${tag.name}`}
                        onClick={() => {
                          setSelectedTagId(tag.id);
                          void fileCenterApi
                            .moveManagedTag(tag.id, 'up')
                            .then((result) => {
                              setFeedback({ message: result.message, tone: 'success' });
                              return refreshSnapshot(tag.id);
                            })
                            .catch((error) => {
                              setFeedback({ message: error instanceof Error ? error.message : '标签排序失败', tone: 'critical' });
                            });
                        }}
                      >
                        <ArrowUp size={15} />
                      </IconButton>
                      <IconButton
                        ariaLabel={`标签下移 ${tag.name}`}
                        onClick={() => {
                          setSelectedTagId(tag.id);
                          void fileCenterApi
                            .moveManagedTag(tag.id, 'down')
                            .then((result) => {
                              setFeedback({ message: result.message, tone: 'success' });
                              return refreshSnapshot(tag.id);
                            })
                            .catch((error) => {
                              setFeedback({ message: error instanceof Error ? error.message : '标签排序失败', tone: 'critical' });
                            });
                        }}
                      >
                        <ArrowDown size={15} />
                      </IconButton>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </div>
        </section>

        <section className="content-card tag-detail-panel" aria-label="标签详情" role="region">
          {selectedTag && tagDraft ? (
            <>
              <header className="section-header">
                <div>
                  <strong>{selectedTag.name}</strong>
                  <p className="muted-paragraph">管理名称、分组、作用资产库和危险操作。</p>
                </div>
                <TonePill tone={selectedTag.usageCount > 0 ? 'success' : 'warning'}>
                  {selectedTag.usageCount > 0 ? '使用中' : '未使用'}
                </TonePill>
              </header>

              <div className="sheet-form">
                <label className="form-field">
                  <span>标签名称</span>
                  <input
                    aria-label="标签名称"
                    value={tagDraft.name}
                    onChange={(event) => setTagDraft((current) => (current ? { ...current, name: event.target.value } : current))}
                  />
                </label>

                <label className="form-field">
                  <span>所属分组</span>
                  <select
                    aria-label="所属分组"
                    value={tagDraft.groupId}
                    onChange={(event) =>
                      setTagDraft((current) => (current ? { ...current, groupId: event.target.value } : current))
                    }
                  >
                    {snapshot?.groups.map((group) => (
                      <option key={group.id} value={group.id}>
                        {group.name}
                      </option>
                    ))}
                  </select>
                </label>

                <div className="form-field">
                  <span>作用资产库</span>
                  <div className="checkbox-row">
                    {libraries.map((library) => {
                      const checked = tagDraft.libraryIds.includes(library.id);
                      return (
                        <label key={library.id} className={`scope-check${checked ? ' checked' : ''}`}>
                          <input
                            aria-label={library.name}
                            checked={checked}
                            type="checkbox"
                            onChange={() =>
                              setTagDraft((current) => {
                                if (!current) return current;
                                return {
                                  ...current,
                                  libraryIds: checked
                                    ? current.libraryIds.filter((id) => id !== library.id)
                                    : [...current.libraryIds, library.id],
                                };
                              })
                            }
                          />
                          <span>{library.name}</span>
                        </label>
                      );
                    })}
                  </div>
                </div>

                <div className="toolbar-group wrap">
                  <ActionButton
                    onClick={() =>
                      setTagDraft((current) => (current ? { ...current, isPinned: !current.isPinned } : current))
                    }
                  >
                    {tagDraft.isPinned ? <PinOff size={14} /> : <Pin size={14} />}
                    {tagDraft.isPinned ? '取消置顶' : '设为置顶'}
                  </ActionButton>
                  <ActionButton tone="primary" onClick={() => void handleSaveTag()}>
                    <Save size={14} />
                    保存标签配置
                  </ActionButton>
                </div>
              </div>

              <div className="sheet-section">
                <div className="dense-row">
                  <span>所属分组</span>
                  <strong>{selectedTag.groupName}</strong>
                </div>
                <div className="dense-row">
                  <span>使用次数</span>
                  <strong>{selectedTag.usageCount}</strong>
                </div>
                <div className="dense-row">
                  <span>作用资产库</span>
                  <strong>{selectedTag.libraryIds.length}</strong>
                </div>
                <div className="dense-row">
                  <span>涉及资产库</span>
                  <strong>{selectedTag.linkedLibraryIds.length}</strong>
                </div>
              </div>

              {selectedTag.outOfScopeUsageCount > 0 ? (
                <div className="inline-warning">
                  当前有 {selectedTag.outOfScopeUsageCount} 次历史引用超出作用范围，本期不会自动移除。
                </div>
              ) : null}

              <div className="sheet-section danger-zone">
                <div className="section-header">
                  <div>
                    <strong>合并标签（危险操作）</strong>
                    <p className="muted-paragraph">合并标签需二次确认，请确认范围。</p>
                  </div>
                </div>
                <label className="form-field">
                  <span>合并到目标标签</span>
                  <select
                    aria-label="合并到目标标签"
                    value={tagDraft.mergeTargetId}
                    onChange={(event) =>
                      setTagDraft((current) => (current ? { ...current, mergeTargetId: event.target.value } : current))
                    }
                  >
                    <option value="">请选择目标标签</option>
                    {snapshot?.tags
                      .filter((tag) => tag.id !== selectedTag.id)
                      .map((tag) => (
                        <option key={tag.id} value={tag.id}>
                          {tag.name}
                        </option>
                      ))}
                  </select>
                </label>
                <div className="toolbar-group wrap">
                  <ActionButton
                    onClick={() => {
                      if (!tagDraft.mergeTargetId) {
                        setFeedback({ message: '请先选择要合并到的目标标签', tone: 'warning' });
                        return;
                      }
                      setMergeState({ source: selectedTag, targetId: tagDraft.mergeTargetId, saving: false });
                    }}
                  >
                    <Shuffle size={14} />
                    合并标签
                  </ActionButton>
                  <ActionButton
                    className="danger-text"
                    onClick={() => setDeleteState({ tag: selectedTag, saving: false })}
                  >
                    <Trash2 size={14} />
                    删除标签
                  </ActionButton>
                </div>
              </div>
            </>
          ) : (
            <EmptyState title="请选择一个标签" description="从左侧标签列表中选择一个标签后，可在这里编辑详情与执行危险操作。" />
          )}
        </section>
      </div>

      {createGroupState ? (
        <Dialog title="新增分组" onClose={() => setCreateGroupState(null)}>
          <div className="sheet-form">
            <label className="form-field">
              <span>分组名称</span>
              <input
                aria-label="分组名称"
                value={createGroupState.name}
                onChange={(event) => setCreateGroupState((current) => (current ? { ...current, name: event.target.value } : current))}
              />
            </label>
          </div>
          <div className="sheet-actions right">
            <ActionButton onClick={() => setCreateGroupState(null)}>取消</ActionButton>
            <ActionButton tone="primary" onClick={() => void handleCreateGroup()}>
              创建分组
            </ActionButton>
          </div>
        </Dialog>
      ) : null}

      {createTagState ? (
        <Dialog title="新增标签" onClose={() => setCreateTagState(null)}>
          <div className="sheet-form">
            <label className="form-field">
              <span>标签名称</span>
              <input
                aria-label="标签名称"
                value={createTagState.name}
                onChange={(event) =>
                  setCreateTagState((current) => (current ? { ...current, name: event.target.value } : current))
                }
              />
            </label>
            <label className="form-field">
              <span>所属分组</span>
              <select
                aria-label="所属分组"
                value={createTagState.groupId}
                onChange={(event) =>
                  setCreateTagState((current) => (current ? { ...current, groupId: event.target.value } : current))
                }
              >
                {snapshot?.groups.map((group) => (
                  <option key={group.id} value={group.id}>
                    {group.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="form-field">
              <span>作用资产库</span>
              <div className="checkbox-row">
                {libraries.map((library) => {
                  const checked = createTagState.libraryIds.includes(library.id);
                  return (
                    <label key={library.id} className={`scope-check${checked ? ' checked' : ''}`}>
                      <input
                        aria-label={library.name}
                        checked={checked}
                        type="checkbox"
                        onChange={() =>
                          setCreateTagState((current) => {
                            if (!current) return current;
                            return {
                              ...current,
                              libraryIds: checked
                                ? current.libraryIds.filter((id) => id !== library.id)
                                : [...current.libraryIds, library.id],
                            };
                          })
                        }
                      />
                      <span>{library.name}</span>
                    </label>
                  );
                })}
              </div>
            </div>
            <label className="tag-checkbox">
              <input
                aria-label="置顶标签"
                checked={createTagState.isPinned}
                type="checkbox"
                onChange={(event) =>
                  setCreateTagState((current) => (current ? { ...current, isPinned: event.target.checked } : current))
                }
              />
              <span>创建后立即置顶</span>
            </label>
          </div>
          <div className="sheet-actions right">
            <ActionButton onClick={() => setCreateTagState(null)}>取消</ActionButton>
            <ActionButton tone="primary" onClick={() => void handleCreateTag()}>
              创建标签
            </ActionButton>
          </div>
        </Dialog>
      ) : null}

      {mergeState ? (
        <Dialog title="确认合并标签" onClose={() => setMergeState(null)}>
          <div className="dialog-card">
            <p className="muted-paragraph">
              将把“{mergeState.source.name}”的资产关联迁移到“
              {snapshot?.tags.find((tag) => tag.id === mergeState.targetId)?.name ?? '目标标签'}
              ”，并自动去重重复关联。
            </p>
          </div>
          <div className="sheet-actions right">
            <ActionButton onClick={() => setMergeState(null)}>取消</ActionButton>
            <ActionButton tone="primary" onClick={() => void handleConfirmMerge()}>
              确认合并
            </ActionButton>
          </div>
        </Dialog>
      ) : null}

      {deleteState ? (
        <Dialog title="确认删除标签" onClose={() => setDeleteState(null)}>
          <div className="dialog-card">
            <p className="muted-paragraph">
              将删除标签“{deleteState.tag.name}”，并解除所有关联。影响资产 {deleteState.tag.usageCount} 项，
              涉及资产库 {deleteState.tag.linkedLibraryIds.length} 个。
            </p>
          </div>
          <div className="sheet-actions right">
            <ActionButton onClick={() => setDeleteState(null)}>取消</ActionButton>
            <ActionButton tone="danger" onClick={() => void handleConfirmDelete()}>
              确认删除
            </ActionButton>
          </div>
        </Dialog>
      ) : null}
    </section>
  );
}

function TagSummaryBadge({
  label,
  tone = 'default',
  value,
}: {
  label: string;
  tone?: Severity | 'default';
  value: string;
}) {
  return (
    <div className={`tag-summary-badge${tone !== 'default' ? ` ${tone}` : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function Dialog({
  children,
  onClose,
  title,
}: {
  children: React.ReactNode;
  onClose: () => void;
  title: string;
}) {
  return (
    <div className="dialog-backdrop" role="presentation" onClick={onClose}>
      <section
        aria-label={title}
        className="dialog-panel"
        role="dialog"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="sheet-header">
          <strong>{title}</strong>
          <IconButton ariaLabel="关闭" onClick={onClose}>
            <X size={15} />
          </IconButton>
        </div>
        {children}
      </section>
    </div>
  );
}
