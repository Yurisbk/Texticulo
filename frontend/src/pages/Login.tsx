import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { apiUrl, loginEmail, registerEmail } from "../api/client";
import { useAuth } from "../context/AuthContext";

function parseJwtPayload(token: string): { uid?: string; email?: string } {
  try {
    const b64 = token.split(".")[1];
    if (!b64) return {};
    const json = atob(b64.replace(/-/g, "+").replace(/_/g, "/"));
    return JSON.parse(json) as { uid?: string; email?: string };
  } catch {
    return {};
  }
}

type Mode = "login" | "register";

export function Login() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { setSession } = useAuth();

  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [oauthError, setOauthError] = useState<string | null>(null);

  const handledRef = useRef(false);

  // Handle OAuth redirect — token is delivered via URL fragment (never reaches server logs).
  useEffect(() => {
    if (handledRef.current) return;

    const hash = window.location.hash.slice(1);
    if (hash) {
      const params = new URLSearchParams(hash);
      const token = params.get("token");
      const emailParam = params.get("email");
      if (token && emailParam) {
        handledRef.current = true;
        const payload = parseJwtPayload(token);
        setSession(token, { id: payload.uid ?? "", email: emailParam });
        window.history.replaceState(null, "", window.location.pathname + window.location.search);
        navigate("/dashboard", { replace: true });
        return;
      }
    }

    // Handle OAuth error query param
    const err = searchParams.get("error");
    if (err) {
      setOauthError(err === "oauth_denied" ? "denied" : "generic");
      const next = new URLSearchParams(searchParams);
      next.delete("error");
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, setSearchParams, setSession, navigate]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const fn = mode === "login" ? loginEmail : registerEmail;
      const data = await fn(email.trim().toLowerCase(), password);
      const payload = parseJwtPayload(data.token);
      setSession(data.token, { id: payload.uid ?? "", email: data.email });
      navigate("/dashboard", { replace: true });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "login_failed";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  function errorText(key: string): string {
    const map: Record<string, string> = {
      invalid_credentials: t("auth.invalidCredentials"),
      email_taken: t("auth.emailTaken"),
      weak_password: t("auth.weakPassword"),
      invalid_email: t("auth.loginError"),
      use_google: t("auth.useGoogle"),
      too_many_attempts: t("auth.tooManyAttempts"),
    };
    return map[key] ?? t("auth.loginError");
  }

  const googleHref = apiUrl("/api/auth/google");

  return (
    <div className="mx-auto max-w-md px-4 py-12">
      <h1 className="mb-1 text-center text-2xl font-bold text-slate-900 dark:text-slate-100">
        {mode === "login" ? t("auth.loginTitle") : t("auth.registerTitle")}
      </h1>
      <p className="mb-8 text-center text-sm text-slate-600 dark:text-slate-400">
        {t("auth.subtitle")}
      </p>

      <div className="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-6 shadow-sm space-y-5">
        {/* OAuth error banners */}
        {oauthError === "denied" && (
          <p className="text-center text-sm text-amber-700 dark:text-amber-300">{t("auth.oauthDenied")}</p>
        )}
        {oauthError === "generic" && (
          <p className="text-center text-sm text-red-600 dark:text-red-400">{t("auth.oauthError")}</p>
        )}

        {/* Google button */}
        <a
          href={googleHref}
          className="flex w-full items-center justify-center gap-2 rounded-xl border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-900 py-3 font-semibold text-slate-800 dark:text-slate-100 shadow-sm hover:bg-slate-50 dark:hover:bg-slate-800 transition"
        >
          <svg className="h-5 w-5" viewBox="0 0 24 24" aria-hidden>
            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
          </svg>
          {t("auth.googleLogin")}
        </a>

        {/* Divider */}
        <div className="flex items-center gap-3">
          <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
          <span className="text-xs text-slate-400 dark:text-slate-500">{t("auth.orDivider")}</span>
          <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
        </div>

        {/* Email / password form */}
        <form onSubmit={handleSubmit} className="space-y-4" noValidate>
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">
              {t("auth.emailLabel")}
            </label>
            <input
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-xl border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-900 px-4 py-2.5 text-sm text-slate-900 dark:text-slate-100 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              placeholder="you@example.com"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">
              {t("auth.passwordLabel")}
            </label>
            <input
              type="password"
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-xl border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-900 px-4 py-2.5 text-sm text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
            {mode === "register" && (
              <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">{t("auth.passwordHint")}</p>
            )}
          </div>

          {error && (
            <p className="text-center text-sm text-red-600 dark:text-red-400">{errorText(error)}</p>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-xl bg-indigo-600 py-3 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50 transition"
          >
            {loading
              ? "…"
              : mode === "login"
              ? t("auth.submitLogin")
              : t("auth.submitRegister")}
          </button>
        </form>

        {/* Mode toggle */}
        <p className="text-center text-sm text-slate-500 dark:text-slate-400">
          <button
            type="button"
            onClick={() => { setMode(mode === "login" ? "register" : "login"); setError(null); }}
            className="font-medium text-indigo-600 dark:text-indigo-400 hover:underline"
          >
            {mode === "login" ? t("auth.switchToRegister") : t("auth.switchToLogin")}
          </button>
        </p>
      </div>
    </div>
  );
}
