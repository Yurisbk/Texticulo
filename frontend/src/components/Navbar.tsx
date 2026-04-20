import { Link, NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { LanguageToggle } from "./LanguageToggle";
import { DarkModeToggle } from "./DarkModeToggle";
import { useAuth } from "../context/AuthContext";

export function Navbar() {
  const { t } = useTranslation();
  const { token, logout } = useAuth();

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `rounded-md px-3 py-2 text-sm font-medium transition ${
      isActive
        ? "bg-brand-100 text-brand-700 dark:bg-brand-900/40 dark:text-brand-300"
        : "text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
    }`;

  return (
    <header className="border-b border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/80 backdrop-blur sticky top-0 z-20">
      <div className="mx-auto flex max-w-4xl items-center justify-between gap-4 px-4 py-3">
        <Link to="/" className="logo-text text-3xl font-black tracking-tight leading-none text-brand-600 dark:text-brand-400">
          {t("app.name")}
        </Link>
        <nav className="flex flex-wrap items-center gap-2">
          <NavLink to="/" className={linkClass} end>
            {t("nav.home")}
          </NavLink>
          {token && (
            <NavLink to="/dashboard" className={linkClass}>
              {t("nav.dashboard")}
            </NavLink>
          )}
          <LanguageToggle />
          <DarkModeToggle />
          {token ? (
            <button
              type="button"
              onClick={logout}
              className="rounded-md px-3 py-2 text-sm font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition"
            >
              {t("nav.logout")}
            </button>
          ) : (
            <NavLink
              to="/login"
              className="rounded-md bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700 transition"
            >
              {t("nav.login")}
            </NavLink>
          )}
        </nav>
      </div>
    </header>
  );
}
