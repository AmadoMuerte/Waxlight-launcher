import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import { useToastStore } from "../../app/stores/toast";
import type { Account } from "../../entities/account/model";
import type { GameVersion } from "../../entities/game-version/model";
import { instancesApi } from "../../entities/instance/api";
import type { Instance } from "../../entities/instance/model";
import { modsApi } from "../../entities/mod/api";
import type { InstalledMod, InstanceModUpdateReport } from "../../entities/mod/model";
import { settingsApi } from "../../entities/settings/api";
import { useSettingsQuery } from "../../entities/settings/queries";
import { errorMessage } from "../../shared/api/bridge";
import { INSTANCES_QUERY_KEY } from "../../shared/api/keys";
import { formatDuration } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { Field } from "../../shared/ui/field";
import { Modal } from "../../shared/ui/modal";
import { StatusPill } from "../../shared/ui/status-pill";
import { SubmitForm } from "../../shared/ui/submit-form";
import { BackupsTab } from "../instance/BackupsTab";
import { ModUpdatesModal } from "../mods/ModUpdatesModal";

type InstanceTab = "overview" | "mods" | "settings" | "backups";

interface InstanceModalProps {
  instance: Instance;
  versions: GameVersion[];
  accounts: Account[];
  onClose: () => void;
  onExport: () => void;
  onClone: () => void;
}

