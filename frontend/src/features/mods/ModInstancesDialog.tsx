import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { errorMessage } from "../../shared/api/bridge";
import { modsApi } from "../../shared/api/mods";
import type {
  DownloadedMod,
  GameVersion,
  Instance,
  InstalledMod,
  InstalledModInstance,
} from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { Modal } from "../../shared/ui/modal";
import { SearchInput } from "../../shared/ui/search-input";
import { compatibilityFor, compatibilityLabel, instanceGameVersion } from "./lib";

interface ModInstancesDialogProps {
  mod: DownloadedMod;
  instances: Instance[];
  gameVersions: GameVersion[];
  confirmDeletion: boolean;
  onClose: () => void;
  onInstallRequest: (mod: DownloadedMod, instance: Instance) => void;
  onMutated: () => Promise<void> | void;
}

export function ModInstancesDialog({
  mod,
  instances,
  gameVersions,
  confirmDeletion,
  onClose,
  onInstallRequest,
  onMutated,
}: ModInstancesDialogProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [installed, setInstalled] = useState<Map<string, InstalledMod>>(new Map());
  const [errors, setErrors] = useState<Map<string, string>>(new Map());
  const [busy, setBusy] = useState<Set<string>>(new Set());
  const [removeConfirm, setRemoveConfirm] = useState<InstalledMod>();
  const [removeDependencies, setRemoveDependencies] = useState<{
    mod: InstalledMod;
    dependencies: InstalledMod[];
  }>();
  const busyRows = useRef(new Set<string>());
  const rowVersions = useRef(new Map<string, number>());
  const source = `moddb:${mod.modId}:${mod.versionId}`;
  const release = useMemo(
    () => ({
      id: mod.versionId,
      version: mod.downloadedVersion,
      gameVersions: mod.gameVersions,
      releaseType: "stable" as const,
      fileName: mod.fileName,
      fileSize: mod.fileSize,
    }),
    [mod.downloadedVersion, mod.fileName, mod.fileSize, mod.gameVersions, mod.versionId],
  );
  const visibleInstances = useMemo(
    () =>
      instances
        .filter((instance) => instance.name.toLowerCase().includes(query.toLowerCase()))
        .toSorted(
          (left, right) => Number(installed.has(right.id)) - Number(installed.has(left.id)),
        ),
    [installed, instances, query],
  );
  const currentMod = useMemo<DownloadedMod>(
    () => ({
      ...mod,
      installedInstances: instances.flatMap((instance): InstalledModInstance[] => {
        const installedMod = installed.get(instance.id);
        return installedMod
          ? [
              {
                instanceId: instance.id,
                instanceName: instance.name,
                version: installedMod.version,
                enabled: installedMod.enabled,
              },
            ]
          : [];
      }),
    }),
    [installed, instances, mod],
  );

  useEffect(() => {
    let active = true;
    const versionsAtLoad = new Map(rowVersions.current);
    void Promise.all(
      instances.map(async (instance) => {
        try {
          const mods = await modsApi.list(instance.id);
          return [instance.id, mods.find((item) => item.source === source)] as const;
        } catch (error) {
          return [instance.id, undefined, errorMessage(error)] as const;
        }
      }),
    ).then((results) => {
      if (!active) return undefined;
      setInstalled((current) => {
        const next = new Map(current);
        for (const [instanceId, installedMod] of results) {
          if (rowVersions.current.get(instanceId) !== versionsAtLoad.get(instanceId)) continue;
          if (installedMod) next.set(instanceId, installedMod);
          else next.delete(instanceId);
        }
        return next;
      });
      setErrors((current) => {
        const next = new Map(current);
        for (const [instanceId, , error] of results) {
          if (rowVersions.current.get(instanceId) !== versionsAtLoad.get(instanceId)) continue;
          if (error) next.set(instanceId, error);
          else next.delete(instanceId);
        }
        return next;
      });
      return undefined;
    });
    return () => {
      active = false;
    };
  }, [instances, source]);

  function setRowBusy(instanceId: string, value: boolean) {
    setBusy((current) => {
      const next = new Set(current);
      if (value) next.add(instanceId);
      else next.delete(instanceId);
      return next;
    });
  }

  function beginRowMutation(instanceId: string) {
    if (busyRows.current.has(instanceId)) return false;
    busyRows.current.add(instanceId);
    rowVersions.current.set(instanceId, (rowVersions.current.get(instanceId) ?? 0) + 1);
    setRowBusy(instanceId, true);
    return true;
  }

  function endRowMutation(instanceId: string) {
    busyRows.current.delete(instanceId);
    setRowBusy(instanceId, false);
  }

  function setRowError(instanceId: string, message?: string) {
    setErrors((current) => {
      const next = new Map(current);
      if (message) next.set(instanceId, message);
      else next.delete(instanceId);
      return next;
    });
  }

  async function toggle(instance: Instance, installedMod: InstalledMod, enabled: boolean) {
    if (!beginRowMutation(instance.id)) return;
    setRowError(instance.id);
    try {
      const updated = await modsApi.toggle(installedMod.id, enabled);
      setInstalled((current) => new Map(current).set(instance.id, updated));
      await onMutated();
    } catch (error) {
      setRowError(instance.id, errorMessage(error));
    } finally {
      endRowMutation(instance.id);
    }
  }

  async function remove(
    instanceId: string,
    installedMod: InstalledMod,
    deleteDependencies: boolean,
    alreadyLocked = false,
  ) {
    if (alreadyLocked ? !busyRows.current.has(instanceId) : !beginRowMutation(instanceId)) return;
    setRowError(instanceId);
    try {
      await modsApi.remove(installedMod.id, deleteDependencies);
      setInstalled((current) => {
        const next = new Map(current);
        next.delete(instanceId);
        return next;
      });
      await onMutated();
    } catch (error) {
      setRowError(instanceId, errorMessage(error));
    } finally {
      endRowMutation(instanceId);
    }
  }

  async function requestRemoval(instance: Instance, installedMod: InstalledMod) {
    if (!beginRowMutation(instance.id)) return;
    let ownsLock = true;
    setRowError(instance.id);
    try {
      const preview = await modsApi.previewDelete(installedMod.id);
      if (preview.dependencies.length > 0) {
        setRemoveDependencies({ mod: installedMod, dependencies: preview.dependencies });
      } else if (confirmDeletion) {
        setRemoveConfirm(installedMod);
      } else {
        ownsLock = false;
        await remove(instance.id, installedMod, false, true);
      }
    } catch (error) {
      setRowError(instance.id, errorMessage(error));
    } finally {
      if (ownsLock) endRowMutation(instance.id);
    }
  }

  const instanceFor = (installedMod: InstalledMod) =>
    instances.find((instance) => instance.id === installedMod.instanceId);

  if (removeDependencies) {
    return (
      <Modal
        title={t("remove_mod_dependencies_title", { name: removeDependencies.mod.name })}
        onClose={() => setRemoveDependencies(undefined)}
      >
        <div className="modalBody formFields">
          <p className="muted">
            {t("remove_mod_dependencies_hint", { name: removeDependencies.mod.name })}
          </p>
          <ul className="removeDependenciesList">
            {removeDependencies.dependencies.map((dependency) => (
              <li key={dependency.id}>
                <strong>{dependency.name}</strong> <small>{dependency.version}</small>
              </li>
            ))}
          </ul>
        </div>
        <div className="modalActions">
          <Button variant="ghost" onClick={() => setRemoveDependencies(undefined)}>
            {t("cancel")}
          </Button>
          <Button
            variant="secondary"
            onClick={() => {
              const target = removeDependencies;
              setRemoveDependencies(undefined);
              const instance = instanceFor(target.mod);
              if (instance) void remove(instance.id, target.mod, false);
            }}
          >
            {t("remove_mod_only")}
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              const target = removeDependencies;
              setRemoveDependencies(undefined);
              const instance = instanceFor(target.mod);
              if (instance) void remove(instance.id, target.mod, true);
            }}
          >
            {t("remove_mod_with_dependencies")}
          </Button>
        </div>
      </Modal>
    );
  }

  return (
    <Modal title={t("manage_named_mod", { name: mod.name })} onClose={onClose}>
      <div className="modalBody space-y-4">
        <SearchInput
          value={query}
          onValueChange={setQuery}
          placeholder={t("search_instances_placeholder")}
          aria-label={t("search_instances")}
        />
        {visibleInstances.length === 0 ? (
          <Empty title={t("no_instances_available")} description="" />
        ) : (
          <div className="divide-y divide-border-subtle rounded-lg border border-border-subtle">
            {visibleInstances.map((instance) => {
              const installedMod = installed.get(instance.id);
              const rowBusy = busy.has(instance.id);
              const compatibility = compatibilityFor(instance, gameVersions, release);
              return (
                <article key={instance.id} className="space-y-2 px-4 py-3">
                  <div className="flex items-center gap-3">
                    <div className="min-w-0 flex-1">
                      <strong className="block truncate">{instance.name}</strong>
                      <small className="block text-text-muted">
                        {t("vintage_story")} {instanceGameVersion(instance, gameVersions)} ·{" "}
                        {compatibilityLabel(compatibility)}
                      </small>
                      {installedMod && (
                        <small className="block text-text-muted">
                          {t("installed_version_value", { version: installedMod.version })}
                        </small>
                      )}
                    </div>
                    {installedMod ? (
                      <div className="flex shrink-0 items-center gap-2">
                        <Checkbox
                          label={t("enabled")}
                          checked={installedMod.enabled}
                          disabled={rowBusy}
                          onChange={(event) => {
                            if (!busyRows.current.has(instance.id)) {
                              void toggle(instance, installedMod, event.target.checked);
                            }
                          }}
                        />
                        <Button
                          variant="danger"
                          busy={rowBusy}
                          disabled={rowBusy}
                          aria-label={`${t("remove")} ${instance.name}`}
                          onClick={() => {
                            if (!busyRows.current.has(instance.id)) {
                              void requestRemoval(instance, installedMod);
                            }
                          }}
                        >
                          {t("remove")}
                        </Button>
                      </div>
                    ) : (
                      <Button
                        disabled={rowBusy}
                        aria-label={`${t("add_to_instance")} ${instance.name}`}
                        onClick={() => onInstallRequest(currentMod, instance)}
                      >
                        {t("add_to_instance")}
                      </Button>
                    )}
                  </div>
                  {errors.get(instance.id) && (
                    <p className="inlineError" role="alert">
                      {errors.get(instance.id)}
                    </p>
                  )}
                </article>
              );
            })}
          </div>
        )}
      </div>

      <ConfirmDialog
        open={Boolean(removeConfirm)}
        title={t("remove_mod_confirmation", { name: removeConfirm?.name })}
        destructive
        onConfirm={() => {
          const target = removeConfirm;
          setRemoveConfirm(undefined);
          const instance = target && instanceFor(target);
          if (target && instance) void remove(instance.id, target, false);
        }}
        onCancel={() => setRemoveConfirm(undefined)}
      />
    </Modal>
  );
}
