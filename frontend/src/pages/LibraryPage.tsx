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
  Empty,
  Field,
  Modal,
  PageHeader,
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
    <Modal title="New instance" onClose={onClose}>
      <SubmitForm className="form" onSubmit={createInstance}>
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
            <select
              required
              value={versionID}
              onChange={(event) => setVersionID(event.target.value)}
            >
              {versions.map((version) => (
                <option key={version.id} value={version.id}>
                  {version.name}
                </option>
              ))}
            </select>
          </Field>

          <Field label="Launch account">
            <select
              value={accountID}
              onChange={(event) => setAccountID(event.target.value)}
            >
              <option value="">Use the globally selected account</option>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.displayName}
                </option>
              ))}
            </select>
          </Field>
        </div>

        {error && <div className="inlineError">{error}</div>}

        <div className="modalActions">
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

  return (
    <Modal title={instance.name} onClose={onClose}>
      <div className="tabs">
        <button
          className={tab === "overview" ? "active" : ""}
          onClick={() => setTab("overview")}
        >
          Overview
        </button>
        <button
          className={tab === "mods" ? "active" : ""}
          onClick={() => setTab("mods")}
        >
          Mods <b>{mods.length}</b>
        </button>
        <button
          className={tab === "settings" ? "active" : ""}
          onClick={() => setTab("settings")}
        >
          Settings
        </button>
      </div>

      {tab === "overview" && (
        <div className="detailOverview">
          <div className="heroMark">W</div>
          <div>
            <StatusPill status={instance.status} />
            <h2>{instance.name}</h2>
            <p>{instance.description || "No description yet."}</p>
          </div>

          <dl>
            <div>
              <dt>Version</dt>
              <dd>
                {versions.find((version) => version.id === instance.gameVersionId)
                  ?.name ?? instance.gameVersionId}
              </dd>
            </div>
            <div>
              <dt>Mods</dt>
              <dd>
                {mods.filter((mod) => mod.enabled).length} of {mods.length}
              </dd>
            </div>
            <div>
              <dt>Playtime</dt>
              <dd>{formatDuration(instance.playtimeSeconds)}</dd>
            </div>
            <div>
              <dt>Data directory</dt>
              <dd className="path">{instance.directory}</dd>
            </div>
          </dl>

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
            Open data directory
          </Button>
        </div>
      )}

      {tab === "mods" && (
        <div>
          <div className="sectionActions">
            <p className="muted">Local .zip, .cs, and .dll files</p>
            <div className="row">
              <Button
                variant="secondary"
                onClick={() => navigate(`/mods?instanceId=${encodeURIComponent(instance.id)}`)}
              >
                Browse mods
              </Button>
              <Button onClick={() => void installMod()}>＋ Install file</Button>
            </div>
          </div>

          {mods.length === 0 ? (
            <Empty
              icon="◇"
              title="No mods installed"
              description="Install a local mod file for this instance."
            />
          ) : (
            <div className="list">
              {mods.map((mod) => (
                <div className="listItem" key={mod.id}>
                  <div>
                    <strong>{mod.name}</strong>
                    <small>
                      {mod.version} · {mod.fileName}
                    </small>
                  </div>
                  <div className="row">
                    <label className="switch">
                      <input
                        type="checkbox"
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
                      <span />
                    </label>
                    <Button
                      variant="ghost"
                      onClick={async () => {
                        if (!window.confirm(`Remove mod “${mod.name}”?`)) {
                          return;
                        }
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
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === "settings" && (
        <SubmitForm
          className="form detailForm"
          onSubmit={saveSettings}
        >
          <Field label="Name">
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </Field>

          <Field label="Description">
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </Field>

          <div className="formRow">
            <Field label="Game version">
              <select
                value={versionID}
                onChange={(event) => setVersionID(event.target.value)}
              >
                {versions.map((version) => (
                  <option key={version.id} value={version.id}>
                    {version.name}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Launch account">
              <select
                value={accountID}
                onChange={(event) => setAccountID(event.target.value)}
              >
                <option value="">Use the globally selected account</option>
                {accounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.displayName}
                  </option>
                ))}
              </select>
            </Field>
          </div>

          <Field
            label="Launch arguments"
            hint="Separate arguments with spaces. Waxlight always appends its isolated --dataPath."
          >
            <input
              value={argumentsText}
              onChange={(event) => setArgumentsText(event.target.value)}
              placeholder="--tracelog"
            />
          </Field>

          <div className="dangerZone">
            <div>
              <strong>Delete instance</strong>
              <small>This also removes its data directory.</small>
            </div>
            <Button
              type="button"
              variant="danger"
              onClick={() => void deleteInstance()}
            >
              Delete
            </Button>
          </div>

          <div className="modalActions">
            <Button busy={busy}>Save</Button>
          </div>
        </SubmitForm>
      )}
    </Modal>
  );
}
