import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import {
  instancesApi,
  launcherApi,
  modsApi,
  settingsApi,
  type Account,
  type GameVersion,
  type InstalledMod,
  type Instance,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { formatDate, formatDuration } from "../shared/lib";
import {
  Button,
  Checkbox,
  Empty,
  Field,
  Modal,
  PageHeader,
  Select,
  StatusPill,
  SubmitForm,
} from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface LibraryPageProps {
  instances: Instance[];
  versions: GameVersion[];
  accounts: Account[];
  loading: boolean;
  refresh: () => Promise<void>;
  notify: Notify;
}

export function LibraryPage({
  instances,
  versions,
  accounts,
  loading,
  refresh,
  notify,
}: LibraryPageProps) {
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedInstance, setSelectedInstance] = useState<Instance>();
  const [query, setQuery] = useState("");

  const visibleInstances = useMemo(
    () =>
      instances.filter((instance) =>
        instance.name.toLowerCase().includes(query.toLowerCase()),
      ),
    [instances, query],
  );

  async function launch(instance: Instance) {
    try {
      const validation = await launcherApi.validate(
        instance.id,
        instance.defaultAccountId,
      );
      const issues = validation?.issues ?? [];
      const warnings = validation?.warnings ?? [];

      if (!validation?.valid) {
        throw new Error(issues.join(". ") || "The instance cannot be launched.");
      }
      if (
        warnings.length > 0 &&
        !window.confirm(`${warnings.join("\n")}\n\nLaunch anyway?`)
      ) {
        return;
      }

      await launcherApi.launch(instance.id, instance.defaultAccountId);
      notify(`Started “${instance.name}”`);
      await refresh();
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Your worlds"
        title="Library"
        description="Every Vintage Story instance, neatly arranged."
        action={
          <Button onClick={() => setCreateDialogOpen(true)}>
            ＋ New instance
          </Button>
        }
      />

      {instances.length > 0 && (
        <div className="toolbar">
          <div className="search">
            <span>⌕</span>
            <input
              aria-label="Search instances"
              placeholder="Find an instance…"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <span className="muted">
            {visibleInstances.length} {visibleInstances.length === 1 ? "instance" : "instances"}
          </span>
        </div>
      )}

      {loading ? (
        <div className="skeletonGrid">
          <i />
          <i />
          <i />
        </div>
      ) : visibleInstances.length === 0 ? (
        <Empty
          icon="◌"
          title={query ? "Nothing found" : "Light your first world"}
          description={
            query
              ? "Try another instance name."
              : "Create an isolated instance, select an installed game version, and start playing."
          }
          action={
            !query && (
              <Button
                onClick={() => setCreateDialogOpen(true)}
                disabled={versions.length === 0}
              >
                {versions.length > 0
                  ? "Create instance"
                  : "Install a game version first"}
              </Button>
            )
          }
        />
      ) : (
        <div className="instanceGrid">
          {visibleInstances.map((instance) => (
            <InstanceCard
              key={instance.id}
              instance={instance}
              version={versions.find(
                (version) => version.id === instance.gameVersionId,
              )}
              onOpen={() => setSelectedInstance(instance)}
              onLaunch={() => void launch(instance)}
              onStop={async () => {
                try {
                  await launcherApi.stop(instance.id);
                  notify("A graceful stop signal was sent");
                } catch (error) {
                  notify(errorMessage(error), "error");
                }
              }}
            />
          ))}
        </div>
      )}

      {createDialogOpen && (
        <CreateInstanceModal
          versions={versions}
          accounts={accounts}
          onClose={() => setCreateDialogOpen(false)}
          onDone={async () => {
            setCreateDialogOpen(false);
            await refresh();
            notify("Instance created");
          }}
        />
      )}

      {selectedInstance && (
        <InstanceModal
          instance={
            instances.find((item) => item.id === selectedInstance.id) ??
            selectedInstance
          }
          versions={versions}
          accounts={accounts}
          onClose={() => setSelectedInstance(undefined)}
          refresh={refresh}
          notify={notify}
        />
      )}
    </>
  );
}

interface InstanceCardProps {
  instance: Instance;
  version?: GameVersion;
  onOpen: () => void;
  onLaunch: () => void;
  onStop: () => Promise<void>;
}

function InstanceCard({
  instance,
  version,
  onOpen,
  onLaunch,
  onStop,
}: InstanceCardProps) {
  return (
    <article
      className="instanceCard"
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          onOpen();
        }
      }}
      tabIndex={0}
    >
      <div className="cover">
        <span className="coverLetter">W</span>
        <div className="coverGlow" />
        <StatusPill status={instance.status} />
      </div>

      <div className="cardBody">
        <div className="cardTitle">
          <h3>{instance.name}</h3>
          <button className="more" aria-label="Open instance details">
            •••
          </button>
        </div>

        <p>
          {instance.description || "A quiet place for your next adventure."}
        </p>

        <div className="meta">
          <span>◈ {version?.name ?? instance.gameVersionId}</span>
          <span>◇ {instance.enabledModCount} mods</span>
          <span>◷ {formatDuration(instance.playtimeSeconds)}</span>
        </div>

        <div className="cardFooter">
          <span>{formatDate(instance.lastPlayedAt)}</span>
          {instance.status === "running" ? (
            <Button
              variant="danger"
              onClick={(event) => {
                event.stopPropagation();
                void onStop();
              }}
            >
              Stop
            </Button>
          ) : (
            <Button
              onClick={(event) => {
                event.stopPropagation();
                onLaunch();
              }}
            >
              ▶ Play
            </Button>
          )}
        </div>
      </div>
    </article>
  );
}

