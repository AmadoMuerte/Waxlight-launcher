import { call } from "./bridge";
import type { ImportReport, PackageAuthor, PackageInspection, PackageManifest } from "./types";

export const instancePackageApi = {
  export: (request: {
    instanceId: string;
    targetPath: string;
    name?: string;
    description?: string;
    author?: PackageAuthor;
  }) => call<PackageManifest>("InstancePackageController", "ExportInstance", request),
  inspect: (packagePath: string) =>
    call<PackageInspection>("InstancePackageController", "InspectPackage", packagePath),
  import: (request: {
    packagePath: string;
    name: string;
    description?: string;
    directory: string;
    gameVersionId: string;
    installVersion: boolean;
    allowIncompatible: boolean;
    skipUnavailable: boolean;
  }) => call<ImportReport>("InstancePackageController", "ImportPackage", request),
  selectExportPath: (suggestedName: string) =>
    call<string>("InstancePackageController", "SelectExportPath", suggestedName),
  selectPackageFile: () => call<string>("InstancePackageController", "SelectPackageFile"),
};
