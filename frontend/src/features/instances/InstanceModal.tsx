import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowUp,
  Boxes,
  Clock3,
  Download,
  Gamepad2,
  PackageOpen,
  Pin,
  Plus,
  UserRound,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import { useToastStore } from "../../app/stores/toast";
import type { Account } from "../../entities/account/model";
import type { GameVersion } from "../../entities/game-version/model";
import { instancesApi } from "../../entities/instance/api";
import type { Instance } from "../../entities/instance/model";
import { modCatalogApi, modsApi } from "../../entities/mod/api";
import type { InstalledMod, InstanceModUpdateReport, ModVersion } from "../../entities/mod/model";
import { settingsApi } from "../../entities/settings/api";
import { useOptimumStatusQuery, useSettingsQuery } from "../../entities/settings/queries";
import { errorMessage } from "../../shared/api/bridge";
import { INSTANCES_QUERY_KEY } from "../../shared/api/keys";
import type { ModUpdatePolicy } from "../../shared/api/types";
import { formatDuration } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { CoverArt } from "../../shared/ui/cover-art";
import { DialogFooter } from "../../shared/ui/dialog";
import { Empty } from "../../shared/ui/empty";
import { Field } from "../../shared/ui/field";
import { Input } from "../../shared/ui/input";
import { Modal } from "../../shared/ui/modal";
import { StatusPill } from "../../shared/ui/status-pill";
import { SubmitForm } from "../../shared/ui/submit-form";
import { Tabs } from "../../shared/ui/tabs";
import { BackupsTab } from "../instance/BackupsTab";
import { LastKnownGoodSection } from "../instance/LastKnownGoodSection";
import { ModUpdatesModal } from "../mods/ModUpdatesModal";

type InstanceTab = "overview" | "mods" | "settings" | "backups";

interface InstanceModalProps {
  instance: Instance;
  initialTab?: Extract<InstanceTab, "overview" | "settings">;
  versions: GameVersion[];
  accounts: Account[];
  onClose: () => void;
  onExport: () => void;
  onClone: () => void;
  onModUpdatesChanged?: (instanceID: string, report: InstanceModUpdateReport) => void;
}

