import { useEffect, useState } from "react";

import { Settings, settingsApi } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Button, Field, PageHeader, SubmitForm } from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface SettingsPageProps {
  settings?: Settings;
  notify: Notify;
  onSaved: (settings: Settings) => void;
}

export function SettingsPage({
  settings,
  notify,
  onSaved,
}: SettingsPageProps) {
  const [value, setValue] = useState<Settings>();
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setValue(settings);
  }, [settings]);

  if (!value) {
    return null;
  }

  async function save() {
    if (!value) {
      return;
    }

    setBusy(true);

    try {
      const saved = await settingsApi.update({
        ...value,
        language: "en",
      });
      onSaved(saved);
      notify("Settings saved");
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Make it yours"
        title="Settings"
        description="Basic Waxlight and game launch preferences."
      />

      <section className="settingsPanel">
        <SubmitForm className="form" onSubmit={save}>
          <h2>Interface</h2>

          <div className="formRow">
            <Field label="Language">
              <select value="en" disabled>
                <option value="en">English</option>
              </select>
            </Field>

            <Field label="Theme">
              <select
                value={value.theme}
                onChange={(event) =>
                  setValue({ ...value, theme: event.target.value })
                }
              >
                <option value="dark">Dark</option>
                <option value="system">System</option>
              </select>
            </Field>
          </div>

          <h2>Downloads and game</h2>

          <div className="formRow">
            <Field label="Parallel downloads">
              <input
                type="number"
                min={1}
                max={10}
                value={value.downloadsParallel}
                onChange={(event) =>
                  setValue({
                    ...value,
                    downloadsParallel: Number(event.target.value),
                  })
                }
              />
            </Field>

            <Field label="Minimum session duration, seconds">
              <input
                type="number"
                min={0}
                value={value.minSessionDurationSec}
                onChange={(event) =>
                  setValue({
                    ...value,
                    minSessionDurationSec: Number(event.target.value),
                  })
                }
              />
            </Field>
          </div>

          <Field label="Global launch arguments">
            <input
              value={value.globalLaunchArguments.join(" ")}
              onChange={(event) => {
                const argumentsValue = event.target.value.trim();
                setValue({
                  ...value,
                  globalLaunchArguments: argumentsValue
                    ? argumentsValue.split(/\s+/)
                    : [],
                });
              }}
              placeholder="--debug"
            />
          </Field>

          <label className="checkRow">
            <input
              type="checkbox"
              checked={value.confirmDeletion}
              onChange={(event) =>
                setValue({ ...value, confirmDeletion: event.target.checked })
              }
            />
            <span>
              <strong>Confirm deletion</strong>
              <small>Ask before removing instances, versions, and mods.</small>
            </span>
          </label>

          <div className="modalActions">
            <Button busy={busy}>Save settings</Button>
          </div>
        </SubmitForm>
      </section>

      <footer className="legal">
        Waxlight Launcher is not affiliated with or endorsed by the developers
        of Vintage Story.
      </footer>
    </>
  );
}
