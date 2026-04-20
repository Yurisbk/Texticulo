import { useTranslation } from "react-i18next";

export function LanguageToggle() {
  const { i18n, t } = useTranslation();
  const isPt = i18n.language.startsWith("pt");

  return (
    <div className="flex rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 p-0.5 text-sm shadow-sm">
      <button
        type="button"
        className={`rounded-md px-2 py-1 transition ${
          isPt
            ? "bg-brand-600 text-white"
            : "text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
        }`}
        onClick={() => i18n.changeLanguage("pt")}
        aria-pressed={isPt}
      >
        {t("lang.pt")}
      </button>
      <button
        type="button"
        className={`rounded-md px-2 py-1 transition ${
          !isPt
            ? "bg-brand-600 text-white"
            : "text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
        }`}
        onClick={() => i18n.changeLanguage("en")}
        aria-pressed={!isPt}
      >
        {t("lang.en")}
      </button>
    </div>
  );
}
