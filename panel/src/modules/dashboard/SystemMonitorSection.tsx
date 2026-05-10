import { useTranslation } from "react-i18next";
import {
  Database,
  FileText,
  Clock,
  Network,
  Activity,
  Zap,
  Layers,
} from "lucide-react";
import { Card } from "@/modules/ui/Card";
import type { SystemStats } from "./useSystemStats";

const PANEL_SURFACE =
  "rounded-[18px] border border-slate-200/85 bg-white shadow-[0_10px_26px_rgba(15,23,42,0.05)] dark:border-neutral-800 dark:bg-neutral-950/85 dark:shadow-[0_10px_26px_rgba(0,0,0,0.28)]";

/* ═══════════════════════════════════════════════════════════
   Helpers
   ═══════════════════════════════════════════════════════════ */

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function formatUptime(s: number): string {
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function formatMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

/* ═══════════════════════════════════════════════════════════
   Mini KPI (for top-right grid)
   ═══════════════════════════════════════════════════════════ */

function MiniKpi({
  label,
  value,
  icon: Icon,
  color = "text-slate-900 dark:text-white",
  sublabel,
}: {
  label: string;
  value: string;
  icon: typeof Activity;
  color?: string;
  sublabel?: string;
}) {
  return (
    <Card padding="compact" bodyClassName="mt-0" className={`${PANEL_SURFACE} h-full`}>
      <div className="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-white/40">
        <Icon size={12} />
        {label}
      </div>
      <p className={`mt-1.5 text-lg font-bold tabular-nums ${color}`}>{value}</p>
      {sublabel && (
        <p className="mt-0.5 text-[10px] text-slate-400 dark:text-white/35">{sublabel}</p>
      )}
    </Card>
  );
}

/* ═══════════════════════════════════════════════════════════
   Channel Latency (compact bar chart)
   ═══════════════════════════════════════════════════════════ */

function AverageLatencyCard({
  avgLatency,
  apiKeyCount,
}: {
  avgLatency: number;
  apiKeyCount: number;
}) {
  const { t } = useTranslation();

  return (
    <Card
      padding="compact"
      bodyClassName="mt-0"
      className={`${PANEL_SURFACE} h-full overflow-hidden`}
    >
      <div className="mb-2.5 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-white/40">
        <Network size={12} />
        {t("system_monitor.channel_avg_latency")}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-[12px] bg-slate-50 px-3 py-2.5 dark:bg-neutral-900/70 dark:ring-1 dark:ring-white/8">
          <div className="text-[9px] font-semibold uppercase tracking-wider text-slate-400 dark:text-white/45">
            {t("system_monitor.latency")}
          </div>
          <div className="mt-1 text-xl font-bold tabular-nums text-slate-900 dark:text-white">
            {formatMs(avgLatency)}
          </div>
        </div>
        <div className="rounded-[12px] bg-slate-50 px-3 py-2.5 dark:bg-neutral-900/70 dark:ring-1 dark:ring-white/8">
          <div className="text-[9px] font-semibold uppercase tracking-wider text-slate-400 dark:text-white/45">
            {t("system_monitor.key_count")}
          </div>
          <div className="mt-1 text-xl font-bold tabular-nums text-slate-900 dark:text-white">
            {apiKeyCount}
          </div>
        </div>
      </div>
    </Card>
  );
}

/* ═══════════════════════════════════════════════════════════
   Skeleton
   ═══════════════════════════════════════════════════════════ */

function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded bg-slate-200 dark:bg-neutral-700 ${className}`} />;
}

function SkeletonLayout() {
  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Card key={i} padding="compact" bodyClassName="mt-0">
            <Skeleton className="h-3 w-16 mb-3" />
            <Skeleton className="h-5 w-20" />
          </Card>
        ))}
      </div>
      <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
        <Card padding="compact" bodyClassName="mt-0">
          <Skeleton className="h-3 w-24 mb-3" />
          <Skeleton className="h-12 w-full" />
        </Card>
        <Card padding="compact" bodyClassName="mt-0">
          <Skeleton className="h-3 w-16 mb-3" />
          <Skeleton className="h-5 w-20" />
        </Card>
      </div>
    </div>
  );
}

/* ═══════════════════════════════════════════════════════════
   Main Section — exported
   ═══════════════════════════════════════════════════════════ */

export function SystemMonitorSection({
  stats,
  connected = false,
  apiKeyCount = 0,
}: {
  stats?: SystemStats | null;
  connected?: boolean;
  apiKeyCount?: number;
}) {
  const { t } = useTranslation();

  if (!stats) {
    return (
      <Card
        title={t("system_monitor.title")}
        className={PANEL_SURFACE}
        actions={
          <div className="flex items-center gap-1.5 text-xs text-slate-400">
            <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-slate-300 dark:bg-neutral-600" />
            {t("system_monitor.connecting")}
          </div>
        }
      >
        <SkeletonLayout />
      </Card>
    );
  }

  const logDirSizeBytes = stats.log_dir_size_bytes || stats.log_size_bytes;
  const channelLatency = stats.channel_latency ?? [];
  const latencyWeight = channelLatency.reduce((acc, item) => acc + item.count, 0);
  const averageLatency =
    latencyWeight > 0
      ? channelLatency.reduce((acc, item) => acc + item.avg_ms * item.count, 0) / latencyWeight
      : 0;

  return (
    <Card
      title={t("system_monitor.title")}
      description={t("system_monitor.updated_at", { time: new Date().toLocaleTimeString() })}
      className={PANEL_SURFACE}
      actions={
        <div className="flex items-center gap-1.5 text-xs text-slate-400">
          <span
            className={`inline-block h-2 w-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-slate-300 dark:bg-neutral-600"}`}
          />
          {connected ? t("system_monitor.live") : t("system_monitor.polling")}
        </div>
      }
    >
      <div className="space-y-3">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <MiniKpi
            label={t("system_monitor.uptime")}
            value={formatUptime(stats.uptime_seconds)}
            icon={Clock}
            sublabel={t("system_monitor.started", {
              time: new Date(stats.start_time).toLocaleString(),
            })}
          />
          <MiniKpi
            label={t("system_monitor.goroutines")}
            value={String(stats.go_routines)}
            icon={Zap}
            color="text-violet-500"
            sublabel={t("system_monitor.heap", { size: formatBytes(stats.go_heap_bytes) })}
          />
          <MiniKpi
            label={t("system_monitor.database")}
            value={formatBytes(stats.db_size_bytes)}
            icon={Database}
            sublabel={t("system_monitor.sqlite_wal_shm")}
          />
          <MiniKpi
            label={t("system_monitor.log_storage")}
            value={formatBytes(stats.log_content_store_bytes)}
            icon={FileText}
            sublabel={t("system_monitor.request_log_content")}
          />
        </div>

        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
          <AverageLatencyCard avgLatency={averageLatency} apiKeyCount={apiKeyCount} />
          <MiniKpi
            label={t("system_monitor.log_dir")}
            value={formatBytes(logDirSizeBytes)}
            icon={Layers}
            sublabel={t("system_monitor.log_files")}
          />
        </div>
      </div>
    </Card>
  );
}
