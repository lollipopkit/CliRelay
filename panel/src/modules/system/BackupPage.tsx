import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Archive, Download, HardDrive, RefreshCw, Trash2, Upload, Clock, Zap, FolderUp } from "lucide-react";
import { apiClient } from "@/lib/http/client";
import { Button } from "@/modules/ui/Button";
import { Card } from "@/modules/ui/Card";
import { useToast } from "@/modules/ui/ToastProvider";

interface BackupInfo {
  name: string;
  path: string;
  size: number;
  created_at: string;
  source: string;
}

interface ListBackupsResponse {
  backups: BackupInfo[];
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB"];
  let size = bytes;
  let unitIdx = 0;
  while (size >= 1024 && unitIdx < units.length - 1) {
    size /= 1024;
    unitIdx++;
  }
  return `${size.toFixed(1)} ${units[unitIdx]}`;
}

function formatDate(dateStr: string): string {
  try {
    const d = new Date(dateStr);
    return d.toLocaleString();
  } catch {
    return dateStr;
  }
}

export function BackupPage() {
  const { t } = useTranslation();
  const { notify } = useToast();
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadBackups = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await apiClient.get<ListBackupsResponse>("/management/backup");
      setBackups(resp.backups || []);
    } catch (err: unknown) {
      notify({
        type: "error",
        message: err instanceof Error ? err.message : t("backup_page.load_failed"),
      });
    } finally {
      setLoading(false);
    }
  }, [notify, t]);

  useEffect(() => {
    void loadBackups();
  }, [loadBackups]);

  const handleCreate = useCallback(async () => {
    setCreating(true);
    try {
      await apiClient.post("/management/backup", { include_auth_files: false });
      notify({ type: "success", message: t("backup_page.create_success") });
      void loadBackups();
    } catch (err: unknown) {
      notify({
        type: "error",
        message: err instanceof Error ? err.message : t("backup_page.create_failed"),
      });
    } finally {
      setCreating(false);
    }
  }, [notify, t, loadBackups]);

  const handleDownload = useCallback(
    async (name: string) => {
      try {
        const blob = await apiClient.getBlob(`/management/backup/${encodeURIComponent(name)}/download`);
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = name;
        a.click();
        URL.revokeObjectURL(url);
      } catch (err: unknown) {
        notify({
          type: "error",
          message: err instanceof Error ? err.message : t("backup_page.download_failed"),
        });
      }
    },
    [notify, t],
  );

  const handleDelete = useCallback(
    async (name: string) => {
      if (!window.confirm(t("backup_page.delete_confirm", { name }))) return;
      try {
        await apiClient.delete(`/management/backup/${encodeURIComponent(name)}`);
        notify({ type: "success", message: t("backup_page.delete_success") });
        void loadBackups();
      } catch (err: unknown) {
        notify({
          type: "error",
          message: err instanceof Error ? err.message : t("backup_page.delete_failed"),
        });
      }
    },
    [notify, t, loadBackups],
  );

  const handleRestore = useCallback(
    async (name: string) => {
      if (!window.confirm(t("backup_page.restore_confirm", { name }))) return;
      try {
        await apiClient.post(`/management/backup/${encodeURIComponent(name)}/restore`, {
          restore_config: false,
          confirm: true,
        });
        notify({
          type: "success",
          message: t("backup_page.restore_success"),
          duration: 5000,
        });
      } catch (err: unknown) {
        notify({
          type: "error",
          message: err instanceof Error ? err.message : t("backup_page.restore_failed"),
        });
      }
    },
    [notify, t],
  );

  const handleFileUpload = useCallback(
    async (file: File) => {
      if (!window.confirm(t("backup_page.upload_restore_confirm"))) return;
      setUploading(true);
      try {
        const formData = new FormData();
        formData.append("file", file, file.name);
        formData.append("confirm", "true");
        formData.append("restore_config", "false");
        await apiClient.postForm("/management/backup/upload-restore", formData);
        notify({
          type: "success",
          message: t("backup_page.upload_success"),
          duration: 5000,
        });
        void loadBackups();
      } catch (err: unknown) {
        notify({
          type: "error",
          message: err instanceof Error ? err.message : t("backup_page.upload_failed"),
        });
      } finally {
        setUploading(false);
      }
    },
    [notify, t, loadBackups],
  );

  const handleFileInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) {
        void handleFileUpload(file);
      }
      // Reset so same file can be re-selected
      if (fileInputRef.current) fileInputRef.current.value = "";
    },
    [handleFileUpload],
  );

  return (
    <div className="min-w-0 space-y-6 overflow-x-hidden">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-100 dark:bg-amber-900/30">
            <HardDrive size={16} className="text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold tracking-tight text-slate-900 dark:text-white">
              {t("backup_page.title")}
            </h2>
            <p className="hidden text-xs text-slate-500 dark:text-white/45 sm:block">
              {t("backup_page.subtitle")}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={() => void loadBackups()} disabled={loading}>
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} />
            {t("backup_page.refresh")}
          </Button>
          <Button variant="primary" size="sm" onClick={() => void handleCreate()} disabled={creating}>
            <Zap size={13} className={creating ? "animate-spin" : ""} />
            {t("backup_page.create_now")}
          </Button>
        </div>
      </div>

      {/* Upload & Restore area */}
      <Card padding="default">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">
              {t("backup_page.upload_restore")}
            </h3>
            <p className="mt-0.5 text-xs text-slate-500 dark:text-white/45">
              {t("backup_page.upload_restore_desc")}
            </p>
          </div>
          <div>
            <input
              ref={fileInputRef}
              type="file"
              accept=".tar.gz,application/gzip,application/x-gzip"
              className="hidden"
              onChange={handleFileInputChange}
            />
            <Button
              variant="secondary"
              size="sm"
              disabled={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              <FolderUp size={13} className={uploading ? "animate-pulse" : ""} />
              {uploading ? "…" : t("backup_page.upload_restore")}
            </Button>
          </div>
        </div>
      </Card>

      {/* Backup List */}
      <Card padding="none" className="overflow-hidden" bodyClassName="mt-0">
        <div className="flex items-center justify-between border-b border-slate-100 px-5 py-3.5 dark:border-neutral-800">
          <div className="flex items-center gap-2.5">
            <Archive size={15} className="text-slate-500 dark:text-white/40" />
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">
              {t("backup_page.backup_list")}
            </h3>
            <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-bold tabular-nums text-amber-600 dark:bg-amber-900/30 dark:text-amber-300">
              {backups.length}
            </span>
          </div>
        </div>

        {loading && backups.length === 0 ? (
          <div className="flex items-center justify-center py-12 text-sm text-slate-500 dark:text-white/50">
            <RefreshCw size={14} className="mr-2 animate-spin" />
            {t("backup_page.loading")}
          </div>
        ) : backups.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-slate-100 bg-slate-50/60 dark:border-neutral-800 dark:bg-neutral-900/30">
                <tr>
                  <th className="px-5 py-2.5 text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-white/45">
                    {t("backup_page.col_name")}
                  </th>
                  <th className="px-5 py-2.5 text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-white/45">
                    {t("backup_page.col_size")}
                  </th>
                  <th className="px-5 py-2.5 text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-white/45">
                    {t("backup_page.col_created")}
                  </th>
                  <th className="px-5 py-2.5 text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-white/45">
                    {t("backup_page.col_source")}
                  </th>
                  <th className="px-5 py-2.5 text-right text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-white/45">
                    {t("backup_page.col_actions")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-50 dark:divide-neutral-800/60">
                {backups.map((b) => (
                  <tr
                    key={b.name}
                    className="transition hover:bg-slate-50/60 dark:hover:bg-neutral-900/20"
                  >
                    <td className="px-5 py-3 font-mono text-xs text-slate-800 dark:text-white">
                      {b.name}
                    </td>
                    <td className="px-5 py-3 text-slate-600 dark:text-white/70">
                      {formatSize(b.size)}
                    </td>
                    <td className="px-5 py-3 whitespace-nowrap text-xs text-slate-500 dark:text-white/45">
                      <span className="inline-flex items-center gap-1">
                        <Clock size={11} />
                        {formatDate(b.created_at)}
                      </span>
                    </td>
                    <td className="px-5 py-3">
                      <span
                        className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                          b.source === "cron"
                            ? "bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300"
                            : "bg-sky-50 text-sky-600 dark:bg-sky-900/30 dark:text-sky-300"
                        }`}
                      >
                        {b.source}
                      </span>
                    </td>
                    <td className="px-5 py-3 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => void handleDownload(b.name)}
                          title={t("backup_page.download")}
                        >
                          <Download size={14} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => void handleRestore(b.name)}
                          title={t("backup_page.restore")}
                        >
                          <Upload size={14} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => void handleDelete(b.name)}
                          title={t("backup_page.delete")}
                        >
                          <Trash2 size={14} className="text-rose-500" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-12 text-slate-400 dark:text-white/30">
            <Archive size={28} className="mb-2 opacity-40" />
            <p className="text-sm">{t("backup_page.no_backups")}</p>
            <Button variant="secondary" size="sm" className="mt-3" onClick={() => void handleCreate()}>
              {t("backup_page.create_first")}
            </Button>
          </div>
        )}
      </Card>
    </div>
  );
}