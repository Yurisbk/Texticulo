import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";

export function NotFound() {
  const { t } = useTranslation();
  return (
    <div className="mx-auto max-w-md px-4 py-24 text-center">
      <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">{t("notFound.title")}</h1>
      <Link to="/" className="mt-6 inline-block text-brand-600 dark:text-brand-400 hover:underline">
        {t("notFound.back")}
      </Link>
    </div>
  );
}