export function InstanceModal({
  instance,
  versions,
  accounts,
  onClose,
  onExport,
  onClone,
}: InstanceModalProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: settings } = useSettingsQuery();
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
  const [removeModDepsDialog, setRemoveModDepsDialog] = useState<{
    open: boolean;
    modId: string;
    modName: string;
    dependencies: InstalledMod[];
  }>({ open: false, modId: "", modName: "", dependencies: [] });
  async function removeMod(modId: string) {
    await modsApi.remove(modId, false);
    await loadMods();
    await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
  }

  async function removeModWithDependencies(modId: string) {
    await modsApi.remove(modId, true);
    await loadMods();
    await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
  }

  async function requestModRemoval(mod: InstalledMod) {
    try {
      const preview = await modsApi.previewDelete(mod.id);
      if ((preview.dependencies?.length ?? 0) === 0) {
        if (settings?.confirmDeletion === false) {
          try {
            await removeMod(mod.id);
          } catch (error) {
            notify(errorMessage(error), "error");
          }
          return;
        }
        setRemoveModConfirm({
          open: true,
          title: t("remove_mod_confirmation", { name: mod.name }),
          onConfirm: async () => {
            try {
              await removeMod(mod.id);
            } catch (error) {
              notify(errorMessage(error), "error");
            }
          },
        });
        return;
      }
      setRemoveModDepsDialog({
        open: true,
        modId: mod.id,
        modName: mod.name,
        dependencies: preview.dependencies,
      });
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  function closeRemoveModDepsDialog() {
    setRemoveModDepsDialog((state) => ({ ...state, open: false }));
  }

  async function confirmRemoveOnly() {
    const modId = removeModDepsDialog.modId;
    closeRemoveModDepsDialog();
    try {
      await removeMod(modId);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function confirmRemoveWithDependencies() {
    const modId = removeModDepsDialog.modId;
    closeRemoveModDepsDialog();
    try {
      await removeModWithDependencies(modId);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  const [updateReport, setUpdateReport] = useState<InstanceModUpdateReport>();
  const [updatesDialogOpen, setUpdatesDialogOpen] = useState(false);
  const autoLinkedRef = useRef(false);

  const loadMods = useCallback(async () => {
    try {
      setMods((await modsApi.list(instance.id)) ?? []);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }, [instance.id, notify]);

  const loadUpdates = useCallback(async () => {
    try {
      setUpdateReport(await modsApi.checkInstanceUpdates(instance.id));
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }, [instance.id, notify]);

  const linkLocalMods = useCallback(async () => {
    try {
      const result = await modsApi.linkLocal(instance.id);
      await loadMods();
      await loadUpdates();
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
      if (result.linked.length > 0) {
        notify(t("mods_linked_count", { count: result.linked.length }));
      }
      if (result.notMatched.length > 0) {
        notify(t("mods_not_matched_count", { count: result.notMatched.length }));
      }
    } catch {
      // Recognition is best effort; the mods list still loads without it.
    }
  }, [instance.id, loadMods, loadUpdates, notify, queryClient, t]);

  useEffect(() => {
    void loadMods();
    void loadUpdates();
    if (!autoLinkedRef.current) {
      autoLinkedRef.current = true;
      void linkLocalMods();
    }
  }, [loadMods, loadUpdates, linkLocalMods]);

  async function installMods() {
    try {
      const paths = await settingsApi.selectModFiles();
      if (!paths || paths.length === 0) {
        return;
      }
      const result = await modsApi.installMany({
        instanceId: instance.id,
        sourcePaths: paths,
      });
      await loadMods();
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
      const segments: string[] = [];
      if (result.installed.length > 0) {
        segments.push(t("mods_installed_count", { count: result.installed.length }));
      }
      if (result.skipped.length > 0) {
        segments.push(t("mods_skipped_count", { count: result.skipped.length }));
      }
      if (result.failed.length > 0) {
        segments.push(t("mods_failed_count", { count: result.failed.length }));
      }
      notify(segments.join(" · ") || t("mod_installed"));
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
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
      notify(t("instance_settings_saved"));
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setBusy(false);
    }
  }

  async function deleteInstance() {
    if (settings?.confirmDeletion === false) {
      try {
        await instancesApi.remove(instance.id, true);
        onClose();
        await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
        notify(t("instance_deleted"));
      } catch (error) {
        notify(errorMessage(error), "error");
      }
      return;
    }
    setDeleteConfirm({
      open: true,
      title: t("delete_instance_confirmation", { name: instance.name }),
      onConfirm: async () => {
        try {
          await instancesApi.remove(instance.id, true);
          onClose();
          await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
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
        <button
          type="button"
          role="tab"
          aria-selected={tab === "backups"}
          className={tab === "backups" ? "active" : ""}
          onClick={() => setTab("backups")}
        >
          {t("backups")}
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
            <div className="storageToolbar">
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
              <Button variant="secondary" onClick={onClone}>
                {t("clone_instance")}
              </Button>
              <Button onClick={onExport}>⤓ {t("export_instance")}</Button>
            </div>
            <div className="storageCopy">
              <span>{t("data_directory")}</span>
              <code title={instance.directory}>{instance.directory}</code>
            </div>
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
              <Button
                variant="secondary"
                onClick={() => navigate(`/mods?instanceId=${encodeURIComponent(instance.id)}`)}
              >
                {t("browse_mods")}
              </Button>
              <Button onClick={() => void installMods()}>
                <span aria-hidden="true">＋</span> {t("install_files")}
              </Button>
              <Button
                variant="secondary"
                disabled={!updateReport}
                title={
                  updateReport && updateReport.summary.updatesAvailable === 0
                    ? t("no_mod_updates")
                    : undefined
                }
                onClick={() => setUpdatesDialogOpen(true)}
              >
                {t("update_mods")}
                {updateReport && updateReport.summary.updatesAvailable > 0 && (
                  <span className="updateCountBadge" aria-hidden="true">
                    ▲ {updateReport.summary.updatesAvailable}
                  </span>
                )}
              </Button>
            </div>
          </header>

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
                  <Button onClick={() => void installMods()}>{t("install_files")}</Button>
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
                    <strong>
                      {mod.name}
                      {mod.managed ? (
                        <span className="modSourceBadge managed">{t("managed_mod")}</span>
                      ) : (
                        <span className="modSourceBadge local">{t("local_mod")}</span>
                      )}
                    </strong>
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
                          await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
                        } catch (error) {
                          notify(errorMessage(error), "error");
                        }
                      }}
                    />
                    <Button
                      variant="ghost"
                      className="dangerGhost"
                      onClick={() => void requestModRemoval(mod)}
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

      {tab === "backups" && (
        <BackupsTab
          instanceId={instance.id}
          onCreated={() => {
            onClose();
            void navigate("/operations");
          }}
          onRestored={() => {
            void loadMods();
            void loadUpdates();
          }}
        />
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

      {removeModDepsDialog.open && (
        <Modal
          title={t("remove_mod_dependencies_title", { name: removeModDepsDialog.modName })}
          onClose={closeRemoveModDepsDialog}
        >
          <div className="modalBody formFields">
            <p className="muted">
              {t("remove_mod_dependencies_hint", { name: removeModDepsDialog.modName })}
            </p>
            <ul className="removeDependenciesList">
              {removeModDepsDialog.dependencies.map((dependency) => (
                <li key={dependency.id}>
                  <strong>{dependency.name}</strong>
                  <small>{dependency.version}</small>
                </li>
              ))}
            </ul>
          </div>
          <div className="dialogFooter">
            <Button type="button" variant="ghost" onClick={closeRemoveModDepsDialog}>
              {t("cancel")}
            </Button>
            <Button type="button" variant="secondary" onClick={() => void confirmRemoveOnly()}>
              {t("remove_mod_only")}
            </Button>
            <Button
              type="button"
              variant="danger"
              onClick={() => void confirmRemoveWithDependencies()}
            >
              {t("remove_mod_with_dependencies")}
            </Button>
          </div>
        </Modal>
      )}

      {updatesDialogOpen && updateReport && (
        <ModUpdatesModal
          instanceId={instance.id}
          instanceName={instance.name}
          report={updateReport}
          onClose={() => setUpdatesDialogOpen(false)}
          onApplied={async () => {
            await loadMods();
            await loadUpdates();
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
          }}
        />
      )}
    </Modal>
  );
}
