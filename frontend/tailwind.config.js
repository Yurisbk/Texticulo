/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      fontFamily: {
        sans: ["DM Sans", "system-ui", "sans-serif"],
      },
      colors: {
        brand: {
          50:  "#fff0f6",
          100: "#ffd6eb",
          300: "#ff85c2",
          400: "#ff4da6",
          500: "#FF1493",
          600: "#e0117e",
          700: "#bf0e69",
          900: "#7a0943",
        },
      },
    },
  },
  plugins: [],
};
