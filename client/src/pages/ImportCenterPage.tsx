import type { ImportBatch, ImportSourceFile } from '../data';
import { DenseRow, SelectPill } from '../components/Shared';

export function ImportCenterPage(props: {
  batches: ImportBatch[];
  selectedBatch: ImportBatch;
  selectedImportTargets: Record<string, string[]>;
  setSelectedBatchId: (value: string) => void;
  setSelectedImportTargets: React.Dispatch<React.SetStateAction<Record<string, string[]>>>;
  visibleFiles: ImportSourceFile[];
}) {
  const {
    batches,
    selectedBatch,
    selectedImportTargets,
    setSelectedBatchId,
    setSelectedImportTargets,
    visibleFiles,
  } = props;

  return (
    <section className="page-stack">
      <div className="toolbar-card">
        <div className="toolbar-group">
          <SelectPill
            ariaLabel="导入批次"
            options={batches.map((batch) => batch.name)}
            value={selectedBatch.name}
            onChange={(value) => {
              const batch = batches.find((item) => item.name === value) ?? null;
              if (batch) {
                setSelectedBatchId(batch.id);
              }
            }}
          />
        </div>
      </div>

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
          </header>
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
                                setSelectedImportTargets((current) => {
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
        </div>
      </div>
    </section>
  );
}
