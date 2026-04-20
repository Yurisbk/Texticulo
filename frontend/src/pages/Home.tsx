import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { QRCodeSVG } from "qrcode.react";
import { getMetrics, shortenUrl, type MetricsResponse } from "../api/client";
import { useAuth } from "../context/AuthContext";

export function Home() {
  const { t } = useTranslation();
  const { token } = useAuth();
  const [url, setUrl] = useState("");
  const [shortUrl, setShortUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [metrics, setMetrics] = useState<MetricsResponse | null>(null);

  useEffect(() => {
    getMetrics().then(setMetrics).catch(() => null);
  }, []);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setShortUrl(null);
    setLoading(true);
    try {
      const res = await shortenUrl(url, token);
      setShortUrl(res.short_url);
      // refresh metrics after shortening
      getMetrics().then(setMetrics).catch(() => null);
    } catch (err) {
      if (err instanceof Error) {
        if (err.message === "rate_limit") setError("rateLimit");
        else if (err.message === "link_limit") setError("linkLimit");
        else setError("generic");
      } else {
        setError("generic");
      }
    } finally {
      setLoading(false);
    }
  }

  async function copy() {
    if (!shortUrl) return;
    await navigator.clipboard.writeText(shortUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="mx-auto max-w-xl px-4 py-12">
      <p className="mb-2 text-center text-sm uppercase tracking-widest text-brand-500 dark:text-brand-400 font-semibold">
        {t("app.name")}
      </p>
      <h1 className="mb-2 text-center text-3xl font-bold text-slate-900 dark:text-slate-100">
        {t("home.title")}
      </h1>
      <p className="mb-8 text-center text-slate-600 dark:text-slate-400">{t("app.tagline")}</p>

      {!token && (
        <p className="mb-4 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/30 px-3 py-2 text-center text-sm text-amber-900 dark:text-amber-300">
          {t("home.hintLogin")}
        </p>
      )}

      <form onSubmit={onSubmit} className="space-y-4">
        <input
          type="url"
          required
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder={t("home.placeholder")}
          className="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-3 text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 shadow-sm outline-none ring-brand-500 focus:ring-2"
        />
        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-xl bg-brand-600 py-3 font-semibold text-white shadow hover:bg-brand-700 disabled:opacity-60 transition"
        >
          {loading ? "…" : t("home.submit")}
        </button>
      </form>

      {error === "generic" && (
        <p className="mt-4 text-center text-sm text-red-600 dark:text-red-400">{t("home.error")}</p>
      )}
      {error === "rateLimit" && (
        <p className="mt-4 text-center text-sm text-red-600 dark:text-red-400">{t("home.rateLimit")}</p>
      )}
      {error === "linkLimit" && (
        <p className="mt-4 text-center text-sm text-red-600 dark:text-red-400">{t("home.linkLimit")}</p>
      )}

      {shortUrl && (
        <div className="mt-10 rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-6 shadow-sm">
          <p className="mb-2 text-sm font-medium text-slate-500 dark:text-slate-400">{t("home.result")}</p>
          <div className="flex flex-col items-center gap-6 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 flex-1">
              <p className="break-all font-mono text-lg text-brand-600 dark:text-brand-400">{shortUrl}</p>
              <button
                type="button"
                onClick={copy}
                className="mt-3 rounded-lg bg-slate-100 dark:bg-slate-700 px-4 py-2 text-sm font-medium text-slate-800 dark:text-slate-200 hover:bg-slate-200 dark:hover:bg-slate-600 transition"
              >
                {copied ? t("home.copied") : t("home.copy")}
              </button>
            </div>
            <div className="shrink-0 rounded-xl bg-white p-2 shadow-inner ring-1 ring-slate-100">
              <QRCodeSVG value={shortUrl} size={120} level="M" includeMargin />
            </div>
          </div>
        </div>
      )}

      {/* Global metrics bar */}
      {metrics && (
        <div className="mt-12 flex flex-wrap items-center justify-center gap-6 rounded-2xl border border-slate-100 dark:border-slate-800 bg-white dark:bg-slate-900/50 px-6 py-4">
          <Stat value={metrics.total_links} label={t("metrics.links")} />
          <Divider />
          <Stat value={metrics.total_clicks} label={t("metrics.clicks")} />
          <Divider />
          <Stat value={metrics.clicks_today} label={t("metrics.clicksToday")} />
          <Divider />
          <Stat value={metrics.total_users} label={t("metrics.users")} />
        </div>
      )}

    </div>
  );
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <div className="text-center">
      <p className="text-xl font-black text-brand-600 dark:text-brand-400">{value.toLocaleString()}</p>
      <p className="text-xs text-slate-500 dark:text-slate-400">{label}</p>
    </div>
  );
}

function Divider() {
  return <span className="hidden sm:block text-slate-300 dark:text-slate-600 text-lg">·</span>;
}
