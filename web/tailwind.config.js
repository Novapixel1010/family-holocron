/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "media",
  theme: {
    extend: {
      colors: {
        brand: {
          50: "#f0f7f6", 100: "#d9ecea", 200: "#b4d9d5", 300: "#86bfb9", 400: "#5aa19a",
          500: "#3f857f", 600: "#316a66", 700: "#2a5653", 800: "#244644", 900: "#1f3b3a",
        },
      },
      fontFamily: {
        serif: ["Georgia", "Cambria", "Times New Roman", "serif"],
      },
    },
  },
  plugins: [],
};
