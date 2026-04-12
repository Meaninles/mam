import type { SettingSection, SettingsTab } from '../data';
import { ActionButton, InlineSettingControl } from '../components/Shared';
import { settingsTabs } from '../data';

export function SettingsPage({
  customContent,
  sections,
  settingsTab,
  setSettingsTab,
  onChangeSetting,
  onResetSettings,
  onSaveSettings,
}: {
  customContent?: React.ReactNode;
  sections: SettingSection[];
  settingsTab: SettingsTab;
  setSettingsTab: (value: SettingsTab) => void;
  onChangeSetting: (sectionId: string, rowId: string, value: string) => void;
  onResetSettings: () => void;
  onSaveSettings: () => void;
}) {
  return (
    <section className="page-stack">
      <div className="settings-header">
        <div className="settings-tabs">
          {settingsTabs.map((tab) => (
            <button
              key={tab.id}
              className={`settings-tab${tab.id === settingsTab ? ' active' : ''}`}
              type="button"
              onClick={() => setSettingsTab(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </div>
        {settingsTab !== 'tag-management' && settingsTab !== 'dependency-services' ? (
          <div className="toolbar-group wrap">
            <ActionButton onClick={onResetSettings}>恢复默认</ActionButton>
            <ActionButton tone="primary" onClick={onSaveSettings}>
              保存设置
            </ActionButton>
          </div>
        ) : null}
      </div>

      {customContent ? (
        customContent
      ) : (
        <div className="settings-layout single-column">
          {sections.map((section) => (
            <section className="content-card" key={section.id}>
              <header className="section-header">
                <strong>{section.title}</strong>
              </header>
              <div className="setting-list">
                {section.rows.map((row) => (
                  <div className="setting-row editable" key={row.id}>
                    <div className="setting-copy">
                      <span>{row.label}</span>
                      {row.description ? <small>{row.description}</small> : null}
                    </div>
                    <InlineSettingControl
                      control={row.control}
                      label={row.label}
                      options={row.options}
                      value={row.value}
                      onChange={(value) => onChangeSetting(section.id, row.id, value)}
                    />
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </section>
  );
}
