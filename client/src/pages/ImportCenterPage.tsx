import { ArrowDownToLine, Target } from 'lucide-react';
import type { ImportBatch, ImportSourceFile } from '../data';
import { ActionButton, DenseRow, EmptyState, SelectPill } from '../components/Shared';

export function ImportCenterPage(props: {
  batches: ImportBatch[];
  selectedBatch: ImportBatch;
  selectedImportTargets: Record<string, string[]>;
  submitDisabled: boolean;
  validationMessage: string | null;
  visibleFiles: ImportSourceFile[];
  onApplyTargetToAll: (target: string) => void;
  onSetSelectedBatchId: (value: string) => void;
  onSetSelectedImportTargets: React.Dispatch<React.SetStateAction<Record<string, string[]>>>;
  onSubmitImport: () => void;
}) {
  const {
    batches,
    selectedBatch,
    selectedImportTargets,
    submitDisabled,
    validationMessage,
    visibleFiles,
    onApplyTargetToAll,
    onSetSelectedBatchId,
    onSetSelectedImportTargets,
    onSubmitImport,
  } = props;

  return (
    <section className="page-stack">
      <div className="toolbar-card action-toolbar">
        <div className="toolbar-group wrap">
          <SelectPill
            ariaLabel="导入批次"
            options={batches.map((batch) => batch.name)}
            value={selectedBatch.name}
            onChange={(value) => {
              const batch = batches.find((item) => item.name === value) ?? null;
              if (batch) {
                onSetSelectedBatchId(batch.id);
              }
            }}
          />
          {selectedBatch.targetCandidates.map((target) => (
            <ActionButton key={target} onClick={() => onApplyTargetToAll(target)}>
              <Target size={14} />
              全部加入 {target}
            </ActionButton>
          ))}
        </div>

        <ActionButton tone="primary" onClick={onSubmitImport}>
          <ArrowDownToLine size={14} />
          提交导入任务
        </ActionButton>
      </div>

      {validationMessage ? <div className="inline-warning">{validationMessage}</div> : null}

      <div className="import-layout">
        <div className="workspace-card compact-card">
          <header className="section-header">
            <strong>来源</strong>
          </header>
          <DenseRow label="类型" value={selectedBatch.sourceType} />
          <DenseRow label="路径" value={selectedBatch.source} />
          <DenseRow label="文件规模" value={selectedBatch.fileCount} />
          <DenseRow label="状态" value={selectedBatch.status} />
        </div>

        <div className="workspace-card">
          <header className="section-header">
            <strong>待导入文件</strong>
            <span className="muted-text">每个文件至少选择一个目标端</span>
          </header>
          {visibleFiles.length === 0 ? (
            <EmptyState title="当前批次暂无文件" description="可以重新扫描来源，或切换到其他待导入批次。" />
          ) : (
            <table className="file-table compact-table">
              <thead>
                <tr>
                  <th scope="col">文件</th>
                  <th scope="col">类型</th>
                  <th scope="col">大小</th>
                  <th scope="col">导入端</th>
                  <th scope="col">状态</th>
                </tr>
              </thead>
              <tbody>
                {visibleFiles.map((file) => (
                  <tr key={file.id}>
                    <td>
                      <div className="inline-file">
                        <strong>{file.name}</strong>
                        <span>{file.relativePath}</span>
                      </div>
                    </td>
                    <td>{file.type}</td>
                    <td>{file.size}</td>
                    <td>
                      <div className="checkbox-row">
                        {selectedBatch.targetCandidates.map((target) => {
                          const checked = (selectedImportTargets[file.id] ?? []).includes(target);
                          return (
                            <label key={`${file.id}-${target}`} className={`target-check${checked ? ' checked' : ''}`}>
                              <input
                                checked={checked}
                                type="checkbox"
                                onChange={() =>
                                  onSetSelectedImportTargets((current) => {
                                    const currentTargets = current[file.id] ?? [];
                                    return {
                                      ...current,
                                      [file.id]: checked
                                        ? currentTargets.filter((item) => item !== target)
                                        : [...currentTargets, target],
                                    };
                                  })
                                }
                              />
                              <span>{target}</span>
                            </label>
                          );
                        })}
                      </div>
                    </td>
                    <td>{file.status}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {submitDisabled ? (
        <p className="muted-text">当前批次已经提交，可在任务中心继续查看执行情况。</p>
      ) : null}
    </section>
  );
}
