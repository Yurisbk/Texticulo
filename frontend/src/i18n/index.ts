import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./en.json";
import pt from "./pt.json";

const saved = typeof localStorage !== "undefined" ? localStorage.getItem("texticulo_lang") : null;
const fallback = "en";

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    pt: { translation: pt },
  },
  lng: saved === "pt" || saved === "en" ? saved : typeof navigator !== "undefined" && navigator.language.startsWith("pt") ? "pt" : fallback,
  fallbackLng: fallback,
  interpolation: { escapeValue: false },
});

i18n.on("languageChanged", (lng) => {
  if (typeof localStorage !== "undefined") {
    localStorage.setItem("texticulo_lang", lng);
  }
});

export default i18n;
