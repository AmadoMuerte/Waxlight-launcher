import { call } from "./bridge";

export interface SupportReportPreview {
  snapshotId: string;
  payload: string;
}

export interface SupportReportResult {
  reportId: string;
  status: string;
}

export const supportReportsApi = {
  preview: (description: string, instanceId = "") =>
    call<SupportReportPreview>("SupportReportController", "Preview", description, instanceId),
  submit: (snapshotId: string) =>
    call<SupportReportResult>("SupportReportController", "Submit", snapshotId),
};
