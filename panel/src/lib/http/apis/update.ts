import { apiClient } from "@/lib/http/client";

export interface UpdateCheckResponse {
  enabled: boolean;
  current_version: string;
  current_commit: string;
  current_ui_version?: string;
  current_ui_commit?: string;
  build_date: string;
  target_channel: string;
  latest_version: string;
  latest_commit: string;
  latest_commit_url?: string;
  latest_ui_version?: string;
  latest_ui_commit?: string;
  latest_ui_commit_url?: string;
  docker_tag: string;
  release_notes?: string;
  release_url?: string;
  update_available: boolean;
  updater_available: boolean;
  message?: string;
}

export const updateApi = {
  check: (signal?: AbortSignal) =>
    apiClient.get<UpdateCheckResponse>("/update/check", {
      signal,
      timeoutMs: 15_000,
    }),
  current: (signal?: AbortSignal) =>
    apiClient.get<UpdateCheckResponse>("/update/current", {
      signal,
      timeoutMs: 5_000,
    }),
};
