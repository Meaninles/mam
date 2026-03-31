import type { FileTypeFilter, RecycleRecord } from '../data';
import { SelectPill } from '../components/Shared';

export function RecyclePage({
  endpointFilter,
  items,
  setEndpointFilter,
  setTypeFilter,
  typeFilter,
}: {
  endpointFilter: string;
  items: RecycleRecord[];
  setEndpointFilter: (value: string) => void;
  setTypeFilter: (value: FileTypeFilter) => void;
  typeFilter: FileTypeFilter;
}) {
  return (
    <section className="page-stack">
      <div className="toolbar-card">
        <div className="toolbar-group">
          <SelectPill
            ariaLabel="文件类型"
            options={['全部', '图片', '视频', '音频', '文档']}
            value={typeFilter}
            onChange={(value) => setTypeFilter(value as FileTypeFilter)}
          />
          <SelectPill
            ariaLabel="所在端点"
            options={['全部', '本地NVMe', '影像NAS', '115']}
            value={endpointFilter}
            onChange={setEndpointFilter}
          />
        </div>
      </div>

      <div className="workspace-card compact-list">
        {items.map((item) => (
          <div className="list-row" key={item.id}>
            <div className="row-main">
              <strong>{item.name}</strong>
              <span>{item.originalPath}</span>
            </div>
            <span>{item.fileType}</span>
            <span>{item.endpoint}</span>
            <span>{item.deletedAt}</span>
            <span>{item.size}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
