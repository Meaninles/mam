import type { SettingsTab } from '../data';
import { settingsContent, settingsTabs } from '../data';
import { SettingControl } from '../components/Shared';

export function SettingsPage({
  settingsTab,
  setSettingsTab,
}: {
  settingsTab: SettingsTab;
  setSettingsTab: (value: SettingsTab) => void;
}) {
  const sections = settingsContent[settingsTab];

  return (
    <section className="page-stack">
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

      <div className="settings-layout single-column">
        {sections.map((section) => (
          <section className="content-card" key={section.title}>
            <header className="section-header">
              <strong>{section.title}</strong>
            </header>
            <div className="setting-list">
              {section.rows.map((row) => (
                <div className="setting-row" key={row.label}>
                  <span>{row.label}</span>
                  <SettingControl control={row.control} value={row.value} />
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
    </section>
  );
}
