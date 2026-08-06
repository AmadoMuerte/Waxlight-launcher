import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { ConfirmDialog } from "../components/ui/confirm-dialog";
import { ModUpdatesModal } from "../features/mods/ModUpdatesModal";
import {
  instancesApi,
  launcherApi,
  modsApi,
  settingsApi,
  type Account,
  type GameVersion,
  type InstanceModUpdateReport,
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
  const { t } = useTranslation();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedInstance, setSelectedInstance] = useState<Instance>();
  const [query, setQuery] = useState("");
  const [modUpdates, setModUpdates] = useState<Record<string, InstanceModUpdateReport>>({});
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const instancesRef = useRef(instances);
  const checkedOnceRef = useRef(false);
  const navigate = useNavigate();

  useEffect(() => {
    instancesRef.current = instances;
  }, [instances]);

  const checkAllUpdates = useCallback(async () => {
    const current = instancesRef.current;
    if (current.length === 0) {
      return;
    }
    setCheckingUpdates(true);
    const entries = await Promise.all(
      current.map(async (instance) => {
        try {
          const report = await modsApi.checkInstanceUpdates(instance.id);
          return [instance.id, report] as const;
        } catch {
          return undefined;
        }
      }),
    );
    const collected: Record<string, InstanceModUpdateReport> = {};
    for (const entry of entries) {
      if (entry) {
        collected[entry[0]] = entry[1];
      }
    }
    setModUpdates(collected);
    setCheckingUpdates(false);
  }, []);

  useEffect(() => {
    if (checkedOnceRef.current) {
      return;
    }
    checkedOnceRef.current = true;
    void checkAllUpdates();
  }, [checkAllUpdates]);

  const visibleInstances = useMemo(
    () => instances.filter((instance) => instance.name.toLowerCase().includes(query.toLowerCase())),
    [instances, query],
  );

  const [launchWarnConfirm, setLaunchWarnConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  async function launch(instance: Instance) {
    try {
      const validation = await launcherApi.validate(instance.id, instance.defaultAccountId);
      const issues = validation?.issues ?? [];
      const warnings = validation?.warnings ?? [];

      if (!validation?.valid) {
        throw new Error(issues.join(". ") || t("instance_cannot_launch"));
      }
      if (warnings.length > 0) {
        setLaunchWarnConfirm({
          open: true,
          title: t("launch_anyway"),
          message: warnings.join("\n"),
          onConfirm: async () => {
            await launcherApi.launch(instance.id, instance.defaultAccountId);
            notify(t("started_instance", { name: instance.name }));
            await refresh();
          },
        });
        return;
      }

      await launcherApi.launch(instance.id, instance.defaultAccountId);
      notify(t("started_instance", { name: instance.name }));
      await refresh();
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow={t("your_worlds")}
        title={t("library")}
        description={t("library_description")}
        action={
          versions.length === 0 ? undefined : (
            <Button onClick={() => setCreateDialogOpen(true)}>＋ {t("new_instance")}</Button>
          )
        }
      />

      {instances.length > 0 && (
        <div className="toolbar">
          <div className="search">
            <span>⌕</span>
            <input
              aria-label={t("search_instances")}
              placeholder={t("find_instance_placeholder")}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <span className="muted">{t("instances_count", { count: visibleInstances.length })}</span>
          <Button variant="secondary" busy={checkingUpdates} onClick={() => void checkAllUpdates()}>
            {t("check_mod_updates")}
          </Button>
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
          title={query ? t("nothing_found") : t("light_your_first_world")}
          description={
            query ? t("try_another_instance_name") : t("create_first_instance_description")
          }
          action={
            !query && (
              <Button
                onClick={
                  versions.length > 0
                    ? () => setCreateDialogOpen(true)
                    : () => navigate("/versions")
                }
              >
                {versions.length > 0 ? t("create_instance") : t("install_game_version_first")}
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
              version={versions.find((version) => version.id === instance.gameVersionId)}
              updateCount={modUpdates[instance.id]?.summary.updatesAvailable ?? 0}
              onOpen={() => setSelectedInstance(instance)}
              onLaunch={() => void launch(instance)}
              onStop={async () => {
                try {
                  await launcherApi.stop(instance.id);
                  notify(t("stop_signal_sent"));
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
            notify(t("instance_created"));
          }}
        />
      )}

      {selectedInstance && (
        <InstanceModal
          instance={instances.find((item) => item.id === selectedInstance.id) ?? selectedInstance}
          versions={versions}
          accounts={accounts}
          onClose={() => setSelectedInstance(undefined)}
          refresh={refresh}
          notify={notify}
        />
      )}

      <ConfirmDialog
        open={launchWarnConfirm.open}
        title={launchWarnConfirm.title}
        message={launchWarnConfirm.message}
        onConfirm={() => {
          setLaunchWarnConfirm((s) => ({ ...s, open: false }));
          launchWarnConfirm.onConfirm();
        }}
        onCancel={() => setLaunchWarnConfirm((s) => ({ ...s, open: false }))}
      />
    </>
  );
}

interface InstanceCardProps {
  instance: Instance;
  version?: GameVersion;
  updateCount?: number;
  onOpen: () => void;
  onLaunch: () => void;
  onStop: () => Promise<void>;
}

function InstanceCard({
  instance,
  version,
  updateCount = 0,
  onOpen,
  onLaunch,
  onStop,
}: InstanceCardProps) {
  const { t } = useTranslation();
  return (
    <article className="instanceCard" style={{ position: "relative" }}>
      <button
        type="button"
        aria-label={t("open_instance_details")}
        onClick={onOpen}
        style={{ position: "absolute", zIndex: 1, inset: 0, border: 0, background: "transparent" }}
      />
      <div className="cover">
        <span className="coverLetter">W</span>
        <div className="coverGlow" />
        <StatusPill status={instance.status} />
        {updateCount > 0 && (
          <span
            className="modUpdatesBadge"
            title={t("mod_updates_available", { count: updateCount })}
          >
            ▲ {updateCount}
          </span>
        )}
      </div>

      <div className="cardBody">
        <div className="cardTitle">
          <h3>{instance.name}</h3>
          <button
            type="button"
            className="more"
            aria-label={t("open_instance_details")}
            onClick={onOpen}
            style={{ position: "relative", zIndex: 2 }}
          >
            •••
          </button>
        </div>

        <p>{instance.description || t("instance_default_description")}</p>

        <div className="meta">
          <span>◈ {version?.name ?? instance.gameVersionId}</span>
          <span>◇ {t("mods_count", { count: instance.enabledModCount })}</span>
          <span>◷ {formatDuration(instance.playtimeSeconds)}</span>
        </div>

        <div className="cardFooter">
          <span>{formatDate(instance.lastPlayedAt)}</span>
          {instance.status === "running" ? (
            <Button
              variant="danger"
              style={{ position: "relative", zIndex: 2 }}
              onClick={(event) => {
                event.stopPropagation();
                void onStop();
              }}
            >
              {t("stop")}
            </Button>
          ) : (
            <Button
              style={{ position: "relative", zIndex: 2 }}
              onClick={(event) => {
                event.stopPropagation();
                onLaunch();
              }}
            >
              ▶ {t("play")}
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

function CreateInstanceModal({ versions, accounts, onClose, onDone }: CreateInstanceModalProps) {
  const { t } = useTranslation();
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
    <Modal title={t("new_instance")} className="createInstanceDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={createInstance}>
        <div className="modalBody formFields">
          <Field label={t("name")}>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("instance_name_example")}
            />
          </Field>

          <Field label={t("description")}>
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={t("instance_description_prompt")}
            />
          </Field>

          <div className="formRow">
            <Field label={t("game_version")}>
              <Select value={versionID} onValueChange={setVersionID}>
                <SelectTrigger>
                  <SelectValue placeholder={t("game_version")} />
                </SelectTrigger>
                <SelectContent>
                  {versions.map((version) => (
                    <SelectItem key={version.id} value={version.id}>
                      {version.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>

            <Field label={t("launch_account")}>
              <Select
                value={accountID ? `account:${accountID}` : "global"}
                onValueChange={(value) =>
                  setAccountID(value === "global" ? "" : value.slice("account:".length))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="global">{t("use_globally_selected_account")}</SelectItem>
                  {accounts.map((account) => (
                    <SelectItem key={account.id} value={`account:${account.id}`}>
                      {account.displayName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>

          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
        </div>

        <div className="dialogFooter">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button busy={busy} disabled={versions.length === 0}>
            {t("create")}
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
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [tab, setTab] = useState<InstanceTab>("overview");
  const [mods, setMods] = useState<InstalledMod[]>([]);
  const [name, setName] = useState(instance.name);
  const [description, setDescription] = useState(instance.description);
  const [versionID, setVersionID] = useState(instance.gameVersionId);
  const [accountID, setAccountID] = useState(instance.defaultAccountId ?? "");
  const [argumentsText, setArgumentsText] = useState(instance.launchArguments.join(" "));
  const [busy, setBusy] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });
  const [removeModConfirm, setRemoveModConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });
  const [updateReport, setUpdateReport] = useState<InstanceModUpdateReport>();
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [updatesDialogOpen, setUpdatesDialogOpen] = useState(false);

  const loadMods = useCallback(async () => {
    try {
      setMods((await modsApi.list(instance.id)) ?? []);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }, [instance.id, notify]);

  const loadUpdates = useCallback(async () => {
    setCheckingUpdates(true);
    try {
      setUpdateReport(await modsApi.checkInstanceUpdates(instance.id));
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setCheckingUpdates(false);
    }
  }, [instance.id, notify]);

  useEffect(() => {
    void loadMods();
    void loadUpdates();
  }, [loadMods, loadUpdates]);

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
      notify(t("mod_installed"));
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
        launchArguments: argumentsText.trim() ? argumentsText.trim().split(/\s+/) : [],
      });
      await refresh();
      notify(t("instance_settings_saved"));
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setBusy(false);
    }
  }

  async function deleteInstance() {
    setDeleteConfirm({
      open: true,
      title: t("delete_instance_confirmation", { name: instance.name }),
      onConfirm: async () => {
        try {
          await instancesApi.remove(instance.id, true);
          onClose();
          await refresh();
          notify(t("instance_deleted"));
        } catch (error) {
          notify(errorMessage(error), "error");
        }
      },
    });
  }

  const selectedVersion = versions.find((version) => version.id === instance.gameVersionId);
  const selectedAccount = accounts.find((account) => account.id === instance.defaultAccountId);
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
      <div className="tabs" role="tablist" aria-label={t("instance_details")}>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "overview"}
          className={tab === "overview" ? "active" : ""}
          onClick={() => setTab("overview")}
        >
          {t("overview")}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "mods"}
          className={tab === "mods" ? "active" : ""}
          onClick={() => setTab("mods")}
        >
          {t("mods")} <b className="tabBadge">{mods.length}</b>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "settings"}
          className={tab === "settings" ? "active" : ""}
          onClick={() => setTab("settings")}
        >
          {t("settings")}
        </button>
      </div>

      {tab === "overview" && (
        <div className="instanceTabBody detailOverview" role="tabpanel">
          <section className="instanceHero">
            <div className="heroMark" aria-hidden="true">
              W
            </div>
            <div className="instanceHeroCopy">
              <div className="instanceHeroTitle">
                <h2 title={instance.name}>{instance.name}</h2>
                <StatusPill status={instance.status} />
              </div>
              <p className={instance.description ? "" : "placeholderCopy"}>
                {instance.description || t("no_description_yet")}
              </p>
            </div>
          </section>

          <section className="instanceStats" aria-label={t("instance_statistics")}>
            <article>
              <span>{t("game_version")}</span>
              <strong>{selectedVersion?.name ?? instance.gameVersionId}</strong>
            </article>
            <article>
              <span>{t("mods")}</span>
              <strong>
                {t("installed_count", { count: mods.filter((mod) => mod.enabled).length })}
              </strong>
              <small>{t("total_count", { count: mods.length })}</small>
            </article>
            <article>
              <span>{t("playtime")}</span>
              <strong>{formatDuration(instance.playtimeSeconds)}</strong>
            </article>
            <article>
              <span>{t("launch_account")}</span>
              <strong>{selectedAccount?.displayName ?? t("global_default")}</strong>
            </article>
          </section>

          <section className="storageSection">
            <div className="storageCopy">
              <span>{t("data_directory")}</span>
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
              {t("open_directory")}
            </Button>
          </section>
        </div>
      )}

      {tab === "mods" && (
        <div className="instanceTabBody modsTab" role="tabpanel">
          <header className="instanceToolbar">
            <div>
              <h3>{t("mods")}</h3>
              <p>{t("manage_instance_mods")}</p>
            </div>
            <div className="row">
              <Button variant="secondary" busy={checkingUpdates} onClick={() => void loadUpdates()}>
                {t("check_mod_updates")}
              </Button>
              <Button
                variant="secondary"
                onClick={() => navigate(`/mods?instanceId=${encodeURIComponent(instance.id)}`)}
              >
                {t("browse_mods")}
              </Button>
              <Button onClick={() => void installMod()}>
                <span aria-hidden="true">＋</span> {t("install_file")}
              </Button>
            </div>
          </header>

          {updateReport && updateReport.summary.updatesAvailable > 0 && (
            <div className="modUpdatesBanner">
              <div>
                <strong>
                  {t("mod_updates_available", { count: updateReport.summary.updatesAvailable })}
                </strong>
                <p>{t("mod_updates_banner_description")}</p>
              </div>
              <Button onClick={() => setUpdatesDialogOpen(true)}>{t("update_mods")}</Button>
            </div>
          )}

          {mods.length === 0 ? (
            <Empty
              icon="◇"
              title={t("no_mods_installed")}
              description={t("browse_or_install_mod_file")}
              action={
                <div className="row">
                  <Button
                    variant="secondary"
                    onClick={() => navigate(`/mods?instanceId=${encodeURIComponent(instance.id)}`)}
                  >
                    {t("browse_mods")}
                  </Button>
                  <Button onClick={() => void installMod()}>{t("install_file")}</Button>
                </div>
              }
            />
          ) : (
            <div className="installedModList">
              {mods.map((mod) => (
                <article className="installedModRow" key={mod.id}>
                  <div className="modRowIcon" aria-hidden="true">
                    ◇
                  </div>
                  <div className="modRowCopy">
                    <strong>{mod.name}</strong>
                    <small>{t("version_value", { version: mod.version })}</small>
                    <code title={mod.fileName}>{mod.fileName}</code>
                  </div>
                  <div className="modRowActions">
                    <Checkbox
                      label={t("enabled")}
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
                      onClick={() => {
                        setRemoveModConfirm({
                          open: true,
                          title: t("remove_mod_confirmation", { name: mod.name }),
                          onConfirm: async () => {
                            try {
                              await modsApi.remove(mod.id);
                              await loadMods();
                              await refresh();
                            } catch (error) {
                              notify(errorMessage(error), "error");
                            }
                          },
                        });
                      }}
                    >
                      {t("remove")}
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
                <h3>{t("general")}</h3>
                <p>{t("instance_basic_information_description")}</p>
              </header>
              <div className="formFields">
                <Field label={t("name")}>
                  <input value={name} onChange={(event) => setName(event.target.value)} />
                </Field>

                <Field label={t("description")}>
                  <textarea
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </Field>

                <div className="formRow">
                  <Field label={t("game_version")}>
                    <Select value={versionID} onValueChange={setVersionID}>
                      <SelectTrigger>
                        <SelectValue placeholder={t("game_version")} />
                      </SelectTrigger>
                      <SelectContent>
                        {versions.map((version) => (
                          <SelectItem key={version.id} value={version.id}>
                            {version.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>

                  <Field label={t("launch_account")}>
                    <Select
                      value={accountID ? `account:${accountID}` : "global"}
                      onValueChange={(value) =>
                        setAccountID(value === "global" ? "" : value.slice("account:".length))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="global">{t("use_globally_selected_account")}</SelectItem>
                        {accounts.map((account) => (
                          <SelectItem key={account.id} value={`account:${account.id}`}>
                            {account.displayName}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                </div>
              </div>
            </section>

            <section className="settingsSection advancedSection">
              <header>
                <h3>{t("advanced")}</h3>
                <p>{t("optional_vintage_story_launch_arguments")}</p>
              </header>
              <Field label={t("launch_arguments")} hint={t("launch_arguments_hint")}>
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
                <h3>{t("danger_zone")}</h3>
                <p>{t("instance_irreversible_actions")}</p>
              </header>
              <div className="dangerZone">
                <div>
                  <strong>{t("delete_instance")}</strong>
                  <small>{t("instance_permanent_removal_warning")}</small>
                </div>
                <Button type="button" variant="danger" onClick={() => void deleteInstance()}>
                  {t("delete_instance")}
                </Button>
              </div>
            </section>
          </div>

          <div className="settingsFooter">
            <span className={settingsDirty ? "unsavedStatus active" : "unsavedStatus"}>
              {settingsDirty ? t("unsaved_changes") : t("all_changes_saved")}
            </span>
            <div className="row">
              <Button
                type="button"
                variant="ghost"
                disabled={!settingsDirty || busy}
                onClick={resetSettings}
              >
                {t("reset")}
              </Button>
              <Button busy={busy} disabled={!settingsDirty}>
                {t("save_changes")}
              </Button>
            </div>
          </div>
        </SubmitForm>
      )}

      <ConfirmDialog
        open={deleteConfirm.open}
        title={deleteConfirm.title}
        message={deleteConfirm.message}
        destructive
        onConfirm={() => {
          setDeleteConfirm((s) => ({ ...s, open: false }));
          deleteConfirm.onConfirm();
        }}
        onCancel={() => setDeleteConfirm((s) => ({ ...s, open: false }))}
      />

      <ConfirmDialog
        open={removeModConfirm.open}
        title={removeModConfirm.title}
        message={removeModConfirm.message}
        destructive
        onConfirm={() => {
          setRemoveModConfirm((s) => ({ ...s, open: false }));
          removeModConfirm.onConfirm();
        }}
        onCancel={() => setRemoveModConfirm((s) => ({ ...s, open: false }))}
      />

      {updatesDialogOpen && updateReport && (
        <ModUpdatesModal
          instanceId={instance.id}
          instanceName={instance.name}
          report={updateReport}
          onClose={() => setUpdatesDialogOpen(false)}
          onApplied={async () => {
            await loadMods();
            await loadUpdates();
            await refresh();
          }}
          notify={notify}
        />
      )}
    </Modal>
  );
}
