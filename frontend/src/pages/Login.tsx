import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { apiUrl } from "../api/client";
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

export function Login() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { setSession } = useAuth();
  const [oauthError, setOauthError] = useState<string | null>(null);

  useEffect(() => {
    const err = searchParams.get("error");
    const token = searchParams.get("token");
    const emailParam = searchParams.get("email");

    if (err) {
      setOauthError(err === "oauth_denied" ? "denied" : "generic");
      const next = new URLSearchParams(searchParams);
      next.delete("error");
      setSearchParams(next, { replace: true });
      return;
    }

    if (token && emailParam) {
      const payload = parseJwtPayload(token);
      setSession(token, {
        id: payload.uid ?? "",
        email: emailParam,
      });
      setSearchParams({}, { replace: true });
      navigate("/dashboard", { replace: true });
    }
  }, [searchParams, setSearchParams, setSession, navigate]);

  const googleHref = apiUrl("/api/auth/google");

  return (
    <div className="mx-auto max-w-md px-4 py-12">
      <h1 className="mb-2 text-center text-2xl font-bold text-slate-900 dark:text-slate-100">
        {t("auth.loginTitle")}
      </h1>
      <p className="mb-8 text-center text-sm text-slate-600 dark:text-slate-400">
        {t("auth.oauthSubtitle")}
      </p>

      <div className="space-y-3 rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-6 shadow-sm">
        {oauthError === "denied" && (
          <p className="text-center text-sm text-amber-700 dark:text-amber-300">{t("auth.oauthDenied")}</p>
        )}
        {oauthError === "generic" && (
          <p className="text-center text-sm text-red-600 dark:text-red-400">{t("auth.oauthError")}</p>
        )}

        <a
          href={googleHref}
          className="flex w-full items-center justify-center gap-2 rounded-xl border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-900 py-3 font-semibold text-slate-800 dark:text-slate-100 shadow-sm hover:bg-slate-50 dark:hover:bg-slate-800 transition"
        >
          <svg className="h-5 w-5" viewBox="0 0 24 24" aria-hidden>
            <path
              fill="#4285F4"
              d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
            />
            <path
              fill="#34A853"
              d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
            />
            <path
              fill="#FBBC05"
              d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
            />
            <path
              fill="#EA4335"
              d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
            />
          </svg>
          {t("auth.googleLogin")}
        </a>
      </div>
    </div>
  );
}