interface CreateInstanceModalProps {
  versions: GameVersion[];
  accounts: Account[];
  onClose: () => void;
  onDone: () => Promise<void>;
}

function CreateInstanceModal({
  versions,
  accounts,
  onClose,
  onDone,
}: CreateInstanceModalProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [versionID, setVersionID] = useState(versions[0]?.id ?? "");
  const [accountID, setAccountID] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function createInstance() {
    setBusy(true);
    setError("");
    try {
      await instancesApi.create({
        name,
        description,
        gameVersionId: versionID,
        defaultAccountId: accountID || undefined,
        directory: "",
        launchArguments: [],
      });
      await onDone();
    } catch (createError) {
      setError(errorMessage(createError));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title="New instance" className="createInstanceDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={createInstance}>
        <div className="modalBody formFields">
          <Field label="Name">
            <input
              autoFocus
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="For example, A Warm Home"
            />
          </Field>

          <Field label="Description">
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="What makes this instance special?"
            />
          </Field>

          <div className="formRow">
            <Field label="Game version">
              <Select
                required
                value={versionID}
                onChange={(event) => setVersionID(event.target.value)}
              >
                {versions.map((version) => (
                  <option key={version.id} value={version.id}>
                    {version.name}
                  </option>
                ))}
              </Select>
            </Field>

            <Field label="Launch account">
              <Select
                value={accountID}
                onChange={(event) => setAccountID(event.target.value)}
              >
                <option value="">Use the globally selected account</option>
                {accounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.displayName}
                  </option>
                ))}
              </Select>
            </Field>
          </div>

          {error && <div className="inlineError" role="alert">{error}</div>}
        </div>

        <div className="dialogFooter">
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button busy={busy} disabled={versions.length === 0}>
            Create
          </Button>
        </div>
      </SubmitForm>
    </Modal>
  );
}

type InstanceTab = "overview" | "mods" | "settings";

interface InstanceModalProps {
  instance: Instance;
  versions: GameVersion[];
  accounts: Account[];
  onClose: () => void;
  refresh: () => Promise<void>;
  notify: Notify;
}

