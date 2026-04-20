import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { deleteLink, getMetrics, linkStats, listLinks, type LinkRow } from "../api/client";
import { useAuth } from "../context/AuthContext";

export function Dashboard() {
  const { t } = useTranslation();
  const { token } = useAuth();
  const [links, setLinks] = useState<LinkRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [totalClicks, setTotalClicks] = useState<number | null>(null);

  useEffect(() => {
    if (!token) return;
    loadLinks();
  }, [token]);

  async function loadLinks() {
    if (!token) return;
    setLoading(true);
    setError(false);
    try {
      const data = await listLinks(token);
      setLinks(data);
      const sum = data.reduce((acc, l) => acc + l.clicks, 0);
      setTotalClicks(sum);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }

  async function handleDelete(code: string) {
    if (!token) return;
    if (!confirm(t("dashboard.deleteConfirm"))) return;
    try {
      await deleteLink(code, token);
      setLinks((prev) => prev.filter((l) => l.short_code !== code));
      // refresh metrics
      getMetrics().catch(() => null);
    } catch {
      alert(t("dashboard.deleteError"));
    }
  }

  const usagePercent = Math.min((links.length / 5) * 100, 100);

  return (
    <div className="mx-auto max-w-4xl px-4 py-10">
      <h1 className="mb-6 text-2xl font-bold text-slate-900 dark:text-slate-100">{t("dashboard.title")}</h1>

      {/* Usage + total clicks */}
      {!loading && !error && (
        <div className="mb-6 grid gap-4 sm:grid-cols-2">
          <div className="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-4">
            <p className="mb-1 text-sm font-medium text-slate-600 dark:text-slate-400">
              {t("dashboard.usage", { count: links.length })}
            </p>
            <div className="h-2.5 rounded-full bg-slate-100 dark:bg-slate-700">
              <div
                className="h-2.5 rounded-full bg-brand-500 transition-all"
                style={{ width: `${usagePercent}%` }}
              />
            </div>
          </div>
          <div className="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-4 flex items-center gap-3">
            <span className="text-2xl font-black text-brand-600 dark:text-brand-400">
              {(totalClicks ?? 0).toLocaleString()}
            </span>
            <span className="text-sm text-slate-600 dark:text-slate-400">{t("dashboard.yourClicks")}</span>
          </div>
        </div>
      )}

      {loading && <p className="text-slate-500 dark:text-slate-400">…</p>}
      {error && <p className="text-red-600 dark:text-red-400">{t("dashboard.loadError")}</p>}
      {!loading && !error && links.length === 0 && (
        <p className="rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-6 text-slate-600 dark:text-slate-400">
          {t("dashboard.empty")}
        </p>
      )}

      {!loading && links.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 shadow-sm">
          <table className="min-w-full text-left text-sm">
            <thead className="bg-slate-50 dark:bg-slate-900/50 text-slate-600 dark:text-slate-400">
              <tr>
                <th className="px-4 py-3 font-medium">{t("dashboard.short")}</th>
                <th className="px-4 py-3 font-medium">{t("dashboard.original")}</th>
                <th className="px-4 py-3 font-medium">{t("dashboard.clicks")}</th>
                <th className="px-4 py-3 font-medium">{t("dashboard.created")}</th>
                <th className="px-4 py-3 font-medium" />
              </tr>
            </thead>
            <tbody>
              {links.map((row) => (
                <DashboardRow
                  key={row.short_code}
                  row={row}
                  expanded={expanded === row.short_code}
                  onToggleStats={() =>
                    setExpanded((prev) => (prev === row.short_code ? null : row.short_code))
                  }
                  onDelete={() => handleDelete(row.short_code)}
                  token={token!}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function DashboardRow({
  row,
  expanded,
  onToggleStats,
  onDelete,
  token,
}: {
  row: LinkRow;
  expanded: boolean;
  onToggleStats: () => void;
  onDelete: () => void;
  token: string;
}) {
  const { t } = useTranslation();
  const [detail, setDetail] = useState<{ recent_clicks: string[] } | null>(null);

  useEffect(() => {
    if (!expanded) {
      setDetail(null);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const s = await linkStats(row.short_code, token);
        if (!cancelled) setDetail({ recent_clicks: s.recent_clicks });
      } catch {
        if (!cancelled) setDetail(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [expanded, row.short_code, token]);

  return (
    <>
      <tr className="border-t border-slate-100 dark:border-slate-700">
        <td className="px-4 py-3 font-mono text-brand-600 dark:text-brand-400">
          <a href={row.short_url} className="hover:underline" target="_blank" rel="noreferrer">
            {row.short_url}
          </a>
        </td>
        <td className="max-w-xs truncate px-4 py-3 text-slate-700 dark:text-slate-300" title={row.original_url}>
          {row.original_url}
        </td>
        <td className="px-4 py-3 text-slate-700 dark:text-slate-300">{row.clicks.toLocaleString()}</td>
        <td className="px-4 py-3 text-slate-500 dark:text-slate-400">
          {new Date(row.created_at).toLocaleDateString()}
        </td>
        <td className="px-4 py-3">
          <div className="flex gap-3">
            <button type="button" onClick={onToggleStats} className="text-brand-600 dark:text-brand-400 hover:underline text-sm">
              {t("dashboard.stats")}
            </button>
            <button type="button" onClick={onDelete} className="text-red-500 dark:text-red-400 hover:underline text-sm">
              {t("dashboard.delete")}
            </button>
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-t border-slate-100 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30">
          <td colSpan={5} className="px-4 py-4 text-sm text-slate-700 dark:text-slate-300">
            <p className="mb-2 font-medium">{t("dashboard.recentClicks")}</p>
            {detail && detail.recent_clicks.length === 0 && (
              <p className="text-slate-500 dark:text-slate-500">—</p>
            )}
            {detail && detail.recent_clicks.length > 0 && (
              <ul className="list-inside list-disc space-y-1">
                {detail.recent_clicks.slice(0, 20).map((c) => (
                  <li key={c} className="font-mono text-xs">
                    {c}
                  </li>
                ))}
              </ul>
            )}
            {!detail && expanded && <p className="text-slate-500 dark:text-slate-500">…</p>}
          </td>
        </tr>
      )}
    </>
  );
}