function formatEnvironmentVariables(values: Record<string, string> | undefined): string {
  return Object.entries(values ?? {})
    .toSorted(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join("\n");
}

function parseEnvironmentVariables(text: string): {
  values: Record<string, string>;
  invalidLine?: number;
} {
  const values: Record<string, string> = {};
  const lines = text.split(/\r?\n/);
  for (let index = 0; index < lines.length; index += 1) {
    const rawLine = lines[index];
    if (rawLine.trim() === "") continue;
    const separator = rawLine.indexOf("=");
    if (separator <= 0) return { values: {}, invalidLine: index + 1 };
    const key = rawLine.slice(0, separator).trim();
    if (!key) return { values: {}, invalidLine: index + 1 };
    values[key] = rawLine.slice(separator + 1);
  }
  return { values };
}

function isModUpdatePolicy(value: string): value is ModUpdatePolicy {
  return ["automatic", "compatible_only", "pinned"].includes(value);
}

export function InstanceModal({
  instance,
  initialTab = "overview",
  versions,
  accounts,
  onClose,
  onExport,
  onClone,
  onModUpdatesChanged,
}: InstanceModalProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: settings } = useSettingsQuery();
  const { data: optimumStatus } = useOptimumStatusQuery();
  const [tab, setTab] = useState<InstanceTab>(initialTab);
  const [mods, setMods] = useState<InstalledMod[]>([]);
  const [versionsByModID, setVersionsByModID] = useState<Record<string, ModVersion[]>>({});
  const [loadingVersionModIDs, setLoadingVersionModIDs] = useState<Set<string>>(new Set());
  const [updatingModID, setUpdatingModID] = useState("");
  const [versionChangeError, setVersionChangeError] = useState<{
    modID: string;
    message: string;
  }>();
  const [name, setName] = useState(instance.name);
  const [description, setDescription] = useState(instance.description);
  const [versionID, setVersionID] = useState(instance.gameVersionId);
  const [gameClient, setGameClient] = useState(instance.gameClient ?? "vanilla");
  const [accountID, setAccountID] = useState(instance.defaultAccountId ?? "");
  const [argumentsText, setArgumentsText] = useState(instance.launchArguments.join(" "));
  const [environmentText, setEnvironmentText] = useState(
    formatEnvironmentVariables(instance.environmentVariables),
  );
  const [coverUrl, setCoverUrl] = useState(instance.coverUrl);
  const [coverSourcePath, setCoverSourcePath] = useState<string>();
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
      const report = await modsApi.checkInstanceUpdates(instance.id);
      setUpdateReport(report);
      return report;
    } catch (error) {
      notify(errorMessage(error), "error");
      return undefined;
    }
  }, [instance.id, notify]);

  const linkLocalMods = useCallback(async () => {
    try {
      const result = await modsApi.linkLocal(instance.id);
      await loadMods();
      const report = await loadUpdates();
      if (report) onModUpdatesChanged?.(instance.id, report);
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
  }, [instance.id, loadMods, loadUpdates, notify, onModUpdatesChanged, queryClient, t]);

  useEffect(() => {
    void loadMods();
    void loadUpdates();
    if (!autoLinkedRef.current) {
      autoLinkedRef.current = true;
      void linkLocalMods();
    }
  }, [loadMods, loadUpdates, linkLocalMods]);

  async function loadModVersions(mod: InstalledMod) {
    const [source, modID] = mod.source.split(":");
    if (
      source !== "moddb" ||
      !modID ||
      Object.hasOwn(versionsByModID, mod.id) ||
      loadingVersionModIDs.has(mod.id)
    ) {
      return;
    }
    setLoadingVersionModIDs((current) => new Set(current).add(mod.id));
    try {
      const details = await modCatalogApi.get(modID);
      setVersionsByModID((current) => ({ ...current, [mod.id]: details.versions }));
    } catch {
      setVersionsByModID((current) => ({ ...current, [mod.id]: [] }));
    } finally {
      setLoadingVersionModIDs((current) => {
        const next = new Set(current);
        next.delete(mod.id);
        return next;
      });
    }
  }

  async function changeModVersion(mod: InstalledMod, targetVersionID: string) {
    const [source, modID] = mod.source.split(":");
    if (source !== "moddb" || !modID) return;
    setUpdatingModID(mod.id);
    setVersionChangeError(undefined);
    try {
      await modsApi.updateInstance({
        instanceId: instance.id,
        mods: [{ modId: modID, versionId: targetVersionID }],
        allowIncompatible: false,
      });
      await loadMods();
      await loadUpdates();
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
    } catch (error) {
      setVersionChangeError({ modID: mod.id, message: errorMessage(error) });
    } finally {
      setUpdatingModID("");
    }
  }

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
      const parsedEnvironment = parseEnvironmentVariables(environmentText);
      if (parsedEnvironment.invalidLine !== undefined) {
        notify(
          t("environment_variables_invalid_line", { line: parsedEnvironment.invalidLine }),
          "error",
        );
        return;
      }
      const updated = await instancesApi.update({
        id: instance.id,
        name,
        description,
        gameVersionId: versionID,
        gameClient,
        defaultAccountId: accountID || undefined,
        launchArguments: argumentsText.trim() ? argumentsText.trim().split(/\s+/) : [],
        environmentVariables: parsedEnvironment.values,
        coverSourcePath,
      });
      setCoverUrl(updated.coverUrl);
      setCoverSourcePath(undefined);
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
  const settingsVersion = versions.find((version) => version.id === versionID);
  const selectedAccount = accounts.find((account) => account.id === instance.defaultAccountId);
  const optimumVersionMismatch =
    gameClient === "optimum" &&
    optimumStatus?.ready === true &&
    optimumStatus.gameVersion !== "" &&
    settingsVersion !== undefined &&
    optimumStatus.gameVersion !== settingsVersion.id;
  const settingsDirty =
    name !== instance.name ||
    description !== instance.description ||
    versionID !== instance.gameVersionId ||
    gameClient !== (instance.gameClient ?? "vanilla") ||
    accountID !== (instance.defaultAccountId ?? "") ||
    argumentsText !== instance.launchArguments.join(" ") ||
    environmentText !== formatEnvironmentVariables(instance.environmentVariables) ||
    coverSourcePath !== undefined;

  function resetSettings() {
    setName(instance.name);
    setDescription(instance.description);
    setVersionID(instance.gameVersionId);
    setGameClient(instance.gameClient ?? "vanilla");
    setAccountID(instance.defaultAccountId ?? "");
    setArgumentsText(instance.launchArguments.join(" "));
    setEnvironmentText(formatEnvironmentVariables(instance.environmentVariables));
    setCoverSourcePath(undefined);
  }

  return (
    <Modal title={instance.name} className="instanceDialog" onClose={onClose}>
      <Tabs
        className="shrink-0 overflow-x-auto border-b border-border-subtle bg-surface-1 px-6"
        label={t("instance_details")}
        value={tab}
        options={[
          {
            value: "overview",
            label: t("overview"),
            tabId: "instance-overview-tab",
            panelId: "instance-tab-panel",
          },
          {
            value: "mods",
            label: (
              <>
                {t("mods")} <b className="tabBadge">{mods.length}</b>
              </>
            ),
            tabId: "instance-mods-tab",
            panelId: "instance-tab-panel",
          },
          {
            value: "settings",
            label: t("settings"),
            tabId: "instance-settings-tab",
            panelId: "instance-tab-panel",
          },
          {
            value: "backups",
            label: t("backups"),
            tabId: "instance-backups-tab",
            panelId: "instance-tab-panel",
          },
        ]}
        onValueChange={setTab}
      />

      {tab === "overview" && (
        <div
          id="instance-tab-panel"
          className="instanceTabBody detailOverview"
          role="tabpanel"
          aria-labelledby="instance-overview-tab"
        >
          <section className="instanceHeroBanner">
            <CoverArt className="instanceHeroBannerArt" src={coverUrl} seed={instance.name} />
            <div className="instanceHeroBannerCopy">
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
              <div className="statTileIcon" aria-hidden="true">
                <Gamepad2 size={15} />
              </div>
              <span>{t("game_version")}</span>
              <strong title={selectedVersion?.name ?? instance.gameVersionId}>
                {selectedVersion?.name ?? instance.gameVersionId}
              </strong>
            </article>
            <article>
              <div className="statTileIcon" aria-hidden="true">
                <Boxes size={15} />
              </div>
              <span>{t("mods")}</span>
              <strong>
                {t("installed_count", { count: mods.filter((mod) => mod.enabled).length })}
              </strong>
              <small>{t("total_count", { count: mods.length })}</small>
            </article>
            <article>
              <div className="statTileIcon" aria-hidden="true">
                <Clock3 size={15} />
              </div>
              <span>{t("playtime")}</span>
              <strong>{formatDuration(instance.playtimeSeconds)}</strong>
            </article>
            <article>
              <div className="statTileIcon" aria-hidden="true">
                <UserRound size={15} />
              </div>
              <span>{t("launch_account")}</span>
              <strong title={selectedAccount?.displayName ?? t("global_default")}>
                {selectedAccount?.displayName ?? t("global_default")}
              </strong>
            </article>
          </section>

          <LastKnownGoodSection instanceId={instance.id} />

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
              <Button onClick={onExport}>
                <Download className="size-4" aria-hidden="true" /> {t("export_instance")}
              </Button>
            </div>
            <div className="storageCopy">
              <span>{t("data_directory")}</span>
              <code title={instance.directory}>{instance.directory}</code>
            </div>
          </section>
        </div>
      )}

      {tab === "mods" && (
        <div
          id="instance-tab-panel"
          className="instanceTabBody modsTab"
          role="tabpanel"
          aria-labelledby="instance-mods-tab"
        >
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
                <Plus className="size-4" aria-hidden="true" /> {t("install_files")}
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
                    <ArrowUp className="size-3" /> {updateReport.summary.updatesAvailable}
                  </span>
                )}
              </Button>
            </div>
          </header>

          {mods.length === 0 ? (
            <Empty
              icon={<Boxes size={24} aria-hidden="true" />}
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
              {mods.map((mod) => {
                const modVersions = (versionsByModID[mod.id] ?? []).filter(
                  (version) => version.version !== mod.version,
                );
                const catalogManaged = mod.source.startsWith("moddb:");
                const loadingVersions = loadingVersionModIDs.has(mod.id);
                return (
                  <article className="installedModRow" key={mod.id}>
                    <div className="modRowIcon" aria-hidden="true">
                      <PackageOpen className="size-5" />
                    </div>
                    <div className="modRowCopy">
                      <strong>
                        {mod.name}
                        {!mod.managed && (
                          <span className="modSourceBadge local">{t("local_mod")}</span>
                        )}
                        {mod.updatePolicy === "pinned" && (
                          <Pin
                            className="modPinnedIcon"
                            aria-label={t("mod_update_policy_pinned")}
                          />
                        )}
                      </strong>
                      <div className="installedModControls">
                        {catalogManaged ? (
                          <Select
                            value=""
                            disabled={updatingModID === mod.id}
                            onValueChange={(targetVersionID) =>
                              void changeModVersion(mod, targetVersionID)
                            }
                            onOpenChange={(open) => {
                              if (open) void loadModVersions(mod);
                            }}
                          >
                            <SelectTrigger
                              className="installedModVersion"
                              aria-label={t("update_to_version", { version: mod.name })}
                            >
                              <SelectValue
                                placeholder={t("version_value", { version: mod.version })}
                              />
                            </SelectTrigger>
                            <SelectContent>
                              {loadingVersions ? (
                                <SelectItem value="loading" disabled>
                                  {t("loading_mods")}
                                </SelectItem>
                              ) : modVersions.length > 0 ? (
                                modVersions.map((version) => (
                                  <SelectItem key={version.id} value={version.id}>
                                    {t("version_value", { version: version.version })}
                                  </SelectItem>
                                ))
                              ) : (
                                <SelectItem value="unavailable" disabled>
                                  {t("no_downloadable_mod_version")}
                                </SelectItem>
                              )}
                            </SelectContent>
                          </Select>
                        ) : (
                          <small>{t("version_value", { version: mod.version })}</small>
                        )}
                        {catalogManaged && (
                          <Select
                            value={mod.updatePolicy}
                            onValueChange={async (updatePolicy) => {
                              if (!isModUpdatePolicy(updatePolicy)) return;
                              try {
                                await modsApi.setUpdatePolicy(mod.id, updatePolicy);
                                await loadMods();
                                const report = await loadUpdates();
                                if (report) onModUpdatesChanged?.(instance.id, report);
                              } catch (error) {
                                notify(errorMessage(error), "error");
                              }
                            }}
                          >
                            <SelectTrigger
                              className="installedModVersion"
                              aria-label={t("mod_update_policy")}
                            >
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="automatic">
                                {t("mod_update_policy_automatic")}
                              </SelectItem>
                              <SelectItem value="compatible_only">
                                {t("mod_update_policy_compatible_only")}
                              </SelectItem>
                              <SelectItem value="pinned">
                                {t("mod_update_policy_pinned")}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        )}
                      </div>
                      {versionChangeError?.modID === mod.id && (
                        <p className="installedModVersionError" role="alert">
                          {versionChangeError.message}
                        </p>
                      )}
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
                      <Button variant="danger" onClick={() => void requestModRemoval(mod)}>
                        {t("remove")}
                      </Button>
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </div>
      )}

      {tab === "settings" && (
        <SubmitForm className="dialogForm" onSubmit={saveSettings}>
          <div
            id="instance-tab-panel"
            className="modalBody settingsBody"
            role="tabpanel"
            aria-labelledby="instance-settings-tab"
          >
            <section className="settingsSection">
              <header>
                <h3>{t("general")}</h3>
                <p>{t("instance_basic_information_description")}</p>
              </header>
              <div className="formFields">
                <Field label={t("name")}>
                  <Input value={name} onChange={(event) => setName(event.target.value)} />
                </Field>

                <Field label={t("description")}>
                  <textarea
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </Field>

                <div className="field">
                  <span>{t("instance_cover")}</span>
                  <div className="row">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={async () => {
                        try {
                          const path = await instancesApi.selectCover();
                          if (path) {
                            setCoverSourcePath(path);
                          }
                        } catch (error) {
                          notify(errorMessage(error), "error");
                        }
                      }}
                    >
                      {t("select")}
                    </Button>
                    {(coverUrl || (coverSourcePath !== undefined && coverSourcePath !== "")) && (
                      <Button type="button" variant="ghost" onClick={() => setCoverSourcePath("")}>
                        {t("remove")}
                      </Button>
                    )}
                  </div>
                </div>

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

                <Field label={t("game_client")} hint={t("game_client_description")}>
                  <Select
                    value={gameClient}
                    onValueChange={(client) => {
                      if (client === "vanilla" || client === "optimum") {
                        setGameClient(client);
                      }
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="vanilla">{t("vanilla")}</SelectItem>
                      <SelectItem value="optimum">Optimum</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>

                {gameClient === "optimum" && (
                  <>
                    {optimumVersionMismatch ? (
                      <div className="inlineNotice warning" role="alert">
                        <strong>{t("optimum_version_mismatch")}</strong>
                        <div>
                          {t("optimum_version_mismatch_description", {
                            optimumVersion: optimumStatus.gameVersion,
                            instanceVersion: settingsVersion.id,
                          })}
                        </div>
                      </div>
                    ) : (
                      <div>
                        <div className="settingRowText">
                          <strong>
                            {optimumStatus?.ready ? t("optimum_ready") : t("optimum_missing")}
                          </strong>
                          <small className="settingRowDescription">
                            {optimumStatus?.ready
                              ? optimumStatus.gameVersion
                                ? t("optimum_vintage_story_version", {
                                    version: optimumStatus.gameVersion,
                                  })
                                : optimumStatus.path
                              : optimumStatus?.message || t("optimum_not_configured_description")}
                          </small>
                        </div>
                        {!optimumStatus?.ready && (
                          <Button
                            type="button"
                            variant="secondary"
                            onClick={() => {
                              onClose();
                              void navigate("/settings");
                            }}
                          >
                            {t("configure_optimum")}
                          </Button>
                        )}
                      </div>
                    )}
                  </>
                )}
              </div>
            </section>

            <section className="settingsSection">
              <header>
                <h3>{t("advanced")}</h3>
                <p>{t("advanced_launch_configuration")}</p>
              </header>
              <Field label={t("launch_arguments")} hint={t("launch_arguments_hint")}>
                <Input
                  className="codeInput"
                  value={argumentsText}
                  onChange={(event) => setArgumentsText(event.target.value)}
                  placeholder="--tracelog"
                />
              </Field>
              <Field label={t("environment_variables")} hint={t("environment_variables_hint")}>
                <textarea
                  className="codeInput"
                  value={environmentText}
                  onChange={(event) => setEnvironmentText(event.target.value)}
                  placeholder={"KEY=value"}
                  rows={4}
                  spellCheck={false}
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

          <DialogFooter className="settingsFooter">
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
              <Button type="submit" busy={busy} disabled={!settingsDirty}>
                {t("save_changes")}
              </Button>
            </div>
          </DialogFooter>
        </SubmitForm>
      )}

      {tab === "backups" && (
        <div id="instance-tab-panel" role="tabpanel" aria-labelledby="instance-backups-tab">
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
        </div>
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
          <DialogFooter>
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
          </DialogFooter>
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
            const report = await loadUpdates();
            if (report) onModUpdatesChanged?.(instance.id, report);
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
          }}
        />
      )}
    </Modal>
  );
}