function InstanceModal({
  instance,
  versions,
  accounts,
  onClose,
  refresh,
  notify,
}: InstanceModalProps) {
  const navigate = useNavigate();
  const [tab, setTab] = useState<InstanceTab>("overview");
  const [mods, setMods] = useState<InstalledMod[]>([]);
  const [name, setName] = useState(instance.name);
  const [description, setDescription] = useState(instance.description);
  const [versionID, setVersionID] = useState(instance.gameVersionId);
  const [accountID, setAccountID] = useState(instance.defaultAccountId ?? "");
  const [argumentsText, setArgumentsText] = useState(
    instance.launchArguments.join(" "),
  );
  const [busy, setBusy] = useState(false);

  async function loadMods() {
    try {
      setMods((await modsApi.list(instance.id)) ?? []);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  useEffect(() => {
    void loadMods();
  }, [instance.id]);

  async function installMod() {
    try {
      const path = await settingsApi.selectModFile();
      if (!path) {
        return;
      }
      await modsApi.install({
        instanceId: instance.id,
        sourcePath: path,
        name: "",
        version: "",
      });
      await loadMods();
      await refresh();
      notify("Mod installed");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function saveSettings() {
    setBusy(true);
    try {
      await instancesApi.update({
        id: instance.id,
        name,
        description,
        gameVersionId: versionID,
        defaultAccountId: accountID || undefined,
        launchArguments: argumentsText.trim()
          ? argumentsText.trim().split(/\s+/)
          : [],
      });
      await refresh();
      notify("Instance settings saved");
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setBusy(false);
    }
  }

  async function deleteInstance() {
    const confirmed = window.confirm(
      `Delete “${instance.name}” and all of its files?`,
    );
    if (!confirmed) {
      return;
    }

    try {
      await instancesApi.remove(instance.id, true);
      onClose();
      await refresh();
      notify("Instance deleted");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  const selectedVersion = versions.find(
    (version) => version.id === instance.gameVersionId,
  );
  const selectedAccount = accounts.find(
    (account) => account.id === instance.defaultAccountId,
  );
  const settingsDirty =
    name !== instance.name ||
    description !== instance.description ||
    versionID !== instance.gameVersionId ||
    accountID !== (instance.defaultAccountId ?? "") ||
    argumentsText !== instance.launchArguments.join(" ");

  function resetSettings() {
    setName(instance.name);
    setDescription(instance.description);
    setVersionID(instance.gameVersionId);
    setAccountID(instance.defaultAccountId ?? "");
    setArgumentsText(instance.launchArguments.join(" "));
  }

  return (
    <Modal title={instance.name} className="instanceDialog" onClose={onClose}>
      <div className="tabs" role="tablist" aria-label="Instance details">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "overview"}
          className={tab === "overview" ? "active" : ""}
          onClick={() => setTab("overview")}
        >
          Overview
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "mods"}
          className={tab === "mods" ? "active" : ""}
          onClick={() => setTab("mods")}
        >
          Mods <b className="tabBadge">{mods.length}</b>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "settings"}
          className={tab === "settings" ? "active" : ""}
          onClick={() => setTab("settings")}
        >
          Settings
        </button>
      </div>

      {tab === "overview" && (
        <div className="instanceTabBody detailOverview" role="tabpanel">
          <section className="instanceHero">
            <div className="heroMark" aria-hidden="true">W</div>
            <div className="instanceHeroCopy">
              <div className="instanceHeroTitle">
                <h2 title={instance.name}>{instance.name}</h2>
                <StatusPill status={instance.status} />
              </div>
              <p className={instance.description ? "" : "placeholderCopy"}>
                {instance.description || "No description yet."}
              </p>
            </div>
          </section>

          <section className="instanceStats" aria-label="Instance statistics">
            <article>
              <span>Game version</span>
              <strong>{selectedVersion?.name ?? instance.gameVersionId}</strong>
            </article>
            <article>
              <span>Mods</span>
              <strong>{mods.filter((mod) => mod.enabled).length} installed</strong>
              <small>{mods.length} total</small>
            </article>
            <article>
              <span>Playtime</span>
              <strong>{formatDuration(instance.playtimeSeconds)}</strong>
            </article>
            <article>
              <span>Launch account</span>
              <strong>{selectedAccount?.displayName ?? "Global default"}</strong>
            </article>
          </section>

          <section className="storageSection">
            <div className="storageCopy">
              <span>Data directory</span>
              <code title={instance.directory}>{instance.directory}</code>
            </div>
            <Button
              variant="secondary"
              onClick={async () => {
                try {
                  await settingsApi.openDirectory(instance.directory);
                } catch (error) {
                  notify(errorMessage(error), "error");
                }
              }}
            >
              Open directory
            </Button>
          </section>
        </div>
      )}

      {tab === "mods" && (
        <div className="instanceTabBody modsTab" role="tabpanel">
          <header className="instanceToolbar">
            <div>
              <h3>Mods</h3>
              <p>Manage mods installed in this instance.</p>
            </div>
            <div className="row">
              <Button
                variant="secondary"
                onClick={() => navigate(`/mods?instanceId=${encodeURIComponent(instance.id)}`)}
              >
                Browse mods
              </Button>
              <Button onClick={() => void installMod()}>
                <span aria-hidden="true">＋</span> Install file
              </Button>
            </div>
          </header>

          {mods.length === 0 ? (
            <Empty
              icon="◇"
              title="No mods installed"
              description="Browse the mod catalog or install a local .zip, .cs, or .dll file."
              action={
                <div className="row">
                  <Button
                    variant="secondary"
                    onClick={() => navigate(`/mods?instanceId=${encodeURIComponent(instance.id)}`)}
                  >
                    Browse mods
                  </Button>
                  <Button onClick={() => void installMod()}>Install file</Button>
                </div>
              }
            />
          ) : (
            <div className="installedModList">
              {mods.map((mod) => (
                <article className="installedModRow" key={mod.id}>
                  <div className="modRowIcon" aria-hidden="true">◇</div>
                  <div className="modRowCopy">
                    <strong>{mod.name}</strong>
                    <small>Version {mod.version}</small>
                    <code title={mod.fileName}>{mod.fileName}</code>
                  </div>
                  <div className="modRowActions">
                    <Checkbox
                      label="Enabled"
                      checked={mod.enabled}
                      onChange={async (event) => {
                        try {
                          await modsApi.toggle(mod.id, event.target.checked);
                          await loadMods();
                          await refresh();
                        } catch (error) {
                          notify(errorMessage(error), "error");
                        }
                      }}
                    />
                    <Button
                      variant="ghost"
                      className="dangerGhost"
                      onClick={async () => {
                        if (!window.confirm(`Remove mod “${mod.name}”?`)) return;
                        try {
                          await modsApi.remove(mod.id);
                          await loadMods();
                          await refresh();
                        } catch (error) {
                          notify(errorMessage(error), "error");
                        }
                      }}
                    >
                      Remove
                    </Button>
                  </div>
                </article>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === "settings" && (
        <SubmitForm className="dialogForm settingsForm" onSubmit={saveSettings}>
          <div className="modalBody settingsBody" role="tabpanel">
            <section className="settingsSection">
              <header>
                <h3>General</h3>
                <p>Basic information and launch configuration.</p>
              </header>
              <div className="formFields">
                <Field label="Name">
                  <input value={name} onChange={(event) => setName(event.target.value)} />
                </Field>

                <Field label="Description">
                  <textarea value={description} onChange={(event) => setDescription(event.target.value)} />
                </Field>

                <div className="formRow">
                  <Field label="Game version">
                    <Select value={versionID} onChange={(event) => setVersionID(event.target.value)}>
                      {versions.map((version) => (
                        <option key={version.id} value={version.id}>{version.name}</option>
                      ))}
                    </Select>
                  </Field>

                  <Field label="Launch account">
                    <Select value={accountID} onChange={(event) => setAccountID(event.target.value)}>
                      <option value="">Use the globally selected account</option>
                      {accounts.map((account) => (
                        <option key={account.id} value={account.id}>{account.displayName}</option>
                      ))}
                    </Select>
                  </Field>
                </div>
              </div>
            </section>

            <section className="settingsSection advancedSection">
              <header>
                <h3>Advanced</h3>
                <p>Optional arguments passed when Vintage Story starts.</p>
              </header>
              <Field
                label="Launch arguments"
                hint="Arguments are separated by spaces. Waxlight always appends its isolated data path."
              >
                <input
                  className="codeInput"
                  value={argumentsText}
                  onChange={(event) => setArgumentsText(event.target.value)}
                  placeholder="--tracelog"
                />
              </Field>
            </section>

            <section className="dangerSection">
              <header>
                <h3>Danger zone</h3>
                <p>Irreversible actions for this instance.</p>
              </header>
              <div className="dangerZone">
                <div>
                  <strong>Delete instance</strong>
                  <small>Permanently removes this instance and its data directory.</small>
                </div>
                <Button type="button" variant="danger" onClick={() => void deleteInstance()}>
                  Delete instance
                </Button>
              </div>
            </section>
          </div>

          <div className="settingsFooter">
            <span className={settingsDirty ? "unsavedStatus active" : "unsavedStatus"}>
              {settingsDirty ? "Unsaved changes" : "All changes saved"}
            </span>
            <div className="row">
              <Button type="button" variant="ghost" disabled={!settingsDirty || busy} onClick={resetSettings}>
                Reset
              </Button>
              <Button busy={busy} disabled={!settingsDirty}>
                Save changes
              </Button>
            </div>
          </div>
        </SubmitForm>
      )}
    </Modal>
  );
}
